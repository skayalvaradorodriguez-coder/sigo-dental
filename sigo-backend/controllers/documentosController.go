package controllers

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"sigo-backend/domain"
	"sigo-backend/middleware"
	"sigo-backend/models"
	"sigo-backend/notificaciones"
	"sigo-backend/repository"
	"sigo-backend/seguridad"
	"sigo-backend/validacion"
)

func sha256Hex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// ================================================================ Consentimientos (RF-06)
//
// Dos formas de firma, ninguna "biométrica" ni "digital con certificado":
//   - MANUSCRITA: el paciente (o su tutor) firma con el dedo o un lápiz en la pantalla; se guarda
//     la imagen PNG del trazo, su SHA-256 y el SHA-256 del documento.
//   - OTP: se envía un código de 6 dígitos al teléfono o correo del paciente (proveedor SIMULADO en
//     esta etapa) y se firma al ingresarlo: 10 minutos de vigencia, 5 intentos.

const (
	otpVigencia     = 10 * time.Minute
	otpMaxIntentos  = 5
	firmaMaxBytes   = 300 * 1024
	firmaMinBytes   = 200
	plantillaMaxLen = 20000
)

var reCodigoPlantilla = regexp.MustCompile(`^[A-Z0-9-]{3,20}$`)

// ListarPlantillas — GET /api/consentimientos/plantillas (última versión de cada una)
func (h *Controllers) ListarPlantillas(c *fiber.Ctx) error {
	ps, err := h.Repo.PlantillasVigentes(c.UserContext())
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(ps)
}

type plantillaIn struct {
	Codigo          string `json:"codigo"`
	Titulo          string `json:"titulo"`
	Texto           string `json:"texto"`
	ProcedimientoID string `json:"procedimientoId"`
}

// CrearPlantilla — POST /api/consentimientos/plantillas: crea o versiona (mismo código ⇒ versión nueva).
func (h *Controllers) CrearPlantilla(c *fiber.Ctx) error {
	var in plantillaIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	in.Codigo = strings.ToUpper(strings.TrimSpace(in.Codigo))
	errs := validacion.Errores{}
	if !reCodigoPlantilla.MatchString(in.Codigo) {
		errs.Agregar("codigo", "Código: 3 a 20 letras mayúsculas, números o guiones.")
	}
	errs.Longitud("titulo", strings.TrimSpace(in.Titulo), 3, 150, "Título: 3 a 150 caracteres.")
	errs.Longitud("texto", strings.TrimSpace(in.Texto), 50, plantillaMaxLen, "Texto: 50 a 20000 caracteres.")
	errs.UUID("procedimientoId", in.ProcedimientoID, false)
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	ctx := c.UserContext()
	if in.ProcedimientoID != "" {
		if ok, err := h.Repo.ProcedimientoActivo(ctx, in.ProcedimientoID); err != nil || !ok {
			return malaPeticion(c, "PROCEDIMIENTO_INVALIDO", "El procedimiento no existe o está inactivo.")
		}
	}
	id, err := h.Repo.NuevaVersionPlantilla(ctx, in.Codigo, strings.TrimSpace(in.Titulo), strings.TrimSpace(in.Texto),
		validacion.Opcional(in.ProcedimientoID), middleware.Actual(c).Usuario.ID)
	if err != nil {
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "plantilla_consentimiento", EntidadID: id, Detalle: map[string]any{"codigo": in.Codigo}})
	p, err := h.Repo.PlantillaPorID(ctx, id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.Status(201).JSON(p)
}

// fechaLarga: "30 de octubre de 2026" en la zona de la clínica.
func (h *Controllers) fechaLarga(t time.Time) string {
	meses := []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
	l := t.In(h.Cfg.ZonaHoraria)
	return fmt.Sprintf("%d de %s de %d", l.Day(), meses[l.Month()-1], l.Year())
}

func (h *Controllers) hashConsentimiento(c *models.Consentimiento, firmado time.Time) string {
	sha := ""
	if c.FirmaImagenSHA256 != nil {
		sha = *c.FirmaImagenSHA256
	}
	dest := ""
	if c.OTPDestinatario != nil {
		dest = *c.OTPDestinatario
	}
	return hashDe(map[string]any{"tipo": "CONSENTIMIENTO", "id": c.ID, "paciente": c.PacienteID, "documento": c.HashDocumento,
		"metodo": c.Metodo, "firmante": c.FirmanteNombre, "firmanteDocumento": c.FirmanteDocumento, "relacion": c.FirmanteRelacion,
		"firmaImagen": sha, "otpDestinatario": dest, "firmadoEn": iso(firmado)})
}

// completarConsentimiento verifica que el documento y la firma no cambiaron.
func (h *Controllers) completarConsentimiento(c *models.Consentimiento) {
	if c.Estado != "FIRMADO" || c.FirmadoEn == nil || c.HashFirma == nil {
		return
	}
	ok := sha256Hex([]byte(c.Texto)) == c.HashDocumento && *c.HashFirma == h.hashConsentimiento(c, *c.FirmadoEn)
	if ok && c.FirmaImagenRaw != nil && c.FirmaImagenSHA256 != nil {
		ok = sha256Hex(c.FirmaImagenRaw) == *c.FirmaImagenSHA256
	}
	c.Integro = &ok
}

// ListarConsentimientosPaciente — GET /api/pacientes/:id/consentimientos
func (h *Controllers) ListarConsentimientosPaciente(c *fiber.Ctx) error {
	p := h.pacienteClinico(c, false)
	if p == nil {
		return nil
	}
	cs, err := h.Repo.ConsentimientosPaciente(c.UserContext(), p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	for i := range cs {
		h.completarConsentimiento(&cs[i])
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "consentimientos", EntidadID: p.ID})
	return c.JSON(cs)
}

// ListarConsentimientos — GET /api/consentimientos?estado=
func (h *Controllers) ListarConsentimientos(c *fiber.Ctx) error {
	estado := c.Query("estado")
	if estado != "" && estado != "FIRMADO" && estado != "PENDIENTE_OTP" {
		return malaPeticion(c, "VALIDACION", "Estado inválido.")
	}
	cs, err := h.Repo.ConsentimientosRecientes(c.UserContext(), estado)
	if err != nil {
		return h.errorInterno(c, err)
	}
	for i := range cs {
		h.completarConsentimiento(&cs[i])
	}
	return c.JSON(cs)
}

func (h *Controllers) responderConsentimiento(c *fiber.Ctx, status int, id, codigoSimulado string) error {
	x, err := h.Repo.ConsentimientoPorID(c.UserContext(), id)
	if err != nil {
		return h.errorRepo(c, err, "Consentimiento no encontrado.", "")
	}
	h.completarConsentimiento(x)
	if x.FirmaImagenRaw != nil {
		x.FirmaImagen = "data:image/png;base64," + base64.StdEncoding.EncodeToString(x.FirmaImagenRaw)
	}
	x.OTPSimuladoCodigo = codigoSimulado
	return c.Status(status).JSON(x)
}

// ObtenerConsentimiento — GET /api/consentimientos/:id (incluye la imagen de la firma)
func (h *Controllers) ObtenerConsentimiento(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Consentimiento no encontrado.")
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "consentimiento", EntidadID: id})
	return h.responderConsentimiento(c, 200, id, "")
}

type consentimientoIn struct {
	PlantillaID     string `json:"plantillaId"`
	ProcedimientoID string `json:"procedimientoId"`
	Metodo          string `json:"metodo"`
	FirmaImagen     string `json:"firmaImagen"`   // data:image/png;base64,... (sólo MANUSCRITA)
	HashDocumento   string `json:"hashDocumento"` // SHA-256 del texto mostrado en la vista previa
}

var pngFirma = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

func decodificarFirma(dataURL string) ([]byte, string) {
	const prefijo = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefijo) {
		return nil, "La firma debe ser una imagen PNG."
	}
	b, err := base64.StdEncoding.DecodeString(dataURL[len(prefijo):])
	switch {
	case err != nil:
		return nil, "La imagen de la firma no es válida."
	case !bytes.HasPrefix(b, pngFirma):
		return nil, "La firma debe ser una imagen PNG."
	case len(b) < firmaMinBytes:
		return nil, "La firma está vacía: el paciente debe firmar en el recuadro."
	case len(b) > firmaMaxBytes:
		return nil, "La imagen de la firma es demasiado grande (máximo 300 KB)."
	}
	return b, ""
}

func codigoOTP() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	return fmt.Sprintf("%06d", n.Int64())
}

func enmascarar(dest string) string {
	if at := strings.Index(dest, "@"); at > 1 {
		return dest[:1] + strings.Repeat("•", at-1) + dest[at:]
	}
	if len(dest) > 4 {
		return strings.Repeat("•", len(dest)-4) + dest[len(dest)-4:]
	}
	return dest
}

func documentoLegible(tipo, numero string) string {
	etiqueta := map[string]string{"CEDULA": "Cédula", "PASAPORTE": "Pasaporte"}[tipo]
	if etiqueta == "" {
		etiqueta = "Documento"
	}
	return etiqueta + " " + numero
}

type borradorConsentimiento struct {
	p          *models.Paciente
	pl         *models.PlantillaConsentimiento
	n          repository.NuevoConsentimiento
	tel, email *string
}

// prepararConsentimiento arma el documento con los datos del paciente, el firmante (paciente o tutor),
// el profesional y la fecha. Lo usan la vista previa y la firma, así el texto firmado es el mostrado.
func (h *Controllers) prepararConsentimiento(c *fiber.Ctx, in consentimientoIn) *borradorConsentimiento {
	p := h.pacienteClinico(c, false)
	if p == nil {
		return nil
	}
	ctx := c.UserContext()
	pl, err := h.Repo.PlantillaPorID(ctx, in.PlantillaID)
	if err != nil {
		_ = h.errorRepo(c, err, "Plantilla no encontrada.", "")
		return nil
	}
	procID := in.ProcedimientoID
	if procID == "" && pl.ProcedimientoID != nil {
		procID = *pl.ProcedimientoID
	}
	procNombre := "el procedimiento indicado por el profesional"
	if procID != "" {
		procs, err := h.Repo.Procedimientos(ctx, false)
		if err != nil {
			_ = h.errorInterno(c, err)
			return nil
		}
		i := slices.IndexFunc(procs, func(x models.Procedimiento) bool { return x.ID == procID })
		if i < 0 {
			_ = malaPeticion(c, "PROCEDIMIENTO_INVALIDO", "El procedimiento no existe.")
			return nil
		}
		procNombre = procs[i].Nombre
	}

	// Firmante: el paciente o, si es menor de edad, su tutor legal.
	relacion, nombre, doc, tel, email := "PACIENTE", p.Nombres+" "+p.Apellidos, documentoLegible(p.TipoDocumento, p.NumeroDocumento), p.Telefono, p.Email
	if p.EsMenor {
		if p.Tutor == nil {
			_ = responder(c, models.NewAPIError(409, "TUTOR_REQUERIDO", "El paciente es menor de edad: registre a su tutor legal para que firme."))
			return nil
		}
		relacion, nombre, doc = "TUTOR", p.Tutor.Nombre, documentoLegible(p.Tutor.TipoDocumento, p.Tutor.NumeroDocumento)
		t := p.Tutor.Telefono
		tel, email = &t, p.Tutor.Email
	}
	u := middleware.Actual(c).Usuario
	reg := ""
	if u.RegistroProfesional != nil {
		reg = *u.RegistroProfesional
	}
	texto := strings.NewReplacer(
		"{{paciente}}", p.Nombres+" "+p.Apellidos, "{{documento}}", documentoLegible(p.TipoDocumento, p.NumeroDocumento),
		"{{edad}}", fmt.Sprint(p.Edad), "{{procedimiento}}", procNombre, "{{odontologo}}", u.Nombre, "{{registro}}", reg,
		"{{fecha}}", h.fechaLarga(time.Now()), "{{firmante}}", nombre, "{{relacion}}", map[string]string{"PACIENTE": "paciente", "TUTOR": "tutor legal"}[relacion], "{{clinica}}", h.Cfg.NombreClinica,
	).Replace(pl.Texto)
	n := repository.NuevoConsentimiento{ID: uuid.NewString(), PacienteID: p.ID, PlantillaID: pl.ID, OdontologoID: u.ID, Texto: texto,
		HashDocumento: sha256Hex([]byte(texto)), ProcedimientoID: validacion.Opcional(procID), Relacion: relacion,
		FirmanteNombre: nombre, FirmanteDocumento: doc, Metodo: in.Metodo}
	return &borradorConsentimiento{p: p, pl: pl, n: n, tel: tel, email: email}
}

// VistaPreviaConsentimiento — POST /api/pacientes/:id/consentimientos/vista-previa {plantillaId, procedimientoId?}
// Devuelve el texto exacto que se firmará y su SHA-256 (no guarda nada).
func (h *Controllers) VistaPreviaConsentimiento(c *fiber.Ctx) error {
	var in consentimientoIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	errs.UUID("plantillaId", in.PlantillaID, true)
	errs.UUID("procedimientoId", in.ProcedimientoID, false)
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	b := h.prepararConsentimiento(c, in)
	if b == nil {
		return nil
	}
	return c.JSON(fiber.Map{"titulo": b.pl.Titulo, "version": b.pl.Version, "texto": b.n.Texto, "hashDocumento": b.n.HashDocumento,
		"firmanteNombre": b.n.FirmanteNombre, "firmanteDocumento": b.n.FirmanteDocumento, "firmanteRelacion": b.n.Relacion,
		"procedimientoId": b.n.ProcedimientoID})
}

// CrearConsentimiento — POST /api/pacientes/:id/consentimientos
func (h *Controllers) CrearConsentimiento(c *fiber.Ctx) error {
	var in consentimientoIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	errs.UUID("plantillaId", in.PlantillaID, true)
	errs.UUID("procedimientoId", in.ProcedimientoID, false)
	errs.EnLista("metodo", in.Metodo, []string{"MANUSCRITA", "OTP"}, "Método de firma: MANUSCRITA u OTP.")
	if len(in.HashDocumento) != 64 {
		errs.Agregar("hashDocumento", "Muestre primero el documento (vista previa) para que el firmante lo lea.")
	}
	var imagen []byte
	if in.Metodo == "MANUSCRITA" {
		var msg string
		if imagen, msg = decodificarFirma(in.FirmaImagen); msg != "" {
			errs.Agregar("firmaImagen", msg)
		}
	} else if in.FirmaImagen != "" {
		errs.Agregar("firmaImagen", "Con OTP no se envía imagen de firma.")
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	b := h.prepararConsentimiento(c, in)
	if b == nil {
		return nil
	}
	if in.HashDocumento != b.n.HashDocumento {
		return responder(c, models.NewAPIError(409, "DOCUMENTO_CAMBIO", "El documento cambió desde que se mostró: vuelva a leerlo antes de firmar."))
	}
	ctx := c.UserContext()
	p, n, tel, email := b.p, b.n, b.tel, b.email
	nombre, doc, relacion := n.FirmanteNombre, n.FirmanteDocumento, n.Relacion
	ahora := instante(time.Now())
	codigoSimulado := ""
	if in.Metodo == "MANUSCRITA" {
		sha := sha256Hex(imagen)
		n.Estado, n.FirmaImagen, n.FirmaImagenSHA256, n.FirmadoEn = "FIRMADO", imagen, &sha, &ahora
		tmp := models.Consentimiento{ID: n.ID, PacienteID: p.ID, HashDocumento: n.HashDocumento, Metodo: n.Metodo, FirmanteNombre: nombre,
			FirmanteDocumento: doc, FirmanteRelacion: relacion, FirmaImagenSHA256: &sha}
		hash := h.hashConsentimiento(&tmp, ahora)
		n.HashFirma = &hash
	} else {
		canal, dest := "", ""
		switch {
		case p.CanalPreferido != "EMAIL" && tel != nil && *tel != "":
			canal, dest = p.CanalPreferido, *tel
		case email != nil && *email != "":
			canal, dest = "EMAIL", *email
		case tel != nil && *tel != "":
			canal, dest = "SMS", *tel
		default:
			return malaPeticion(c, "SIN_CONTACTO", "No hay teléfono ni correo para enviar el código. Use la firma en pantalla.")
		}
		codigo := codigoOTP()
		prov := h.Despachador.Proveedor
		idProv, err := prov.Enviar(ctx, notificaciones.Mensaje{Canal: canal, Destinatario: dest,
			Texto: fmt.Sprintf("%s: su código para firmar el consentimiento informado es %s. Vence en 10 minutos. No lo comparta.", h.Cfg.NombreClinica, codigo)})
		if err != nil {
			return responder(c, models.NewAPIError(502, "MENSAJERIA_NO_DISPONIBLE", "No se pudo enviar el código. Intente de nuevo o use la firma en pantalla."))
		}
		otpHash := seguridad.SHA256Hex(n.ID + ":" + codigo)
		expira := ahora.Add(otpVigencia)
		nombreProv, simulado, mascara := prov.Nombre(), prov.Simulado(), enmascarar(dest)
		n.Estado, n.OTPHash, n.OTPExpira, n.OTPCanal, n.OTPDestinatario, n.OTPProveedor, n.OTPSimulado, n.OTPIDProv =
			"PENDIENTE_OTP", &otpHash, &expira, &canal, &mascara, &nombreProv, &simulado, &idProv
		if simulado {
			codigoSimulado = codigo // sólo con el proveedor simulado, para poder probar el flujo
		}
	}
	if _, err := h.Repo.CrearConsentimiento(ctx, n); err != nil {
		return h.errorRepo(c, err, "", "")
	}
	accion := "FIRMA"
	if in.Metodo == "OTP" {
		accion = "CREACION"
	}
	h.Auditar(c, repository.Evento{Accion: accion, Entidad: "consentimiento", EntidadID: n.ID,
		Detalle: map[string]any{"paciente": p.ID, "metodo": in.Metodo, "firmante": relacion, "hashDocumento": n.HashDocumento}})
	return h.responderConsentimiento(c, 201, n.ID, codigoSimulado)
}

// VerificarOTPConsentimiento — POST /api/consentimientos/:id/verificar-otp {codigo}
func (h *Controllers) VerificarOTPConsentimiento(c *fiber.Ctx) error {
	var in firmaIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Consentimiento no encontrado.")
	}
	ctx := c.UserContext()
	x, err := h.Repo.ConsentimientoPorID(ctx, id)
	if err != nil {
		return h.errorRepo(c, err, "Consentimiento no encontrado.", "")
	}
	switch {
	case x.Estado == "FIRMADO":
		return responder(c, models.NewAPIError(409, "YA_FIRMADO", "El consentimiento ya está firmado."))
	case x.OTPExpira == nil || time.Now().After(*x.OTPExpira):
		return responder(c, models.NewAPIError(409, "OTP_VENCIDO", "El código venció. Genere un consentimiento nuevo."))
	case x.OTPIntentos >= otpMaxIntentos:
		return responder(c, models.NewAPIError(409, "OTP_BLOQUEADO", "Se superaron los intentos. Genere un consentimiento nuevo."))
	}
	intentos, err := h.Repo.SumarIntentoOTP(ctx, id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	esperado := seguridad.SHA256Hex(id + ":" + strings.TrimSpace(in.Codigo))
	if x.OTPHash == nil || subtle.ConstantTimeCompare([]byte(esperado), []byte(*x.OTPHash)) != 1 {
		h.Auditar(c, repository.Evento{Accion: "FIRMA_FALLIDA", Entidad: "consentimiento", EntidadID: id, Detalle: map[string]any{"intentos": intentos}})
		return responder(c, models.NewAPIError(400, "OTP_INVALIDO", fmt.Sprintf("Código incorrecto. Quedan %d intento(s).", max(0, otpMaxIntentos-intentos))))
	}
	firmado := instante(time.Now())
	hash := h.hashConsentimiento(x, firmado)
	if err := h.Repo.FirmarConsentimiento(ctx, id, firmado, hash); err != nil {
		return h.errorRepo(c, err, "Consentimiento no encontrado.", "")
	}
	h.Auditar(c, repository.Evento{Accion: "FIRMA", Entidad: "consentimiento", EntidadID: id, Detalle: map[string]any{"metodo": "OTP", "hash": hash}})
	return h.responderConsentimiento(c, 200, id, "")
}

// ================================================================ Recetas (propuesta P-01)

func (h *Controllers) hashReceta(pacienteID, odontologoID string, indicaciones *string, items []models.RecetaItem, emitida time.Time) string {
	lista := []any{}
	for _, it := range items {
		lista = append(lista, map[string]any{"medicamento": it.Medicamento, "presentacion": it.Presentacion, "dosis": it.Dosis,
			"via": it.Via, "frecuencia": it.Frecuencia, "duracion": it.Duracion, "indicaciones": it.Indicaciones})
	}
	ind := ""
	if indicaciones != nil {
		ind = *indicaciones
	}
	return hashDe(map[string]any{"tipo": "RECETA", "paciente": pacienteID, "odontologo": odontologoID, "indicaciones": ind,
		"items": lista, "emitidaEn": iso(emitida)})
}

// ListarRecetas — GET /api/pacientes/:id/recetas
func (h *Controllers) ListarRecetas(c *fiber.Ctx) error {
	p := h.pacienteClinico(c, false)
	if p == nil {
		return nil
	}
	rs, err := h.Repo.RecetasPaciente(c.UserContext(), p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	for i := range rs {
		ok := rs[i].HashFirma == h.hashReceta(rs[i].PacienteID, rs[i].OdontologoID, rs[i].Indicaciones, rs[i].Items, rs[i].EmitidaEn)
		rs[i].Integra = &ok
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "recetas", EntidadID: p.ID})
	return c.JSON(rs)
}

type recetaIn struct {
	Items            []models.RecetaItem `json:"items"`
	Indicaciones     string              `json:"indicaciones"`
	ConfirmarAlertas bool                `json:"confirmarAlertas"`
	Codigo           string              `json:"codigo"`
}

// CrearReceta — POST /api/pacientes/:id/recetas. Si un medicamento choca con una alergia o con la
// anticoagulación registradas, responde 409 ALERTA_ALERGIA hasta que el profesional lo confirme.
func (h *Controllers) CrearReceta(c *fiber.Ctx) error {
	var in recetaIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	if len(in.Items) == 0 || len(in.Items) > 10 {
		errs.Agregar("items", "Registre entre 1 y 10 medicamentos.")
	}
	meds := []string{}
	for i := range in.Items {
		it := &in.Items[i]
		k := fmt.Sprintf("items[%d]", i)
		for campo, v := range map[string]*string{"medicamento": &it.Medicamento, "presentacion": &it.Presentacion, "dosis": &it.Dosis,
			"via": &it.Via, "frecuencia": &it.Frecuencia, "duracion": &it.Duracion} {
			*v = strings.TrimSpace(*v)
			errs.Longitud(k+"."+campo, *v, 1, 120, fmt.Sprintf("Medicamento %d: complete %s (hasta 120 caracteres).", i+1, campo))
		}
		it.Indicaciones = strings.TrimSpace(it.Indicaciones)
		errs.Longitud(k+".indicaciones", it.Indicaciones, 0, 500, "Indicaciones del medicamento: hasta 500 caracteres.")
		meds = append(meds, it.Medicamento)
	}
	errs.Longitud("indicaciones", in.Indicaciones, 0, 2000, "Indicaciones generales: hasta 2000 caracteres.")
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	p := h.pacienteClinico(c, true)
	if p == nil {
		return nil
	}
	ctx := c.UserContext()
	vig, err := h.Repo.AnamnesisVigentes(ctx, []string{p.ID})
	if err != nil {
		return h.errorInterno(c, err)
	}
	alertas := domain.AlertasReceta(meds, vig[p.ID].DatosAlerta())
	if len(alertas) > 0 && !in.ConfirmarAlertas {
		return responder(c, models.NewAPIError(409, "ALERTA_ALERGIA", "Revise las alertas antes de emitir la receta.").Con("alertas", alertas))
	}
	if !h.reautenticar(c, in.Codigo, "receta", p.ID) {
		return nil
	}
	u := middleware.Actual(c).Usuario
	emitida := instante(time.Now())
	ind := validacion.Opcional(in.Indicaciones)
	hash := h.hashReceta(p.ID, u.ID, ind, in.Items, emitida)
	id, err := h.Repo.CrearReceta(ctx, p.ID, u.ID, ind, alertas, in.Items, emitida, hash)
	if err != nil {
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "receta", EntidadID: id,
		Detalle: map[string]any{"paciente": p.ID, "medicamentos": len(in.Items), "alertasConfirmadas": len(alertas), "hash": hash}})
	rs, err := h.Repo.RecetasPaciente(ctx, p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	for _, r := range rs {
		if r.ID == id {
			ok := true
			r.Integra = &ok
			return c.Status(201).JSON(r)
		}
	}
	return h.errorInterno(c, fmt.Errorf("receta %s no encontrada", id))
}

// ================================================================ Archivos del paciente

const archivoMaxBytes = 10 << 20

var categoriasArchivo = []string{"RADIOGRAFIA", "FOTOGRAFIA", "DOCUMENTO", "OTRO"}

// tipoReal identifica el archivo por su contenido (no por la extensión que dice el navegador).
func tipoReal(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case bytes.HasPrefix(b, pngFirma):
		return "image/png"
	case bytes.HasPrefix(b, []byte("%PDF-")):
		return "application/pdf"
	}
	return ""
}

// ListarArchivos — GET /api/pacientes/:id/archivos
func (h *Controllers) ListarArchivos(c *fiber.Ctx) error {
	p := h.pacienteClinico(c, false)
	if p == nil {
		return nil
	}
	as, err := h.Repo.ArchivosPaciente(c.UserContext(), p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "archivos", EntidadID: p.ID})
	return c.JSON(as)
}

// SubirArchivo — POST /api/pacientes/:id/archivos (multipart: archivo, categoria, descripcion)
func (h *Controllers) SubirArchivo(c *fiber.Ctx) error {
	errs := validacion.Errores{}
	categoria := c.FormValue("categoria")
	descripcion := strings.TrimSpace(c.FormValue("descripcion"))
	errs.EnLista("categoria", categoria, categoriasArchivo, "Categoría: RADIOGRAFIA, FOTOGRAFIA, DOCUMENTO u OTRO.")
	errs.Longitud("descripcion", descripcion, 0, 300, "Descripción: hasta 300 caracteres.")
	fh, err := c.FormFile("archivo")
	var datos []byte
	switch {
	case err != nil:
		errs.Agregar("archivo", "Adjunte el archivo.")
	case fh.Size > archivoMaxBytes:
		errs.Agregar("archivo", "El archivo supera 10 MB.")
	default:
		f, err := fh.Open()
		if err != nil {
			return h.errorInterno(c, err)
		}
		datos, err = io.ReadAll(io.LimitReader(f, archivoMaxBytes+1))
		_ = f.Close()
		if err != nil {
			return h.errorInterno(c, err)
		}
	}
	mime := tipoReal(datos)
	if datos != nil && (len(datos) == 0 || len(datos) > archivoMaxBytes) {
		errs.Agregar("archivo", "El archivo está vacío o supera 10 MB.")
	} else if datos != nil && mime == "" {
		errs.Agregar("archivo", "Sólo se aceptan JPG, PNG o PDF (se verifica el contenido, no la extensión).")
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	p := h.pacienteClinico(c, true)
	if p == nil {
		return nil
	}
	nombre := filepath.Base(strings.ReplaceAll(fh.Filename, "\\", "/"))
	if len([]rune(nombre)) > 120 {
		nombre = string([]rune(nombre)[:120])
	}
	id := uuid.NewString()
	a := models.Archivo{ID: id, PacienteID: p.ID, Categoria: categoria, Nombre: nombre, TipoMime: mime, Tamano: len(datos),
		SHA256: sha256Hex(datos), Clave: p.ID + "/" + id, Descripcion: validacion.Opcional(descripcion)}
	ctx := c.UserContext()
	if err := h.Archivos.Guardar(ctx, a.Clave, datos); err != nil {
		return h.errorInterno(c, err)
	}
	if _, err := h.Repo.RegistrarArchivo(ctx, a, middleware.Actual(c).Usuario.ID); err != nil {
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "archivo", EntidadID: id,
		Detalle: map[string]any{"paciente": p.ID, "categoria": categoria, "tipo": mime, "tamano": len(datos), "sha256": a.SHA256}})
	x, err := h.Repo.ArchivoPorID(ctx, id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.Status(201).JSON(x)
}

// DescargarArchivo — GET /api/archivos/:id/contenido (verifica el SHA-256 antes de entregarlo)
func (h *Controllers) DescargarArchivo(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Archivo no encontrado.")
	}
	ctx := c.UserContext()
	a, err := h.Repo.ArchivoPorID(ctx, id)
	if err != nil {
		return h.errorRepo(c, err, "Archivo no encontrado.", "")
	}
	datos, err := h.Archivos.Leer(ctx, a.Clave)
	if err != nil {
		return h.errorInterno(c, fmt.Errorf("almacenamiento: %w", err))
	}
	if sha256Hex(datos) != a.SHA256 {
		h.Auditar(c, repository.Evento{Accion: "ARCHIVO_ALTERADO", Entidad: "archivo", EntidadID: id})
		return responder(c, models.NewAPIError(409, "ARCHIVO_ALTERADO", "El archivo no coincide con su huella SHA-256: pudo ser modificado fuera del sistema."))
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "archivo", EntidadID: id, Detalle: map[string]any{"paciente": a.PacienteID}})
	c.Set(fiber.HeaderContentType, a.TipoMime)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`inline; filename="%s"`, strings.ReplaceAll(a.Nombre, `"`, "")))
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Status(http.StatusOK).Send(datos)
}

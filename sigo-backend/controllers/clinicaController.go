package controllers

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/domain"
	"sigo-backend/middleware"
	"sigo-backend/models"
	"sigo-backend/repository"
	"sigo-backend/seguridad"
	"sigo-backend/validacion"
)

// ================================================================ Expediente clínico (Etapa 3)
//
// Reglas comunes:
//   - Toda lectura del expediente queda auditada (RNF-06).
//   - Mientras la anamnesis esté pendiente no se registra nada clínico (RF-02): 409 ANAMNESIS_PENDIENTE.
//   - Firmar (notas, adendas, procedimientos) exige volver a ingresar el código MFA: es una firma
//     ELECTRÓNICA SIMPLE (re-autenticación + hash SHA-256 del contenido), no una firma digital con
//     certificado X.509 (ver docs/01-analisis-requisitos.md, A-05).

var errAnamnesisPendiente = models.NewAPIError(409, "ANAMNESIS_PENDIENTE",
	"Registre primero la anamnesis del paciente: es obligatoria antes de cualquier registro clínico (RF-02).")

// pacienteClinico valida el id y, para escrituras, que la anamnesis esté registrada.
// Devuelve nil si ya respondió con un error.
func (h *Controllers) pacienteClinico(c *fiber.Ctx, escritura bool) *models.Paciente {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		_ = noEncontrado(c, "Paciente no encontrado.")
		return nil
	}
	p, err := h.Repo.PacientePorID(c.UserContext(), id, h.Hoy())
	if err != nil {
		_ = h.errorRepo(c, err, "Paciente no encontrado.", "")
		return nil
	}
	if escritura && p.AnamnesisPendiente {
		_ = responder(c, errAnamnesisPendiente)
		return nil
	}
	return p
}

// reautenticar verifica el código MFA del usuario actual antes de firmar.
func (h *Controllers) reautenticar(c *fiber.Ctx, codigo, entidad, entidadID string) bool {
	u := middleware.Actual(c).Usuario
	if !u.MFAActivo || u.MFASecreto == nil {
		_ = malaPeticion(c, "MFA_NO_CONFIGURADO", "Configure la verificación en dos pasos para poder firmar.")
		return false
	}
	paso, ok := seguridad.VerificarTOTP(*u.MFASecreto, strings.TrimSpace(codigo), time.Now(), u.MFAUltimoPaso)
	if !ok {
		h.Auditar(c, repository.Evento{Accion: "FIRMA_FALLIDA", Entidad: entidad, EntidadID: entidadID})
		_ = responder(c, errCodigoMFA)
		return false
	}
	if err := h.Repo.GuardarPasoMFA(c.UserContext(), u.ID, paso); err != nil {
		_ = h.errorInterno(c, err)
		return false
	}
	return true
}

// instante sin nanosegundos: PostgreSQL guarda microsegundos y el hash debe poder recalcularse.
func instante(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }
func iso(t time.Time) string         { return instante(t).Format(time.RFC3339Nano) }

func hashDe(v map[string]any) string { return seguridad.SHA256Hex(repository.JSONCanonico(v)) }

// ================================================================ Odontograma (RF-03)

func (h *Controllers) odontograma(c *fiber.Ctx, pacienteID string) (*models.Odontograma, error) {
	evs, err := h.Repo.EventosOdontograma(c.UserContext(), pacienteID)
	if err != nil {
		return nil, err
	}
	plano := make([]domain.EventoOdontograma, len(evs))
	desde := map[string]time.Time{}
	for i, e := range evs {
		s := ""
		if e.Superficie != nil {
			s = *e.Superficie
		}
		plano[i] = domain.EventoOdontograma{Diente: e.Diente, Superficie: s, Hallazgo: e.Hallazgo, Accion: e.Accion}
		if e.Accion == "AGREGAR" {
			desde[fmt.Sprint(e.Diente, s, e.Hallazgo)] = e.RegistradoEn
		}
	}
	estado := []models.MarcaOdontograma{}
	for _, m := range domain.EstadoActual(plano) {
		var sup *string
		if m.Superficie != "" {
			s := m.Superficie
			sup = &s
		}
		estado = append(estado, models.MarcaOdontograma{Diente: m.Diente, Superficie: sup, Hallazgo: m.Hallazgo,
			Desde: desde[fmt.Sprint(m.Diente, m.Superficie, m.Hallazgo)]})
	}
	slices.Reverse(evs) // historial: lo más reciente primero
	return &models.Odontograma{Catalogo: domain.Hallazgos, Estado: estado, Historial: evs}, nil
}

// ObtenerOdontograma — GET /api/pacientes/:id/odontograma
func (h *Controllers) ObtenerOdontograma(c *fiber.Ctx) error {
	p := h.pacienteClinico(c, false)
	if p == nil {
		return nil
	}
	o, err := h.odontograma(c, p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "odontograma", EntidadID: p.ID})
	return c.JSON(o)
}

type marcaIn struct {
	Diente     int    `json:"diente"`
	Superficie string `json:"superficie"`
	Hallazgo   string `json:"hallazgo"`
	Accion     string `json:"accion"`
	Nota       string `json:"nota"`
}

// MarcarOdontograma — POST /api/pacientes/:id/odontograma: agrega o quita un hallazgo (nunca borra).
func (h *Controllers) MarcarOdontograma(c *fiber.Ctx) error {
	var in marcaIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	sup, msg := domain.ValidarMarca(in.Diente, in.Superficie, strings.ToUpper(strings.TrimSpace(in.Hallazgo)))
	if msg != "" {
		errs.Agregar("hallazgo", msg)
	}
	in.Hallazgo = strings.ToUpper(strings.TrimSpace(in.Hallazgo))
	if in.Accion == "" {
		in.Accion = "AGREGAR"
	}
	errs.EnLista("accion", in.Accion, []string{"AGREGAR", "QUITAR"}, "Acción inválida (AGREGAR o QUITAR).")
	errs.Longitud("nota", in.Nota, 0, 500, "Nota: hasta 500 caracteres.")
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	p := h.pacienteClinico(c, true)
	if p == nil {
		return nil
	}
	actual, err := h.odontograma(c, p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	presente := slices.ContainsFunc(actual.Estado, func(m models.MarcaOdontograma) bool {
		s := ""
		if m.Superficie != nil {
			s = *m.Superficie
		}
		return m.Diente == in.Diente && s == sup && m.Hallazgo == in.Hallazgo
	})
	if in.Accion == "AGREGAR" && presente {
		return responder(c, models.NewAPIError(409, "MARCA_YA_REGISTRADA", "Ese hallazgo ya está registrado en ese diente."))
	}
	if in.Accion == "QUITAR" && !presente {
		return responder(c, models.NewAPIError(409, "MARCA_INEXISTENTE", "Ese hallazgo no está registrado en ese diente."))
	}
	var supPtr *string
	if sup != "" {
		supPtr = &sup
	}
	a := middleware.Actual(c)
	evID, err := h.Repo.AgregarEventoOdontograma(c.UserContext(), p.ID, in.Diente, supPtr, in.Hallazgo, in.Accion, validacion.Opcional(in.Nota), a.Usuario.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "odontograma", EntidadID: p.ID,
		Detalle: map[string]any{"evento": evID, "diente": in.Diente, "superficie": sup, "hallazgo": in.Hallazgo, "accion": in.Accion}})
	o, err := h.odontograma(c, p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.Status(201).JSON(o)
}

// ================================================================ Periodontograma (RF-04)

// ListarPeriodontogramas — GET /api/pacientes/:id/periodontogramas (más reciente primero).
func (h *Controllers) ListarPeriodontogramas(c *fiber.Ctx) error {
	p := h.pacienteClinico(c, false)
	if p == nil {
		return nil
	}
	ps, err := h.Repo.Periodontogramas(c.UserContext(), p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "periodontograma", EntidadID: p.ID})
	return c.JSON(ps)
}

type perioSitioIn struct {
	Sitio       string `json:"sitio"`
	Profundidad *int   `json:"profundidad"`
	Margen      *int   `json:"margen"`
	Sangrado    bool   `json:"sangrado"`
	Supuracion  bool   `json:"supuracion"`
}

type perioDienteIn struct {
	Diente    int            `json:"diente"`
	Ausente   bool           `json:"ausente"`
	Movilidad *int           `json:"movilidad"`
	Furca     *int           `json:"furca"`
	Sitios    []perioSitioIn `json:"sitios"`
}

type perioIn struct {
	Observaciones string          `json:"observaciones"`
	Dientes       []perioDienteIn `json:"dientes"`
}

// CrearPeriodontograma — POST /api/pacientes/:id/periodontogramas: examen completo (no se edita).
func (h *Controllers) CrearPeriodontograma(c *fiber.Ctx) error {
	var in perioIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	errs.Longitud("observaciones", in.Observaciones, 0, 2000, "Observaciones: hasta 2000 caracteres.")
	if len(in.Dientes) == 0 || len(in.Dientes) > 32 {
		errs.Agregar("dientes", "Registre entre 1 y 32 dientes.")
	}
	vistos := map[int]bool{}
	dientes := []models.PerioDiente{}
	for i, d := range in.Dientes {
		campo := fmt.Sprintf("dientes[%d]", i)
		if !domain.EsPermanente(d.Diente) {
			errs.Agregar(campo+".diente", "El periodontograma usa dientes permanentes en notación FDI (11–48).")
			continue
		}
		if vistos[d.Diente] {
			errs.Agregar(campo+".diente", fmt.Sprintf("El diente %d está repetido.", d.Diente))
			continue
		}
		vistos[d.Diente] = true
		out := models.PerioDiente{Diente: d.Diente, Ausente: d.Ausente, Sitios: []models.PerioSitio{}}
		if d.Ausente {
			if len(d.Sitios) > 0 || d.Movilidad != nil || d.Furca != nil {
				errs.Agregar(campo, fmt.Sprintf("Diente %d ausente: no lleva mediciones.", d.Diente))
			}
			dientes = append(dientes, out)
			continue
		}
		if d.Movilidad != nil && (*d.Movilidad < 0 || *d.Movilidad > 3) {
			errs.Agregar(campo+".movilidad", "Movilidad: grado 0 a 3.")
		}
		if d.Furca != nil && (*d.Furca < 0 || *d.Furca > 3) {
			errs.Agregar(campo+".furca", "Furca: clase 0 a 3.")
		}
		out.Movilidad, out.Furca = d.Movilidad, d.Furca
		if len(d.Sitios) != 6 {
			errs.Agregar(campo+".sitios", fmt.Sprintf("Diente %d: registre los 6 sitios (MV, V, DV, ML, L, DL).", d.Diente))
			continue
		}
		sitiosVistos := map[string]bool{}
		for j, s := range d.Sitios {
			cs := fmt.Sprintf("%s.sitios[%d]", campo, j)
			if !domain.SitioValido(s.Sitio) || sitiosVistos[s.Sitio] {
				errs.Agregar(cs+".sitio", "Sitio inválido o repetido.")
				continue
			}
			sitiosVistos[s.Sitio] = true
			if s.Profundidad == nil || *s.Profundidad < 0 || *s.Profundidad > 15 {
				errs.Agregar(cs+".profundidad", fmt.Sprintf("Diente %d %s: profundidad de sondaje entre 0 y 15 mm.", d.Diente, s.Sitio))
				continue
			}
			if s.Margen == nil || *s.Margen < -10 || *s.Margen > 15 {
				errs.Agregar(cs+".margen", fmt.Sprintf("Diente %d %s: margen gingival entre −10 y 15 mm.", d.Diente, s.Sitio))
				continue
			}
			out.Sitios = append(out.Sitios, models.PerioSitio{Sitio: s.Sitio, Profundidad: *s.Profundidad, Margen: *s.Margen,
				NIC: domain.NIC(*s.Profundidad, *s.Margen), Sangrado: s.Sangrado, Supuracion: s.Supuracion})
		}
		dientes = append(dientes, out)
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	p := h.pacienteClinico(c, true)
	if p == nil {
		return nil
	}
	id, err := h.Repo.CrearPeriodontograma(c.UserContext(), p.ID, validacion.Opcional(in.Observaciones), dientes, middleware.Actual(c).Usuario.ID)
	if err != nil {
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "periodontograma", EntidadID: id,
		Detalle: map[string]any{"paciente": p.ID, "dientes": len(dientes)}})
	ps, err := h.Repo.Periodontogramas(c.UserContext(), p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	for _, x := range ps {
		if x.ID == id {
			return c.Status(201).JSON(x)
		}
	}
	return h.errorInterno(c, fmt.Errorf("periodontograma %s no encontrado tras crearlo", id))
}

// ================================================================ Notas SOEP (RF-05)

func hashNota(n *models.NotaSOEP, firmanteID string, firmadaEn time.Time) string {
	cita := ""
	if n.CitaID != nil {
		cita = *n.CitaID
	}
	return hashDe(map[string]any{"tipo": "NOTA_SOEP", "nota": n.ID, "paciente": n.PacienteID, "cita": cita, "autor": n.AutorID,
		"subjetivo": n.Subjetivo, "objetivo": n.Objetivo, "evaluacion": n.Evaluacion, "plan": n.Plan,
		"creadoEn": iso(n.CreadoEn), "firmante": firmanteID, "firmadaEn": iso(firmadaEn)})
}

func hashAdenda(padreTipo, padreID, autorID, texto string, creadoEn time.Time) string {
	return hashDe(map[string]any{"tipo": "ADENDA", "de": padreTipo, "padre": padreID, "autor": autorID, "texto": texto, "creadoEn": iso(creadoEn)})
}

// completarNota calcula estado, si el usuario puede editarla ahora y si el hash coincide.
func (h *Controllers) completarNota(n *models.NotaSOEP, usuarioID string) {
	ahora := time.Now()
	switch {
	case n.FirmadaEn != nil:
		n.Estado = "FIRMADA"
		ok := n.HashFirma != nil && n.FirmadaPorID != nil && *n.HashFirma == hashNota(n, *n.FirmadaPorID, *n.FirmadaEn)
		n.Integra = &ok
	case !ahora.Before(n.BloqueoEn):
		n.Estado = "BLOQUEADA_SIN_FIRMA"
	default:
		n.Estado = "BORRADOR"
	}
	n.Editable = n.Estado == "BORRADOR" && n.AutorID == usuarioID
}

// ListarNotas — GET /api/pacientes/:id/notas (bitácora cronológica, más reciente primero).
func (h *Controllers) ListarNotas(c *fiber.Ctx) error {
	p := h.pacienteClinico(c, false)
	if p == nil {
		return nil
	}
	ns, err := h.Repo.NotasPaciente(c.UserContext(), p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	uid := middleware.Actual(c).Usuario.ID
	for i := range ns {
		h.completarNota(&ns[i], uid)
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "notas_soep", EntidadID: p.ID})
	return c.JSON(ns)
}

type notaIn struct {
	CitaID     string `json:"citaId"`
	Subjetivo  string `json:"subjetivo"`
	Objetivo   string `json:"objetivo"`
	Evaluacion string `json:"evaluacion"`
	Plan       string `json:"plan"`
}

func (in *notaIn) validar() (repository.ContenidoSOEP, validacion.Errores) {
	errs := validacion.Errores{}
	cont := repository.ContenidoSOEP{Subjetivo: strings.TrimSpace(in.Subjetivo), Objetivo: strings.TrimSpace(in.Objetivo),
		Evaluacion: strings.TrimSpace(in.Evaluacion), Plan: strings.TrimSpace(in.Plan)}
	for campo, v := range map[string]string{"subjetivo": cont.Subjetivo, "objetivo": cont.Objetivo, "evaluacion": cont.Evaluacion, "plan": cont.Plan} {
		errs.Longitud(campo, v, 0, 5000, campo+": hasta 5000 caracteres.")
	}
	if cont.Subjetivo+cont.Objetivo+cont.Evaluacion+cont.Plan == "" {
		errs.Agregar("subjetivo", "La nota está vacía: complete al menos un apartado S, O, E o P.")
	}
	return cont, errs
}

func (h *Controllers) responderNota(c *fiber.Ctx, status int, id string) error {
	n, err := h.Repo.NotaPorID(c.UserContext(), id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.completarNota(n, middleware.Actual(c).Usuario.ID)
	return c.Status(status).JSON(n)
}

// CrearNota — POST /api/pacientes/:id/notas. El asistente puede dejar borradores; sólo firma el odontólogo.
func (h *Controllers) CrearNota(c *fiber.Ctx) error {
	var in notaIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	cont, errs := in.validar()
	errs.UUID("citaId", in.CitaID, false)
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	p := h.pacienteClinico(c, true)
	if p == nil {
		return nil
	}
	ctx := c.UserContext()
	var cita *string
	if in.CitaID != "" {
		ok, err := h.Repo.CitaDelPaciente(ctx, in.CitaID, p.ID)
		if err != nil {
			return h.errorInterno(c, err)
		}
		if !ok {
			return malaPeticion(c, "CITA_INVALIDA", "La cita no existe o es de otro paciente.")
		}
		cita = &in.CitaID
	}
	horas := h.Ajuste(ctx, repository.AjusteBloqueoNotas)
	id, err := h.Repo.CrearNota(ctx, p.ID, cita, middleware.Actual(c).Usuario.ID, cont, horas)
	if err != nil {
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "nota_soep", EntidadID: id, Detalle: map[string]any{"paciente": p.ID, "horasBloqueo": horas}})
	return h.responderNota(c, 201, id)
}

// notaParaEscribir carga la nota y responde si no existe.
func (h *Controllers) notaParaEscribir(c *fiber.Ctx) *models.NotaSOEP {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		_ = noEncontrado(c, "Nota no encontrada.")
		return nil
	}
	n, err := h.Repo.NotaPorID(c.UserContext(), id)
	if err != nil {
		_ = h.errorRepo(c, err, "Nota no encontrada.", "")
		return nil
	}
	h.completarNota(n, middleware.Actual(c).Usuario.ID)
	return n
}

var (
	errNotaFirmada   = models.NewAPIError(409, "NOTA_FIRMADA", "La nota ya está firmada: no se modifica. Agregue una adenda.")
	errNotaBloqueada = models.NewAPIError(409, "NOTA_BLOQUEADA", "Venció el plazo de edición de la nota (RF-05). Agregue una adenda.")
)

// ActualizarNota — PUT /api/notas/:id: sólo el autor, sólo antes del bloqueo y sin firma.
func (h *Controllers) ActualizarNota(c *fiber.Ctx) error {
	var in notaIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	if in.CitaID != "" {
		return malaPeticion(c, "VALIDACION", "La cita de una nota no se cambia.")
	}
	cont, errs := in.validar()
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	n := h.notaParaEscribir(c)
	if n == nil {
		return nil
	}
	switch {
	case n.Estado == "FIRMADA":
		return responder(c, errNotaFirmada)
	case n.Estado == "BLOQUEADA_SIN_FIRMA":
		return responder(c, errNotaBloqueada)
	case !n.Editable:
		return responder(c, models.NewAPIError(403, "NO_ES_AUTOR", "Sólo el autor puede editar el borrador."))
	}
	if err := h.Repo.ActualizarNota(c.UserContext(), n.ID, cont); err != nil {
		if err == repository.ErrConflicto {
			return responder(c, errNotaBloqueada)
		}
		return h.errorRepo(c, err, "Nota no encontrada.", "")
	}
	cambios := map[string]any{}
	for campo, par := range map[string][2]string{"subjetivo": {n.Subjetivo, cont.Subjetivo}, "objetivo": {n.Objetivo, cont.Objetivo},
		"evaluacion": {n.Evaluacion, cont.Evaluacion}, "plan": {n.Plan, cont.Plan}} {
		if par[0] != par[1] {
			cambios[campo] = map[string]string{"de": par[0], "a": par[1]}
		}
	}
	h.Auditar(c, repository.Evento{Accion: "MODIFICACION", Entidad: "nota_soep", EntidadID: n.ID, Detalle: map[string]any{"cambios": cambios}})
	return h.responderNota(c, 200, n.ID)
}

type firmaIn struct {
	Codigo string `json:"codigo"`
}

// FirmarNota — POST /api/notas/:id/firmar {codigo}: firma electrónica simple del odontólogo.
func (h *Controllers) FirmarNota(c *fiber.Ctx) error {
	var in firmaIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	n := h.notaParaEscribir(c)
	if n == nil {
		return nil
	}
	if n.Estado == "FIRMADA" {
		return responder(c, errNotaFirmada)
	}
	if n.Subjetivo == "" || n.Objetivo == "" || n.Evaluacion == "" || n.Plan == "" {
		return malaPeticion(c, "NOTA_INCOMPLETA", "Para firmar, la nota debe tener los cuatro apartados: S, O, E y P.")
	}
	if !h.reautenticar(c, in.Codigo, "nota_soep", n.ID) {
		return nil
	}
	u := middleware.Actual(c).Usuario
	firmada := instante(time.Now())
	hash := hashNota(n, u.ID, firmada)
	if err := h.Repo.FirmarNota(c.UserContext(), n.ID, u.ID, firmada, hash); err != nil {
		if err == repository.ErrConflicto {
			return responder(c, errNotaFirmada)
		}
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "FIRMA", Entidad: "nota_soep", EntidadID: n.ID, Detalle: map[string]any{"hash": hash, "bloqueadaAntes": n.Estado == "BLOQUEADA_SIN_FIRMA"}})
	return h.responderNota(c, 200, n.ID)
}

type adendaIn struct {
	Texto  string `json:"texto"`
	Codigo string `json:"codigo"`
}

func validarAdenda(in *adendaIn) validacion.Errores {
	in.Texto = strings.TrimSpace(in.Texto)
	errs := validacion.Errores{}
	errs.Longitud("texto", in.Texto, 3, 5000, "Escriba la adenda (3 a 5000 caracteres).")
	return errs
}

// AdendaNota — POST /api/notas/:id/adendas {texto, codigo}: corrección firmada (la nota original no cambia).
func (h *Controllers) AdendaNota(c *fiber.Ctx) error {
	var in adendaIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	if errs := validarAdenda(&in); !errs.Ok() {
		return errValidacion(c, errs)
	}
	n := h.notaParaEscribir(c)
	if n == nil {
		return nil
	}
	if n.Estado == "BORRADOR" {
		return malaPeticion(c, "NOTA_EDITABLE", "La nota aún es un borrador editable: corríjala directamente.")
	}
	if !h.reautenticar(c, in.Codigo, "nota_soep", n.ID) {
		return nil
	}
	u := middleware.Actual(c).Usuario
	creado := instante(time.Now())
	hash := hashAdenda("NOTA_SOEP", n.ID, u.ID, in.Texto, creado)
	if err := h.Repo.AgregarAdendaNota(c.UserContext(), n.ID, u.ID, in.Texto, creado, hash); err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "ADENDA", Entidad: "nota_soep", EntidadID: n.ID, Detalle: map[string]any{"hash": hash}})
	return h.responderNota(c, 201, n.ID)
}

// ================================================================ Procedimientos realizados (RN-01)

// ListarRealizados — GET /api/pacientes/:id/procedimientos
func (h *Controllers) ListarRealizados(c *fiber.Ctx) error {
	p := h.pacienteClinico(c, false)
	if p == nil {
		return nil
	}
	rs, err := h.Repo.RealizadosPaciente(c.UserContext(), p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "procedimientos_realizados", EntidadID: p.ID})
	return c.JSON(rs)
}

type realizadoIn struct {
	ProcedimientoID string   `json:"procedimientoId"`
	CitaID          string   `json:"citaId"`
	Diente          *int     `json:"diente"`
	Superficies     []string `json:"superficies"`
	Observaciones   string   `json:"observaciones"`
	Codigo          string   `json:"codigo"`
	TareaID         string   `json:"tareaId"`     // tarea del presupuesto aprobado que se realiza (RF-08)
	UbicacionID     string   `json:"ubicacionId"` // de dónde se descargan los insumos (RF-14); opcional
}

// RegistrarRealizado — POST /api/pacientes/:id/procedimientos: queda firmado e inmutable (RN-01).
func (h *Controllers) RegistrarRealizado(c *fiber.Ctx) error {
	var in realizadoIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	errs.UUID("procedimientoId", in.ProcedimientoID, in.TareaID == "")
	errs.UUID("citaId", in.CitaID, false)
	errs.UUID("tareaId", in.TareaID, false)
	errs.UUID("ubicacionId", in.UbicacionID, false)
	errs.Longitud("observaciones", in.Observaciones, 0, 2000, "Observaciones: hasta 2000 caracteres.")
	sups := []string{}
	if in.Diente != nil && !domain.DienteValido(*in.Diente) {
		errs.Agregar("diente", "Diente inválido: use notación FDI.")
	}
	for _, s := range in.Superficies {
		x, ok := domain.NormalizarSuperficie(s)
		if !ok {
			errs.Agregar("superficies", "Superficie inválida (M, D, O/I, V, L/P).")
			break
		}
		if !slices.Contains(sups, x) {
			sups = append(sups, x)
		}
	}
	if len(sups) > 0 && in.Diente == nil {
		errs.Agregar("superficies", "Indique el diente de esas superficies.")
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	p := h.pacienteClinico(c, true)
	if p == nil {
		return nil
	}
	ctx := c.UserContext()
	var tarea *string
	if in.TareaID != "" {
		t, msg, err := h.tareaDelPaciente(c, in.TareaID, p.ID, false)
		if err != nil {
			return h.errorInterno(c, err)
		}
		if msg == "" && in.ProcedimientoID != "" && in.ProcedimientoID != t.ProcedimientoID {
			msg = "El procedimiento no coincide con el de la tarea."
		}
		if msg != "" {
			return errValidacion(c, validacion.Errores{"tareaId": msg})
		}
		in.ProcedimientoID = t.ProcedimientoID
		if in.Diente == nil {
			in.Diente = t.Diente
			if len(sups) == 0 {
				sups = t.Superficies
			}
		}
		tarea = &in.TareaID
	}
	if ok, err := h.Repo.ProcedimientoActivo(ctx, in.ProcedimientoID); err != nil {
		return h.errorInterno(c, err)
	} else if !ok {
		return malaPeticion(c, "PROCEDIMIENTO_INVALIDO", "El procedimiento no existe o está inactivo.")
	}
	var cita *string
	if in.CitaID != "" {
		ok, err := h.Repo.CitaDelPaciente(ctx, in.CitaID, p.ID)
		if err != nil {
			return h.errorInterno(c, err)
		}
		if !ok {
			return malaPeticion(c, "CITA_INVALIDA", "La cita no existe o es de otro paciente.")
		}
		cita = &in.CitaID
	}
	if !h.reautenticar(c, in.Codigo, "procedimiento_realizado", p.ID) {
		return nil
	}
	u := middleware.Actual(c).Usuario
	cuando := instante(time.Now())
	diente := 0
	if in.Diente != nil {
		diente = *in.Diente
	}
	hash := hashDe(map[string]any{"tipo": "PROCEDIMIENTO_REALIZADO", "paciente": p.ID, "procedimiento": in.ProcedimientoID,
		"diente": diente, "superficies": strings.Join(sups, ","), "observaciones": strings.TrimSpace(in.Observaciones),
		"odontologo": u.ID, "realizadoEn": iso(cuando)})
	id, err := h.Repo.RegistrarRealizado(ctx, repository.NuevoRealizado{PacienteID: p.ID, ProcedimientoID: in.ProcedimientoID,
		OdontologoID: u.ID, CitaID: cita, Observaciones: validacion.Opcional(in.Observaciones), Diente: in.Diente,
		Superficies: sups, RealizadoEn: cuando, Hash: hash, TareaID: tarea, UbicacionID: validacion.Opcional(in.UbicacionID)})
	if errors.Is(err, repository.ErrDuplicado) {
		return responder(c, models.NewAPIError(409, "TAREA_YA_REALIZADA", "Esa tarea ya se registró como realizada."))
	}
	if err != nil {
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "procedimiento_realizado", EntidadID: id,
		Detalle: map[string]any{"paciente": p.ID, "procedimiento": in.ProcedimientoID, "diente": diente, "hash": hash}})
	rs, err := h.Repo.RealizadosPaciente(ctx, p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	for _, r := range rs {
		if r.ID == id {
			return c.Status(201).JSON(r)
		}
	}
	return h.errorInterno(c, fmt.Errorf("procedimiento %s no encontrado tras registrarlo", id))
}

// AdendaRealizado — POST /api/procedimientos-realizados/:id/adendas {texto, codigo}
func (h *Controllers) AdendaRealizado(c *fiber.Ctx) error {
	var in adendaIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	if errs := validarAdenda(&in); !errs.Ok() {
		return errValidacion(c, errs)
	}
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Procedimiento no encontrado.")
	}
	ctx := c.UserContext()
	pid, err := h.Repo.PacienteDeRealizado(ctx, id)
	if err != nil {
		return h.errorRepo(c, err, "Procedimiento no encontrado.", "")
	}
	if !h.reautenticar(c, in.Codigo, "procedimiento_realizado", id) {
		return nil
	}
	u := middleware.Actual(c).Usuario
	creado := instante(time.Now())
	hash := hashAdenda("PROCEDIMIENTO_REALIZADO", id, u.ID, in.Texto, creado)
	if err := h.Repo.AgregarAdendaRealizado(ctx, id, u.ID, in.Texto, creado, hash); err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "ADENDA", Entidad: "procedimiento_realizado", EntidadID: id, Detalle: map[string]any{"paciente": pid, "hash": hash}})
	rs, err := h.Repo.RealizadosPaciente(ctx, pid)
	if err != nil {
		return h.errorInterno(c, err)
	}
	for _, r := range rs {
		if r.ID == id {
			return c.Status(201).JSON(r)
		}
	}
	return h.errorInterno(c, fmt.Errorf("procedimiento %s no encontrado", id))
}

package controllers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/domain"
	"sigo-backend/middleware"
	"sigo-backend/models"
	"sigo-backend/repository"
	"sigo-backend/validacion"
)

var (
	tiposDocumento = []string{"CEDULA", "PASAPORTE", "OTRO"}
	sexos          = []string{"FEMENINO", "MASCULINO", "OTRO", "NO_ESPECIFICA"}
	canales        = []string{"WHATSAPP", "SMS", "EMAIL"}
)

// ListarPacientes — GET /api/pacientes?q&page&limit. Las búsquedas quedan auditadas (RNF-06).
func (h *Controllers) ListarPacientes(c *fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	if len([]rune(q)) > 80 {
		return malaPeticion(c, "VALIDACION", "La búsqueda es demasiado larga.")
	}
	res, err := h.Repo.BuscarPacientes(c.UserContext(), q, queryInt(c, "page", 1), queryInt(c, "limit", 20), h.Hoy())
	if err != nil {
		return h.errorInterno(c, err)
	}
	if q != "" {
		h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "busqueda_pacientes", Detalle: map[string]any{"q": q, "resultados": res.Total}})
	}
	return c.JSON(res)
}

// ObtenerPaciente — GET /api/pacientes/:id (la consulta queda auditada).
func (h *Controllers) ObtenerPaciente(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Paciente no encontrado.")
	}
	p, err := h.Repo.PacientePorID(c.UserContext(), id, h.Hoy())
	if err != nil {
		return h.errorRepo(c, err, "Paciente no encontrado.", "")
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "paciente", EntidadID: id})
	return c.JSON(p)
}

type tutorIn struct {
	Nombre          string `json:"nombre"`
	TipoDocumento   string `json:"tipoDocumento"`
	NumeroDocumento string `json:"numeroDocumento"`
	Parentesco      string `json:"parentesco"`
	Telefono        string `json:"telefono"`
	Email           string `json:"email"`
}

type pacienteIn struct {
	TipoDocumento        string   `json:"tipoDocumento"`
	NumeroDocumento      string   `json:"numeroDocumento"`
	Nombres              string   `json:"nombres"`
	Apellidos            string   `json:"apellidos"`
	FechaNacimiento      string   `json:"fechaNacimiento"`
	Sexo                 string   `json:"sexo"`
	Telefono             string   `json:"telefono"`
	Email                string   `json:"email"`
	Direccion            string   `json:"direccion"`
	EmergenciaNombre     string   `json:"emergenciaNombre"`
	EmergenciaTelefono   string   `json:"emergenciaTelefono"`
	EmergenciaParentesco string   `json:"emergenciaParentesco"`
	CanalPreferido       string   `json:"canalPreferido"`
	AutorizaMensajes     bool     `json:"autorizaMensajes"`
	ConvenioID           string   `json:"convenioId"`
	NumeroAfiliado       string   `json:"numeroAfiliado"`
	Tutor                *tutorIn `json:"tutor"`
	QuitarTutor          bool     `json:"quitarTutor"`
}

// validarPaciente aplica RF-01: datos obligatorios, fecha no futura y tutor si es menor.
func (h *Controllers) validarPaciente(in *pacienteIn, tutorRegistrado bool) (repository.DatosPaciente, *models.Tutor, validacion.Errores) {
	errs := validacion.Errores{}
	errs.EnLista("tipoDocumento", in.TipoDocumento, tiposDocumento, "Tipo de documento inválido.")
	errs.Documento("numeroDocumento", in.NumeroDocumento, "Documento: 5 a 20 letras, números o guiones.")
	errs.Longitud("nombres", in.Nombres, 2, 80, "Ingrese los nombres.")
	errs.Longitud("apellidos", in.Apellidos, 2, 80, "Ingrese los apellidos.")
	esMenor := false
	if nac, ok := errs.Fecha("fechaNacimiento", in.FechaNacimiento, "Fecha de nacimiento inválida (AAAA-MM-DD)."); ok {
		hoy := h.Hoy()
		switch edad := domain.Edad(nac, hoy); {
		case nac.After(hoy):
			errs.Agregar("fechaNacimiento", "La fecha de nacimiento no puede ser futura.")
		case edad > 120:
			errs.Agregar("fechaNacimiento", "Revise la fecha de nacimiento.")
		default:
			esMenor = edad < domain.MayoriaEdad
		}
	}
	errs.EnLista("sexo", in.Sexo, sexos, "Seleccione el sexo.")
	errs.Telefono("telefono", in.Telefono, false, "Teléfono inválido.")
	errs.Email("email", in.Email, false)
	errs.Longitud("direccion", in.Direccion, 0, 200, "Dirección: hasta 200 caracteres.")
	errs.Longitud("emergenciaNombre", in.EmergenciaNombre, 3, 150, "Ingrese el contacto de emergencia.")
	errs.Telefono("emergenciaTelefono", in.EmergenciaTelefono, true, "Teléfono de emergencia inválido.")
	if in.CanalPreferido == "" {
		in.CanalPreferido = "WHATSAPP"
	}
	errs.EnLista("canalPreferido", in.CanalPreferido, canales, "Canal inválido.")
	errs.UUID("convenioId", in.ConvenioID, false)
	var tutor *models.Tutor
	if in.Tutor != nil {
		t := in.Tutor
		errs.Longitud("tutor.nombre", t.Nombre, 3, 150, "Nombre del tutor obligatorio.")
		errs.EnLista("tutor.tipoDocumento", t.TipoDocumento, tiposDocumento, "Tipo de documento del tutor inválido.")
		errs.Documento("tutor.numeroDocumento", t.NumeroDocumento, "Documento del tutor inválido.")
		errs.Longitud("tutor.parentesco", t.Parentesco, 2, 60, "Indique el parentesco del tutor.")
		errs.Telefono("tutor.telefono", t.Telefono, true, "Teléfono del tutor inválido.")
		errs.Email("tutor.email", t.Email, false)
		tutor = &models.Tutor{Nombre: strings.TrimSpace(t.Nombre), TipoDocumento: t.TipoDocumento, NumeroDocumento: strings.TrimSpace(t.NumeroDocumento),
			Parentesco: strings.TrimSpace(t.Parentesco), Telefono: strings.TrimSpace(t.Telefono), Email: validacion.Opcional(t.Email)}
	}
	quedaTutor := tutor != nil || (tutorRegistrado && !in.QuitarTutor)
	if esMenor && !quedaTutor {
		errs.Agregar("tutor", "El paciente es menor de edad: registre a su tutor legal.")
	}
	d := repository.DatosPaciente{
		TipoDocumento: in.TipoDocumento, NumeroDocumento: strings.TrimSpace(in.NumeroDocumento), Nombres: strings.TrimSpace(in.Nombres),
		Apellidos: strings.TrimSpace(in.Apellidos), FechaNacimiento: in.FechaNacimiento, Sexo: in.Sexo,
		Telefono: validacion.Opcional(in.Telefono), Email: validacion.Opcional(in.Email), Direccion: validacion.Opcional(in.Direccion),
		EmergenciaNombre: strings.TrimSpace(in.EmergenciaNombre), EmergenciaTelefono: strings.TrimSpace(in.EmergenciaTelefono),
		EmergenciaParentesco: validacion.Opcional(in.EmergenciaParentesco), CanalPreferido: in.CanalPreferido,
		AutorizaMensajes: in.AutorizaMensajes, ConvenioID: validacion.Opcional(in.ConvenioID), NumeroAfiliado: validacion.Opcional(in.NumeroAfiliado),
	}
	return d, tutor, errs
}

func errTutor(c *fiber.Ctx, errs validacion.Errores) error {
	if m, ok := errs["tutor"]; ok && len(errs) == 1 {
		return responder(c, models.NewAPIError(400, "TUTOR_REQUERIDO", m))
	}
	return errValidacion(c, errs)
}

// CrearPaciente — POST /api/pacientes (RF-01)
func (h *Controllers) CrearPaciente(c *fiber.Ctx) error {
	var in pacienteIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	d, tutor, errs := h.validarPaciente(&in, false)
	if !errs.Ok() {
		return errTutor(c, errs)
	}
	ctx := c.UserContext()
	id, err := h.Repo.CrearPaciente(ctx, d, tutor, middleware.Actual(c).Usuario.ID)
	if err != nil {
		return h.errorRepo(c, err, "", "Ya existe un paciente con ese documento.")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "paciente", EntidadID: id,
		Detalle: map[string]any{"documento": d.TipoDocumento + " " + strings.ToUpper(d.NumeroDocumento), "conTutor": tutor != nil}})
	p, err := h.Repo.PacientePorID(ctx, id, h.Hoy())
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.Status(201).JSON(p)
}

// ActualizarPaciente — PUT /api/pacientes/:id: reemplaza la ficha y audita valor anterior y nuevo.
func (h *Controllers) ActualizarPaciente(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Paciente no encontrado.")
	}
	var in pacienteIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	ctx := c.UserContext()
	antes, err := h.Repo.PacientePorID(ctx, id, h.Hoy())
	if err != nil {
		return h.errorRepo(c, err, "Paciente no encontrado.", "")
	}
	d, tutor, errs := h.validarPaciente(&in, antes.Tutor != nil)
	if !errs.Ok() {
		return errTutor(c, errs)
	}
	accion := "conservar"
	if tutor != nil {
		accion = "guardar"
	} else if in.QuitarTutor {
		accion = "quitar"
	}
	if err := h.Repo.ActualizarPaciente(ctx, id, d, tutor, accion); err != nil {
		return h.errorRepo(c, err, "Paciente no encontrado.", "Ya existe un paciente con ese documento.")
	}
	despues, err := h.Repo.PacientePorID(ctx, id, h.Hoy())
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "MODIFICACION", Entidad: "paciente", EntidadID: id, Detalle: map[string]any{"cambios": difPaciente(antes, despues, accion)}})
	return c.JSON(despues)
}

func difPaciente(a, b *models.Paciente, tutor string) map[string]any {
	s := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	campos := []struct {
		n    string
		x, y string
	}{
		{"tipoDocumento", a.TipoDocumento, b.TipoDocumento}, {"numeroDocumento", a.NumeroDocumento, b.NumeroDocumento},
		{"nombres", a.Nombres, b.Nombres}, {"apellidos", a.Apellidos, b.Apellidos}, {"fechaNacimiento", a.FechaNacimiento, b.FechaNacimiento},
		{"sexo", a.Sexo, b.Sexo}, {"telefono", s(a.Telefono), s(b.Telefono)}, {"email", s(a.Email), s(b.Email)},
		{"direccion", s(a.Direccion), s(b.Direccion)}, {"emergenciaNombre", a.EmergenciaNombre, b.EmergenciaNombre},
		{"emergenciaTelefono", a.EmergenciaTelefono, b.EmergenciaTelefono}, {"canalPreferido", a.CanalPreferido, b.CanalPreferido},
		{"autorizaMensajes", fmt.Sprint(a.AutorizaMensajes), fmt.Sprint(b.AutorizaMensajes)},
		{"convenioId", s(a.ConvenioID), s(b.ConvenioID)}, {"numeroAfiliado", s(a.NumeroAfiliado), s(b.NumeroAfiliado)},
	}
	out := map[string]any{}
	for _, c := range campos {
		if c.x != c.y {
			out[c.n] = map[string]any{"de": c.x, "a": c.y}
		}
	}
	if tutor != "conservar" {
		out["tutor"] = tutor
	}
	return out
}

// HistorialAnamnesis — GET /api/pacientes/:id/anamnesis (versión vigente primero; auditado).
func (h *Controllers) HistorialAnamnesis(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := c.UserContext()
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Paciente no encontrado.")
	}
	if _, err := h.Repo.PacientePorID(ctx, id, h.Hoy()); err != nil {
		return h.errorRepo(c, err, "Paciente no encontrado.", "")
	}
	vs, err := h.Repo.HistorialAnamnesis(ctx, id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "anamnesis", EntidadID: id})
	return c.JSON(vs)
}

type anamnesisIn struct {
	EnfermedadesSistemicas  *[]string         `json:"enfermedadesSistemicas"`
	Alergias                *[]domain.Alergia `json:"alergias"`
	AntecedentesQuirurgicos string            `json:"antecedentesQuirurgicos"`
	Coagulopatia            *bool             `json:"coagulopatia"`
	Anticoagulado           *bool             `json:"anticoagulado"`
	MedicacionActual        string            `json:"medicacionActual"`
	Embarazo                *bool             `json:"embarazo"`
	OtraCondicionCritica    string            `json:"otraCondicionCritica"`
	Observaciones           string            `json:"observaciones"`
}

// AgregarAnamnesis — POST /api/pacientes/:id/anamnesis: nueva versión (RF-02).
// Los apartados obligatorios del SRS deben responderse explícitamente (listas vacías = "niega").
func (h *Controllers) AgregarAnamnesis(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Paciente no encontrado.")
	}
	var in anamnesisIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	if in.EnfermedadesSistemicas == nil {
		errs.Agregar("enfermedadesSistemicas", "Indique las enfermedades sistémicas (lista vacía si niega).")
	}
	if in.Alergias == nil {
		errs.Agregar("alergias", "Indique las alergias (lista vacía si niega).")
	}
	if in.Coagulopatia == nil {
		errs.Agregar("coagulopatia", "Responda si presenta coagulopatía.")
	}
	if in.Anticoagulado == nil {
		errs.Agregar("anticoagulado", "Responda si toma anticoagulantes.")
	}
	enf := []string{}
	if in.EnfermedadesSistemicas != nil {
		if len(*in.EnfermedadesSistemicas) > 30 {
			errs.Agregar("enfermedadesSistemicas", "Máximo 30 enfermedades.")
		}
		for _, e := range *in.EnfermedadesSistemicas {
			if e = strings.TrimSpace(e); e != "" {
				if len([]rune(e)) > 120 {
					errs.Agregar("enfermedadesSistemicas", "Cada enfermedad: hasta 120 caracteres.")
				}
				enf = append(enf, e)
			}
		}
	}
	al := []domain.Alergia{}
	if in.Alergias != nil {
		if len(*in.Alergias) > 30 {
			errs.Agregar("alergias", "Máximo 30 alergias.")
		}
		for i, a := range *in.Alergias {
			a.Sustancia, a.Reaccion = strings.TrimSpace(a.Sustancia), strings.TrimSpace(a.Reaccion)
			errs.Longitud(fmt.Sprintf("alergias[%d].sustancia", i), a.Sustancia, 2, 80, "Indique la sustancia de cada alergia.")
			errs.EnLista(fmt.Sprintf("alergias[%d].tipo", i), a.Tipo, domain.TiposAlergia, "Tipo de alergia inválido.")
			al = append(al, a)
		}
	}
	for campo, v := range map[string]string{"antecedentesQuirurgicos": in.AntecedentesQuirurgicos, "medicacionActual": in.MedicacionActual, "observaciones": in.Observaciones} {
		errs.Longitud(campo, v, 0, 2000, campo+": hasta 2000 caracteres.")
	}
	errs.Longitud("otraCondicionCritica", in.OtraCondicionCritica, 0, 200, "Condición crítica: hasta 200 caracteres.")
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	ctx := c.UserContext()
	if _, err := h.Repo.PacientePorID(ctx, id, h.Hoy()); err != nil {
		return h.errorRepo(c, err, "Paciente no encontrado.", "")
	}
	nueva := models.Anamnesis{EnfermedadesSistemicas: enf, Alergias: al, AntecedentesQuirurgicos: validacion.Opcional(in.AntecedentesQuirurgicos),
		Coagulopatia: *in.Coagulopatia, Anticoagulado: *in.Anticoagulado, MedicacionActual: validacion.Opcional(in.MedicacionActual),
		Embarazo: in.Embarazo, OtraCondicionCritica: validacion.Opcional(in.OtraCondicionCritica), Observaciones: validacion.Opcional(in.Observaciones)}
	res, err := h.Repo.AgregarAnamnesis(ctx, id, nueva, middleware.Actual(c).Usuario.ID)
	if errors.Is(err, repository.ErrConflicto) {
		return responder(c, models.NewAPIError(409, "CONFLICTO", "Otra persona actualizó la anamnesis al mismo tiempo. Recargue e intente de nuevo."))
	}
	if err != nil {
		return h.errorInterno(c, err)
	}
	res.Alertas = domain.CalcularAlertas(res.DatosAlerta())
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "anamnesis", EntidadID: id,
		Detalle: map[string]any{"version": res.Version, "alertasCriticas": len(domain.SoloCriticas(res.Alertas))}})
	return c.Status(201).JSON(res)
}

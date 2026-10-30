package controllers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/domain"
	"sigo-backend/middleware"
	"sigo-backend/models"
	"sigo-backend/notificaciones"
	"sigo-backend/repository"
	"sigo-backend/validacion"
)

const (
	rangoMaximo     = 31 * 24 * time.Hour
	toleranciaAtras = 15 * time.Minute // registrar una cita que empezó hace poco (S-17)
)

func errSillonOcupado(conf *repository.Conflicto) models.APIError {
	e := models.NewAPIError(409, "SILLON_OCUPADO", "El sillón ya tiene una cita en ese horario (RN-03).")
	if conf != nil {
		e = e.Con("conflicto", conf)
	}
	return e
}

// ListarCitas — GET /api/citas?desde&hasta[&sillonId&odontologoId&pacienteId] (RF-09)
func (h *Controllers) ListarCitas(c *fiber.Ctx) error {
	errs := validacion.Errores{}
	desde, _ := errs.Instante("desde", c.Query("desde"), "desde: fecha ISO-8601 obligatoria.")
	hasta, _ := errs.Instante("hasta", c.Query("hasta"), "hasta: fecha ISO-8601 obligatoria.")
	f := repository.FiltroCitas{Desde: desde, Hasta: hasta, SillonID: c.Query("sillonId"), OdontologoID: c.Query("odontologoId"), PacienteID: c.Query("pacienteId")}
	errs.UUID("sillonId", f.SillonID, false)
	errs.UUID("odontologoId", f.OdontologoID, false)
	errs.UUID("pacienteId", f.PacienteID, false)
	if errs.Ok() && !hasta.After(desde) {
		errs.Agregar("hasta", "El rango de fechas es inválido.")
	}
	if errs.Ok() && hasta.Sub(desde) > rangoMaximo {
		errs.Agregar("hasta", "El rango máximo es de 31 días.")
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	cs, err := h.Repo.ListarCitas(c.UserContext(), f)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(cs)
}

func (h *Controllers) ObtenerCita(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Cita no encontrada.")
	}
	ci, err := h.Repo.CitaPorID(c.UserContext(), id)
	if err != nil {
		return h.errorRepo(c, err, "Cita no encontrada.", "")
	}
	return c.JSON(ci)
}

// validarHorario: duración múltiplo de 5 (10 min a 8 h) y no en el pasado (S-17, S-18).
func (h *Controllers) validarHorario(errs validacion.Errores, inicioISO string, duracion int) (time.Time, time.Time) {
	inicio, ok := errs.Instante("inicio", inicioISO, "Fecha y hora de inicio inválidas.")
	errs.Rango("duracionMin", duracion, 10, 480, "La cita dura entre 10 minutos y 8 horas.")
	if duracion%5 != 0 {
		errs.Agregar("duracionMin", "La duración debe ser múltiplo de 5 minutos.")
	}
	if ok && inicio.Before(h.Ahora().Add(-toleranciaAtras)) {
		errs.Agregar("inicio", "No se puede agendar en una fecha u hora pasada.")
	}
	return inicio, inicio.Add(time.Duration(duracion) * time.Minute)
}

func (h *Controllers) validarReferencias(ctx context.Context, errs validacion.Errores, odontologoID, sillonID string, procedimientoID *string) error {
	if ok, err := h.Repo.EsOdontologoActivo(ctx, odontologoID); err != nil {
		return err
	} else if !ok {
		errs.Agregar("odontologoId", "El profesional seleccionado no es un odontólogo activo.")
	}
	if ok, err := h.Repo.SillonActivo(ctx, sillonID); err != nil {
		return err
	} else if !ok {
		errs.Agregar("sillonId", "El sillón no existe o está inactivo.")
	}
	if procedimientoID != nil {
		if ok, err := h.Repo.ProcedimientoActivo(ctx, *procedimientoID); err != nil {
			return err
		} else if !ok {
			errs.Agregar("procedimientoId", "El procedimiento no existe o está inactivo.")
		}
	}
	return nil
}

func (h *Controllers) planificar(ctx context.Context, pacienteID string, inicio time.Time) ([]notificaciones.Planificada, error) {
	ct, err := h.Repo.ContactoPaciente(ctx, pacienteID)
	if err != nil {
		return nil, err
	}
	return notificaciones.Planificar(inicio, notificaciones.Contacto{Canal: ct.Canal, Telefono: ct.Telefono, Email: ct.Email, Autoriza: ct.Autoriza}, h.Ahora()), nil
}

type citaIn struct {
	PacienteID      string `json:"pacienteId"`
	OdontologoID    string `json:"odontologoId"`
	SillonID        string `json:"sillonId"`
	ProcedimientoID string `json:"procedimientoId"`
	Inicio          string `json:"inicio"`
	DuracionMin     int    `json:"duracionMin"`
	Motivo          string `json:"motivo"`
	TareaID         string `json:"tareaId"` // tarea de un presupuesto aprobado (RF-08)
}

// CrearCita — POST /api/citas. 409 SILLON_OCUPADO si el sillón está ocupado (RN-03).
func (h *Controllers) CrearCita(c *fiber.Ctx) error {
	var in citaIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	errs.UUID("pacienteId", in.PacienteID, true)
	errs.UUID("odontologoId", in.OdontologoID, true)
	errs.UUID("sillonId", in.SillonID, true)
	errs.UUID("procedimientoId", in.ProcedimientoID, false)
	errs.UUID("tareaId", in.TareaID, false)
	errs.Longitud("motivo", in.Motivo, 0, 300, "Motivo: hasta 300 caracteres.")
	inicio, fin := h.validarHorario(errs, in.Inicio, in.DuracionMin)
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	ctx := c.UserContext()
	var tarea *string
	if in.TareaID != "" {
		t, msg, err := h.tareaDelPaciente(c, in.TareaID, in.PacienteID, true)
		if err != nil {
			return h.errorInterno(c, err)
		}
		if msg != "" {
			errs.Agregar("tareaId", msg)
			return errValidacion(c, errs)
		}
		if in.ProcedimientoID == "" {
			in.ProcedimientoID = t.ProcedimientoID
		} else if in.ProcedimientoID != t.ProcedimientoID {
			errs.Agregar("procedimientoId", "El procedimiento no coincide con el de la tarea.")
			return errValidacion(c, errs)
		}
		tarea = &in.TareaID
	}
	proc := validacion.Opcional(in.ProcedimientoID)
	if err := h.validarReferencias(ctx, errs, in.OdontologoID, in.SillonID, proc); err != nil {
		return h.errorInterno(c, err)
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	msgs, err := h.planificar(ctx, in.PacienteID, inicio)
	if err != nil {
		return h.errorRepo(c, err, "Paciente no encontrado.", "")
	}
	if conf, err := h.Repo.BuscarSuperposicion(ctx, in.SillonID, inicio, fin, ""); err != nil {
		return h.errorInterno(c, err)
	} else if conf != nil {
		return responder(c, errSillonOcupado(conf))
	}
	id, err := h.Repo.CrearCita(ctx, repository.NuevaCita{PacienteID: in.PacienteID, OdontologoID: in.OdontologoID, SillonID: in.SillonID,
		ProcedimientoID: proc, Motivo: validacion.Opcional(in.Motivo), Inicio: inicio, Fin: fin, CreadoPor: middleware.Actual(c).Usuario.ID,
		TareaID: tarea}, msgs)
	if errors.Is(err, repository.ErrDuplicado) && tarea != nil {
		return responder(c, models.NewAPIError(409, "TAREA_YA_AGENDADA", "Esa tarea ya tiene una cita activa."))
	}
	if errors.Is(err, repository.ErrSuperposicion) {
		// Dos recepcionistas agendando a la vez: la BD dejó pasar sólo a una (RN-03).
		return responder(c, errSillonOcupado(nil))
	}
	if err != nil {
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "cita", EntidadID: id,
		Detalle: map[string]any{"pacienteId": in.PacienteID, "sillonId": in.SillonID, "inicio": inicio.UTC().Format(time.RFC3339), "tareaId": in.TareaID}})
	ci, err := h.Repo.CitaPorID(ctx, id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.Status(201).JSON(ci)
}

type reprogramarIn struct {
	Inicio       string `json:"inicio"`
	DuracionMin  int    `json:"duracionMin"`
	SillonID     string `json:"sillonId"`
	OdontologoID string `json:"odontologoId"`
}

// ReprogramarCita — PATCH /api/citas/:id/reprogramar (sólo Programado/Confirmado).
func (h *Controllers) ReprogramarCita(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Cita no encontrada.")
	}
	var in reprogramarIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	ctx := c.UserContext()
	actual, err := h.Repo.CitaBase(ctx, id)
	if err != nil {
		return h.errorRepo(c, err, "Cita no encontrada.", "")
	}
	if !domain.Reprogramable(actual.Estado) {
		return malaPeticion(c, "NO_REPROGRAMABLE", "Una cita en estado \""+domain.EtiquetaEstado[actual.Estado]+"\" no se puede reprogramar.")
	}
	if in.SillonID == "" {
		in.SillonID = actual.SillonID
	}
	if in.OdontologoID == "" {
		in.OdontologoID = actual.OdontologoID
	}
	errs := validacion.Errores{}
	errs.UUID("sillonId", in.SillonID, true)
	errs.UUID("odontologoId", in.OdontologoID, true)
	inicio, fin := h.validarHorario(errs, in.Inicio, in.DuracionMin)
	if errs.Ok() {
		if err := h.validarReferencias(ctx, errs, in.OdontologoID, in.SillonID, nil); err != nil {
			return h.errorInterno(c, err)
		}
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	if conf, err := h.Repo.BuscarSuperposicion(ctx, in.SillonID, inicio, fin, id); err != nil {
		return h.errorInterno(c, err)
	} else if conf != nil {
		return responder(c, errSillonOcupado(conf))
	}
	msgs, err := h.planificar(ctx, actual.PacienteID, inicio)
	if err != nil {
		return h.errorInterno(c, err)
	}
	err = h.Repo.Reprogramar(ctx, actual, in.SillonID, in.OdontologoID, inicio, fin, middleware.Actual(c).Usuario.ID, msgs)
	switch {
	case errors.Is(err, repository.ErrSuperposicion):
		return responder(c, errSillonOcupado(nil))
	case err != nil:
		return h.errorRepo(c, err, "Cita no encontrada.", "")
	}
	h.Auditar(c, repository.Evento{Accion: "MODIFICACION", Entidad: "cita", EntidadID: id, Detalle: map[string]any{
		"inicio":   map[string]any{"de": actual.Inicio.UTC().Format(time.RFC3339), "a": inicio.UTC().Format(time.RFC3339)},
		"sillonId": map[string]any{"de": actual.SillonID, "a": in.SillonID}, "odontologoId": map[string]any{"de": actual.OdontologoID, "a": in.OdontologoID}}})
	ci, err := h.Repo.CitaPorID(ctx, id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(ci)
}

type estadoIn struct {
	Estado string `json:"estado"`
	Nota   string `json:"nota"`
}

// CambiarEstadoCita — POST /api/citas/:id/estado (RF-10)
func (h *Controllers) CambiarEstadoCita(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Cita no encontrada.")
	}
	var in estadoIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	errs.EnLista("estado", in.Estado, domain.EstadosCita, "Estado inválido.")
	errs.Longitud("nota", in.Nota, 0, 300, "Nota: hasta 300 caracteres.")
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	ctx := c.UserContext()
	actual, err := h.Repo.CitaBase(ctx, id)
	if err != nil {
		return h.errorRepo(c, err, "Cita no encontrada.", "")
	}
	if !domain.TransicionValida(actual.Estado, in.Estado) {
		return malaPeticion(c, "TRANSICION_INVALIDA", "No se puede pasar de \""+domain.EtiquetaEstado[actual.Estado]+"\" a \""+domain.EtiquetaEstado[in.Estado]+"\".")
	}
	nota := validacion.Opcional(in.Nota)
	if in.Estado == domain.Cancelado && (nota == nil || len([]rune(*nota)) < 3) {
		return malaPeticion(c, "MOTIVO_REQUERIDO", "Indique el motivo de la cancelación.")
	}
	if in.Estado == domain.NoAsistio && h.Ahora().Before(actual.Inicio) {
		return malaPeticion(c, "ANTES_DE_LA_CITA", "Sólo se puede marcar \"No asistió\" a partir de la hora de la cita.")
	}
	if err := h.Repo.CambiarEstadoCita(ctx, actual, in.Estado, nota, middleware.Actual(c).Usuario.ID); err != nil {
		if errors.Is(err, repository.ErrConflicto) {
			return responder(c, models.NewAPIError(409, "CONFLICTO", "Otra persona cambió el estado de la cita. Recargue para ver el estado actual."))
		}
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "MODIFICACION", Entidad: "cita", EntidadID: id,
		Detalle: map[string]any{"estado": map[string]any{"de": actual.Estado, "a": in.Estado}, "nota": strings.TrimSpace(in.Nota)}})
	ci, err := h.Repo.CitaPorID(ctx, id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(ci)
}

// ------------------------------------------------------------------ notificaciones

// Bandeja — GET /api/notificaciones: indica si el proveedor es SIMULADO.
func (h *Controllers) Bandeja(c *fiber.Ctx) error {
	ns, err := h.Repo.BandejaNotificaciones(c.UserContext(), queryInt(c, "page", 1), queryInt(c, "limit", 30))
	if err != nil {
		return h.errorInterno(c, err)
	}
	p := h.Despachador.Proveedor
	return c.JSON(fiber.Map{"data": ns, "proveedor": fiber.Map{"nombre": p.Nombre(), "simulado": p.Simulado()}})
}

// ProcesarNotificaciones envía ahora los mensajes vencidos (también corre cada 30 s).
func (h *Controllers) ProcesarNotificaciones(ctx context.Context) (int, int, error) {
	return h.Repo.ProcesarVencidas(ctx, 50, h.Ahora(), func(e repository.EnvioPendiente) repository.ResultadoEnvio {
		r := h.Despachador.Armar(ctx, notificaciones.Envio{ID: e.ID, Tipo: e.Tipo, Canal: e.Canal, Nombres: e.Nombres, Odontologo: e.Odontologo,
			Sede: e.Sede, EstadoCita: e.EstadoCita, Destinatario: e.Destinatario, Inicio: e.Inicio, Intentos: e.Intentos})
		return repository.ResultadoEnvio{Texto: r.Texto, Proveedor: r.Proveedor, IDProveedor: r.IDProveedor, Simulado: r.Simulado, Err: r.Err}
	})
}

// Procesar — POST /api/notificaciones/procesar
func (h *Controllers) Procesar(c *fiber.Ctx) error {
	env, fal, err := h.ProcesarNotificaciones(c.UserContext())
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(fiber.Map{"enviados": env, "fallidos": fal})
}

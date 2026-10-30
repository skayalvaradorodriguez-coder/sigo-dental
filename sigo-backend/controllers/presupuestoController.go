package controllers

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/domain"
	"sigo-backend/middleware"
	"sigo-backend/models"
	"sigo-backend/repository"
	"sigo-backend/validacion"
)

// ================================================================ Presupuestos (RF-07, RF-08)
//
// Flujo: BORRADOR (editable) → PRESENTADO (congelado, con hash) → decisión del paciente por ítem
// (APROBADO, APROBADO_PARCIAL o RECHAZADO). Los ítems aprobados se convierten en tareas agendables.
// Un descuento por encima del tope (presupuestos.topeDescuento) lo autoriza OTRA persona con
// permiso presupuestos.aprobar y su código MFA (supuesto S-25; pregunta Q-06 abierta).

var estadosPresupuesto = []string{"BORRADOR", "PRESENTADO", "APROBADO", "APROBADO_PARCIAL", "RECHAZADO"}

// completarPresupuesto calcula totales, qué descuentos requieren autorización y cuántos faltan.
func (h *Controllers) completarPresupuesto(c *fiber.Ctx, p *models.Presupuesto) {
	tope := h.Ajuste(c.UserContext(), repository.AjusteTopeDescuento)
	items := make([]domain.ItemPresupuesto, len(p.Items))
	p.DescuentosPendientes = 0
	for i := range p.Items {
		it := &p.Items[i]
		hon, _ := domain.ParseDinero(it.Honorario)
		lab, _ := domain.ParseDinero(it.CostoLaboratorio)
		des, _ := domain.ParseDinero(it.Descuento)
		items[i] = domain.ItemPresupuesto{Fase: it.Fase, Honorario: hon, Laboratorio: lab, Desc: des, Decision: it.Decision}
		it.RequiereAutorizacion = domain.RequiereAutorizacion(hon, des, tope)
		if it.RequiereAutorizacion && it.DescuentoAutorizadoPor == nil {
			p.DescuentosPendientes++
		}
	}
	p.Totales = domain.CalcularTotales(items)
	if p.HashPresentado != nil {
		ok := hashPresupuesto(p) == *p.HashPresentado
		p.Integro = &ok
	}
}

type itemPresupuestoIn struct {
	Fase             int      `json:"fase"`
	FaseNombre       string   `json:"faseNombre"`
	Sesion           int      `json:"sesion"`
	ProcedimientoID  string   `json:"procedimientoId"`
	Diente           *int     `json:"diente"`
	Superficies      []string `json:"superficies"`
	Honorario        string   `json:"honorario"`
	CostoLaboratorio string   `json:"costoLaboratorio"`
	Descuento        string   `json:"descuento"`
}

type presupuestoIn struct {
	Titulo        string              `json:"titulo"`
	Observaciones string              `json:"observaciones"`
	Items         []itemPresupuestoIn `json:"items"`
}

func (h *Controllers) validarPresupuesto(c *fiber.Ctx, in *presupuestoIn) ([]repository.ItemNuevo, validacion.Errores) {
	errs := validacion.Errores{}
	in.Titulo = strings.TrimSpace(in.Titulo)
	errs.Longitud("titulo", in.Titulo, 3, 150, "Título: 3 a 150 caracteres.")
	errs.Longitud("observaciones", in.Observaciones, 0, 2000, "Observaciones: hasta 2000 caracteres.")
	if len(in.Items) > 60 {
		errs.Agregar("items", "Máximo 60 ítems por presupuesto.")
	}
	out := []repository.ItemNuevo{}
	procs := map[string]bool{}
	for i, it := range in.Items {
		k := fmt.Sprintf("items[%d]", i)
		if it.Fase < 1 || it.Fase > 20 {
			errs.Agregar(k+".fase", "Fase entre 1 y 20.")
		}
		if it.Sesion < 1 || it.Sesion > 50 {
			errs.Agregar(k+".sesion", "Sesión entre 1 y 50.")
		}
		nombre := strings.TrimSpace(it.FaseNombre)
		if nombre == "" {
			nombre = fmt.Sprintf("Fase %d", it.Fase)
		}
		errs.Longitud(k+".faseNombre", nombre, 1, 80, "Nombre de fase: hasta 80 caracteres.")
		if !validacion.EsUUID(it.ProcedimientoID) {
			errs.Agregar(k+".procedimientoId", "Seleccione el procedimiento.")
		} else {
			procs[it.ProcedimientoID] = true
		}
		if it.Diente != nil && !domain.DienteValido(*it.Diente) {
			errs.Agregar(k+".diente", "Diente inválido: use notación FDI.")
		}
		sups := []string{}
		for _, s := range it.Superficies {
			x, ok := domain.NormalizarSuperficie(s)
			if !ok {
				errs.Agregar(k+".superficies", "Superficie inválida.")
				break
			}
			if !slices.Contains(sups, x) {
				sups = append(sups, x)
			}
		}
		if len(sups) > 0 && it.Diente == nil {
			errs.Agregar(k+".superficies", "Indique el diente de esas superficies.")
		}
		if it.CostoLaboratorio == "" {
			it.CostoLaboratorio = "0"
		}
		if it.Descuento == "" {
			it.Descuento = "0"
		}
		hon, e1 := domain.ParseDinero(it.Honorario)
		lab, e2 := domain.ParseDinero(it.CostoLaboratorio)
		des, e3 := domain.ParseDinero(it.Descuento)
		switch {
		case e1 != nil:
			errs.Agregar(k+".honorario", "Honorario: número con hasta 2 decimales.")
		case e2 != nil:
			errs.Agregar(k+".costoLaboratorio", "Costo de laboratorio: número con hasta 2 decimales.")
		case e3 != nil:
			errs.Agregar(k+".descuento", "Descuento: número con hasta 2 decimales.")
		case des > hon:
			errs.Agregar(k+".descuento", "El descuento no puede superar el honorario.")
		}
		out = append(out, repository.ItemNuevo{Fase: it.Fase, FaseNombre: nombre, Sesion: it.Sesion, ProcedimientoID: it.ProcedimientoID,
			Diente: it.Diente, Superficies: sups, Honorario: hon.String(), Laboratorio: lab.String(), Descuento: des.String()})
	}
	if errs.Ok() {
		for id := range procs {
			if ok, err := h.Repo.ProcedimientoActivo(c.UserContext(), id); err != nil || !ok {
				errs.Agregar("items", "Un procedimiento no existe o está inactivo.")
				break
			}
		}
	}
	return out, errs
}

func (h *Controllers) responderPresupuesto(c *fiber.Ctx, status int, id string) error {
	p, err := h.Repo.PresupuestoPorID(c.UserContext(), id)
	if err != nil {
		return h.errorRepo(c, err, "Presupuesto no encontrado.", "")
	}
	h.completarPresupuesto(c, p)
	return c.Status(status).JSON(p)
}

// ListarPresupuestosPaciente — GET /api/pacientes/:id/presupuestos
func (h *Controllers) ListarPresupuestosPaciente(c *fiber.Ctx) error {
	p := h.pacienteClinico(c, false)
	if p == nil {
		return nil
	}
	ps, err := h.Repo.PresupuestosPaciente(c.UserContext(), p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	for i := range ps {
		h.completarPresupuesto(c, &ps[i])
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "presupuestos", EntidadID: p.ID})
	return c.JSON(ps)
}

// ListarPresupuestos — GET /api/presupuestos?estado= (bandeja del módulo)
func (h *Controllers) ListarPresupuestos(c *fiber.Ctx) error {
	estado := c.Query("estado")
	if estado != "" && !slices.Contains(estadosPresupuesto, estado) {
		return malaPeticion(c, "VALIDACION", "Estado inválido.")
	}
	ps, err := h.Repo.PresupuestosPorEstado(c.UserContext(), estado)
	if err != nil {
		return h.errorInterno(c, err)
	}
	for i := range ps {
		h.completarPresupuesto(c, &ps[i])
	}
	return c.JSON(ps)
}

func (h *Controllers) presupuestoParam(c *fiber.Ctx) *models.Presupuesto {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		_ = noEncontrado(c, "Presupuesto no encontrado.")
		return nil
	}
	p, err := h.Repo.PresupuestoPorID(c.UserContext(), id)
	if err != nil {
		_ = h.errorRepo(c, err, "Presupuesto no encontrado.", "")
		return nil
	}
	h.completarPresupuesto(c, p)
	return p
}

// ObtenerPresupuesto — GET /api/presupuestos/:id
func (h *Controllers) ObtenerPresupuesto(c *fiber.Ctx) error {
	p := h.presupuestoParam(c)
	if p == nil {
		return nil
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "presupuesto", EntidadID: p.ID})
	return c.JSON(p)
}

// CrearPresupuesto — POST /api/pacientes/:id/presupuestos (queda en BORRADOR)
func (h *Controllers) CrearPresupuesto(c *fiber.Ctx) error {
	var in presupuestoIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	items, errs := h.validarPresupuesto(c, &in)
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	p := h.pacienteClinico(c, false)
	if p == nil {
		return nil
	}
	id, err := h.Repo.CrearPresupuesto(c.UserContext(), p.ID, middleware.Actual(c).Usuario.ID, in.Titulo, validacion.Opcional(in.Observaciones), items)
	if err != nil {
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "presupuesto", EntidadID: id, Detalle: map[string]any{"paciente": p.ID, "items": len(items)}})
	return h.responderPresupuesto(c, 201, id)
}

var errNoEditable = models.NewAPIError(409, "PRESUPUESTO_NO_EDITABLE", "Sólo un presupuesto en borrador se modifica.")

// ActualizarPresupuesto — PUT /api/presupuestos/:id (sólo BORRADOR; reemplaza los ítems)
func (h *Controllers) ActualizarPresupuesto(c *fiber.Ctx) error {
	var in presupuestoIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	items, errs := h.validarPresupuesto(c, &in)
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	p := h.presupuestoParam(c)
	if p == nil {
		return nil
	}
	if p.Estado != "BORRADOR" {
		return responder(c, errNoEditable)
	}
	if err := h.Repo.ReemplazarBorrador(c.UserContext(), p.ID, in.Titulo, validacion.Opcional(in.Observaciones), items); err != nil {
		if errors.Is(err, repository.ErrConflicto) {
			return responder(c, errNoEditable)
		}
		return h.errorRepo(c, err, "Presupuesto no encontrado.", "")
	}
	h.Auditar(c, repository.Evento{Accion: "MODIFICACION", Entidad: "presupuesto", EntidadID: p.ID,
		Detalle: map[string]any{"itemsAntes": len(p.Items), "itemsDespues": len(items), "totalAntes": p.Totales.Total}})
	return h.responderPresupuesto(c, 200, p.ID)
}

// AutorizarDescuentos — POST /api/presupuestos/:id/autorizar-descuentos {codigo}
func (h *Controllers) AutorizarDescuentos(c *fiber.Ctx) error {
	var in firmaIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	p := h.presupuestoParam(c)
	if p == nil {
		return nil
	}
	if p.Estado != "BORRADOR" {
		return responder(c, errNoEditable)
	}
	ids := []string{}
	for _, it := range p.Items {
		if it.RequiereAutorizacion && it.DescuentoAutorizadoPor == nil {
			ids = append(ids, it.ID)
		}
	}
	if len(ids) == 0 {
		return responder(c, models.NewAPIError(409, "SIN_DESCUENTOS_PENDIENTES", "No hay descuentos por autorizar."))
	}
	u := middleware.Actual(c).Usuario
	if u.ID == p.OdontologoID {
		return responder(c, models.NewAPIError(403, "AUTORIZACION_PROPIA", "Un descuento sobre el tope lo autoriza otra persona, no quien elaboró el presupuesto."))
	}
	if !h.reautenticar(c, in.Codigo, "presupuesto", p.ID) {
		return nil
	}
	if err := h.Repo.AutorizarDescuentos(c.UserContext(), p.ID, ids, u.ID); err != nil {
		return h.errorRepo(c, err, "Presupuesto no encontrado.", "")
	}
	h.Auditar(c, repository.Evento{Accion: "DESCUENTO_AUTORIZADO", Entidad: "presupuesto", EntidadID: p.ID,
		Detalle: map[string]any{"items": ids, "tope": h.Ajuste(c.UserContext(), repository.AjusteTopeDescuento)}})
	return h.responderPresupuesto(c, 200, p.ID)
}

func hashPresupuesto(p *models.Presupuesto) string {
	items := []any{}
	for _, it := range p.Items {
		diente := 0
		if it.Diente != nil {
			diente = *it.Diente
		}
		items = append(items, map[string]any{"orden": it.Orden, "fase": it.Fase, "sesion": it.Sesion, "procedimiento": it.ProcedimientoID,
			"diente": diente, "superficies": strings.Join(it.Superficies, ","), "honorario": it.Honorario,
			"laboratorio": it.CostoLaboratorio, "descuento": it.Descuento})
	}
	return hashDe(map[string]any{"tipo": "PRESUPUESTO", "id": p.ID, "paciente": p.PacienteID, "titulo": p.Titulo, "items": items, "total": p.Totales.Total})
}

// PresentarPresupuesto — POST /api/presupuestos/:id/presentar: se congela con un hash.
func (h *Controllers) PresentarPresupuesto(c *fiber.Ctx) error {
	p := h.presupuestoParam(c)
	if p == nil {
		return nil
	}
	switch {
	case p.Estado != "BORRADOR":
		return responder(c, models.NewAPIError(409, "YA_PRESENTADO", "El presupuesto ya fue presentado."))
	case len(p.Items) == 0:
		return malaPeticion(c, "PRESUPUESTO_VACIO", "Agregue al menos un procedimiento.")
	case p.DescuentosPendientes > 0:
		return responder(c, models.NewAPIError(409, "DESCUENTO_SIN_AUTORIZAR",
			fmt.Sprintf("Hay %d descuento(s) sobre el tope que debe autorizar otra persona.", p.DescuentosPendientes)))
	}
	hash := hashPresupuesto(p)
	if err := h.Repo.PresentarPresupuesto(c.UserContext(), p.ID, hash); err != nil {
		return h.errorRepo(c, err, "Presupuesto no encontrado.", "")
	}
	h.Auditar(c, repository.Evento{Accion: "PRESENTACION", Entidad: "presupuesto", EntidadID: p.ID, Detalle: map[string]any{"hash": hash, "total": p.Totales.Total}})
	return h.responderPresupuesto(c, 200, p.ID)
}

type decisionIn struct {
	Decisiones []struct {
		ItemID   string `json:"itemId"`
		Aprobado bool   `json:"aprobado"`
	} `json:"decisiones"`
	Nota string `json:"nota"`
}

// DecidirPresupuesto — POST /api/presupuestos/:id/decision: aprobación total o parcial (RF-08).
func (h *Controllers) DecidirPresupuesto(c *fiber.Ctx) error {
	var in decisionIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	if len(in.Nota) > 1000 {
		return malaPeticion(c, "VALIDACION", "Nota: hasta 1000 caracteres.")
	}
	p := h.presupuestoParam(c)
	if p == nil {
		return nil
	}
	if p.Estado != "PRESENTADO" {
		return responder(c, models.NewAPIError(409, "NO_PRESENTADO", "Sólo se decide un presupuesto presentado y aún sin decisión."))
	}
	decisiones := map[string]string{}
	for _, d := range in.Decisiones {
		v := "RECHAZADO"
		if d.Aprobado {
			v = "APROBADO"
		}
		decisiones[d.ItemID] = v
	}
	lista := []string{}
	for _, it := range p.Items {
		d, ok := decisiones[it.ID]
		if !ok {
			return malaPeticion(c, "DECISION_INCOMPLETA", "Indique para cada procedimiento si se aprueba o no.")
		}
		lista = append(lista, d)
	}
	if len(decisiones) != len(p.Items) {
		return malaPeticion(c, "DECISION_INCOMPLETA", "Hay decisiones para ítems que no son de este presupuesto.")
	}
	estado := domain.EstadoDecision(lista)
	u := middleware.Actual(c).Usuario
	if err := h.Repo.DecidirPresupuesto(c.UserContext(), p.ID, decisiones, estado, u.ID, validacion.Opcional(in.Nota)); err != nil {
		if errors.Is(err, repository.ErrConflicto) {
			return responder(c, models.NewAPIError(409, "NO_PRESENTADO", "El presupuesto cambió de estado; recargue."))
		}
		return h.errorRepo(c, err, "Presupuesto no encontrado.", "")
	}
	aprob := 0
	for _, d := range lista {
		if d == "APROBADO" {
			aprob++
		}
	}
	h.Auditar(c, repository.Evento{Accion: "DECISION", Entidad: "presupuesto", EntidadID: p.ID,
		Detalle: map[string]any{"estado": estado, "aprobados": aprob, "items": len(lista)}})
	return h.responderPresupuesto(c, 200, p.ID)
}

// ListarTareas — GET /api/pacientes/:id/tareas: tareas agendables con su estado derivado.
func (h *Controllers) ListarTareas(c *fiber.Ctx) error {
	p := h.pacienteClinico(c, false)
	if p == nil {
		return nil
	}
	ts, err := h.Repo.TareasPaciente(c.UserContext(), p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(ts)
}

// tareaDelPaciente valida que la tarea exista, sea del paciente y no esté realizada.
// Devuelve un mensaje de error para el campo tareaId, o "".
func (h *Controllers) tareaDelPaciente(c *fiber.Ctx, tareaID, pacienteID string, debePendiente bool) (*models.Tarea, string, error) {
	t, pid, err := h.Repo.TareaPorID(c.UserContext(), tareaID)
	if errors.Is(err, repository.ErrNoEncontrado) {
		return nil, "La tarea no existe.", nil
	}
	if err != nil {
		return nil, "", err
	}
	switch {
	case pid != pacienteID:
		return nil, "La tarea es de otro paciente.", nil
	case t.Estado == "REALIZADA":
		return nil, "La tarea ya se realizó.", nil
	case debePendiente && t.Estado != "PENDIENTE":
		return nil, "La tarea ya tiene una cita activa.", nil
	}
	return t, "", nil
}

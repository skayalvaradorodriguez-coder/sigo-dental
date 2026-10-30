package controllers

import (
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/domain"
	"sigo-backend/middleware"
	"sigo-backend/models"
	"sigo-backend/permisos"
	"sigo-backend/repository"
	"sigo-backend/validacion"
)

// ================================================================ Honorarios (RF-13)
//
// Base: lo efectivamente cobrado por cada procedimiento realizado (cobros no anulados). La regla
// aplicable es la más específica vigente a la fecha del cobro. El odontólogo (honorarios.propios)
// sólo ve lo suyo; el Auditor (honorarios.todos) ve todo y cierra liquidaciones.

func reglasDominio(rs []models.ReglaHonorario) []domain.Regla {
	out := make([]domain.Regla, 0, len(rs))
	for _, r := range rs {
		v, _ := domain.ParseDinero(r.Valor)
		d, _ := time.Parse("2006-01-02", r.VigenteDesde)
		x := domain.Regla{ID: r.ID, Tipo: r.Tipo, Valor: v, VigenteDesde: d}
		if r.OdontologoID != nil {
			x.OdontologoID = *r.OdontologoID
		}
		if r.ProcedimientoID != nil {
			x.ProcedimientoID = *r.ProcedimientoID
		}
		out = append(out, x)
	}
	return out
}

func describir(rs []models.ReglaHonorario) {
	for i := range rs {
		rs[i].Descripcion = domain.DescribirRegla(reglasDominio(rs[i : i+1])[0])
	}
}

// ListarReglas — GET /api/honorarios/reglas (el odontólogo ve las que le aplican)
func (h *Controllers) ListarReglas(c *fiber.Ctx) error {
	rs, err := h.Repo.ReglasHonorario(c.UserContext())
	if err != nil {
		return h.errorInterno(c, err)
	}
	a := middleware.Actual(c)
	if !a.Puede(permisos.HonorariosTodos) && !a.Puede(permisos.HonorariosConfigurar) {
		rs = slices.DeleteFunc(rs, func(r models.ReglaHonorario) bool { return r.OdontologoID != nil && *r.OdontologoID != a.Usuario.ID })
	}
	describir(rs)
	return c.JSON(rs)
}

// CrearRegla — POST /api/honorarios/reglas {odontologoId?, procedimientoId?, tipo, valor, vigenteDesde}
func (h *Controllers) CrearRegla(c *fiber.Ctx) error {
	var in struct {
		OdontologoID    string `json:"odontologoId"`
		ProcedimientoID string `json:"procedimientoId"`
		Tipo            string `json:"tipo"`
		Valor           string `json:"valor"`
		VigenteDesde    string `json:"vigenteDesde"`
	}
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	errs.UUID("odontologoId", in.OdontologoID, false)
	errs.UUID("procedimientoId", in.ProcedimientoID, false)
	errs.EnLista("tipo", in.Tipo, domain.TiposRegla, "Tipo: PORCENTAJE, MONTO_FIJO o PORCENTAJE_NETO.")
	v, err := domain.ParseDinero(in.Valor)
	if err != nil {
		errs.Agregar("valor", "Valor: número positivo con hasta 2 decimales.")
	} else if in.Tipo != domain.ReglaMontoFijo && v > 10000 {
		errs.Agregar("valor", "Un porcentaje va de 0 a 100.")
	}
	errs.Fecha("vigenteDesde", in.VigenteDesde, "Fecha de vigencia inválida (AAAA-MM-DD).")
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	ctx := c.UserContext()
	if in.OdontologoID != "" {
		if ok, err := h.Repo.EsOdontologoActivo(ctx, in.OdontologoID); err != nil || !ok {
			return malaPeticion(c, "ODONTOLOGO_INVALIDO", "No es un odontólogo activo.")
		}
	}
	if in.ProcedimientoID != "" {
		if ok, err := h.Repo.ProcedimientoActivo(ctx, in.ProcedimientoID); err != nil || !ok {
			return malaPeticion(c, "PROCEDIMIENTO_INVALIDO", "El procedimiento no existe o está inactivo.")
		}
	}
	id, err := h.Repo.CrearRegla(ctx, validacion.Opcional(in.OdontologoID), validacion.Opcional(in.ProcedimientoID), in.Tipo, v.String(), in.VigenteDesde, middleware.Actual(c).Usuario.ID)
	if err != nil {
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "regla_honorario", EntidadID: id,
		Detalle: map[string]any{"tipo": in.Tipo, "valor": v.String(), "odontologo": in.OdontologoID, "procedimiento": in.ProcedimientoID, "vigenteDesde": in.VigenteDesde}})
	return c.Status(201).JSON(fiber.Map{"id": id})
}

// periodo: odontólogo (el propio si sólo tiene honorarios.propios) y rango de fechas locales.
func (h *Controllers) periodo(c *fiber.Ctx, odo, desdeS, hastaS string) (string, string, string, time.Time, time.Time, bool) {
	a := middleware.Actual(c)
	if !a.Puede(permisos.HonorariosTodos) {
		if odo != "" && odo != a.Usuario.ID {
			_ = responder(c, models.NewAPIError(403, "PROHIBIDO", "Sólo puede consultar sus propios honorarios."))
			return "", "", "", time.Time{}, time.Time{}, false
		}
		odo = a.Usuario.ID
	}
	errs := validacion.Errores{}
	errs.UUID("odontologoId", odo, true)
	d, _ := errs.Fecha("desde", desdeS, "Fecha inicial inválida (AAAA-MM-DD).")
	hs, _ := errs.Fecha("hasta", hastaS, "Fecha final inválida (AAAA-MM-DD).")
	if errs.Ok() && (hs.Before(d) || hs.Sub(d) > 366*24*time.Hour) {
		errs.Agregar("hasta", "El periodo debe ir hacia adelante y durar como máximo un año.")
	}
	if !errs.Ok() {
		_ = errValidacion(c, errs)
		return "", "", "", time.Time{}, time.Time{}, false
	}
	z := h.Cfg.ZonaHoraria
	ini := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, z)
	fin := time.Date(hs.Year(), hs.Month(), hs.Day(), 0, 0, 0, 0, z).AddDate(0, 0, 1)
	return odo, desdeS, hastaS, ini, fin, true
}

// calcular aplica las reglas a las líneas por liquidar.
func (h *Controllers) calcular(c *fiber.Ctx, odo string, ini, fin time.Time) ([]models.LineaHonorario, domain.Centavos, error) {
	ctx := c.UserContext()
	rs, err := h.Repo.ReglasHonorario(ctx)
	if err != nil {
		return nil, 0, err
	}
	reglas := reglasDominio(rs)
	ls, err := h.Repo.LineasPorLiquidar(ctx, odo, ini, fin)
	if err != nil {
		return nil, 0, err
	}
	var total domain.Centavos
	for i := range ls {
		l := &ls[i]
		monto, _ := domain.ParseDinero(l.Monto)
		lab, _ := domain.ParseDinero(l.Laboratorio)
		mat, _ := domain.ParseDinero(l.Materiales)
		r := domain.ElegirRegla(reglas, odo, l.ProcedimientoID, l.CobradoEn.In(h.Cfg.ZonaHoraria))
		if r == nil {
			l.Regla, l.Base, l.Honorario = "Sin regla vigente: configure una regla general", monto.String(), "0.00"
			continue
		}
		base, hon := domain.CalcularHonorario(*r, monto, lab, mat)
		id := r.ID
		l.ReglaID, l.Regla, l.Base, l.Honorario = &id, domain.DescribirRegla(*r), base.String(), hon.String()
		total += hon
	}
	return ls, total, nil
}

// CalcularHonorarios — GET /api/honorarios/calculo?odontologoId&desde&hasta: vista previa de lo
// pendiente de liquidar.
func (h *Controllers) CalcularHonorarios(c *fiber.Ctx) error {
	odo, desde, hasta, ini, fin, ok := h.periodo(c, c.Query("odontologoId"), c.Query("desde"), c.Query("hasta"))
	if !ok {
		return nil
	}
	ls, total, err := h.calcular(c, odo, ini, fin)
	if err != nil {
		return h.errorInterno(c, err)
	}
	sinRegla := 0
	for _, l := range ls {
		if l.ReglaID == nil {
			sinRegla++
		}
	}
	return c.JSON(fiber.Map{"odontologoId": odo, "desde": desde, "hasta": hasta, "lineas": ls, "total": total.String(), "sinRegla": sinRegla})
}

// CrearLiquidacion — POST /api/honorarios/liquidaciones {odontologoId, desde, hasta}
func (h *Controllers) CrearLiquidacion(c *fiber.Ctx) error {
	var in struct {
		OdontologoID string `json:"odontologoId"`
		Desde        string `json:"desde"`
		Hasta        string `json:"hasta"`
	}
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	odo, desde, hasta, ini, fin, ok := h.periodo(c, strings.TrimSpace(in.OdontologoID), in.Desde, in.Hasta)
	if !ok {
		return nil
	}
	ctx := c.UserContext()
	if ok, err := h.Repo.EsOdontologoActivo(ctx, odo); err != nil || !ok {
		return malaPeticion(c, "ODONTOLOGO_INVALIDO", "No es un odontólogo activo.")
	}
	ls, total, err := h.calcular(c, odo, ini, fin)
	if err != nil {
		return h.errorInterno(c, err)
	}
	if len(ls) == 0 {
		return responder(c, models.NewAPIError(409, "NADA_QUE_LIQUIDAR", "No hay cobros pendientes de liquidar en ese periodo."))
	}
	for _, l := range ls {
		if l.ReglaID == nil {
			return responder(c, models.NewAPIError(409, "SIN_REGLA", "Hay procedimientos sin regla de honorarios vigente: configure una antes de liquidar."))
		}
	}
	id, err := h.Repo.CrearLiquidacion(ctx, odo, desde, hasta, total.String(), middleware.Actual(c).Usuario.ID, ls)
	if err != nil {
		return h.errorRepo(c, err, "", "Otro usuario liquidó estos cobros al mismo tiempo. Recargue.")
	}
	h.Auditar(c, repository.Evento{Accion: "LIQUIDACION", Entidad: "liquidacion_honorarios", EntidadID: id,
		Detalle: map[string]any{"odontologo": odo, "desde": desde, "hasta": hasta, "total": total.String(), "lineas": len(ls)}})
	x, err := h.Repo.LiquidacionPorID(ctx, id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.Status(201).JSON(x)
}

// ListarLiquidaciones — GET /api/honorarios/liquidaciones?odontologoId=
func (h *Controllers) ListarLiquidaciones(c *fiber.Ctx) error {
	a := middleware.Actual(c)
	odo := c.Query("odontologoId")
	if !a.Puede(permisos.HonorariosTodos) {
		odo = a.Usuario.ID
	} else if odo != "" && !validacion.EsUUID(odo) {
		return malaPeticion(c, "VALIDACION", "Odontólogo inválido.")
	}
	xs, err := h.Repo.Liquidaciones(c.UserContext(), odo)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(xs)
}

// ObtenerLiquidacion — GET /api/honorarios/liquidaciones/:id
func (h *Controllers) ObtenerLiquidacion(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Liquidación no encontrada.")
	}
	x, err := h.Repo.LiquidacionPorID(c.UserContext(), id)
	if err != nil {
		return h.errorRepo(c, err, "Liquidación no encontrada.", "")
	}
	a := middleware.Actual(c)
	if !a.Puede(permisos.HonorariosTodos) && x.OdontologoID != a.Usuario.ID {
		return noEncontrado(c, "Liquidación no encontrada.")
	}
	return c.JSON(x)
}

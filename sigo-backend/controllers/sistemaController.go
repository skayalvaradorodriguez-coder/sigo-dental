package controllers

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/cache"
	"sigo-backend/domain"
	"sigo-backend/middleware"
	"sigo-backend/permisos"
	"sigo-backend/repository"
	"sigo-backend/validacion"
)

// Health — GET /api/health: 503 si PostgreSQL cae; "degraded" si Redis no responde.
func (h *Controllers) Health(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
	defer cancel()
	if err := h.Repo.Ping(ctx); err != nil {
		return c.Status(503).JSON(fiber.Map{"status": "error", "db": "sin conexión"})
	}
	redis, estado := "ok", "ok"
	if h.Cache.Ping(ctx) != nil {
		redis, estado = "sin conexión", "degraded"
	}
	return c.JSON(fiber.Map{"status": estado, "db": "ok", "redis": redis, "hora": time.Now().UTC().Format(time.RFC3339)})
}

// ListarAjustes — GET /api/ajustes
func (h *Controllers) ListarAjustes(c *fiber.Ctx) error {
	out := []fiber.Map{}
	for _, d := range repository.DefsAjustes {
		v, err := h.Repo.LeerAjuste(c.UserContext(), d.Clave)
		if err != nil {
			return h.errorInterno(c, err)
		}
		out = append(out, fiber.Map{"clave": d.Clave, "etiqueta": d.Etiqueta, "ayuda": d.Ayuda, "min": d.Min, "max": d.Max, "defecto": d.Defecto, "valor": v})
	}
	return c.JSON(out)
}

// GuardarAjuste — PUT /api/ajustes/:clave {valor}
func (h *Controllers) GuardarAjuste(c *fiber.Ctx) error {
	clave := c.Params("clave")
	def, ok := repository.DefAjustePor(clave)
	if !ok {
		return malaPeticion(c, "AJUSTE_DESCONOCIDO", "Parámetro desconocido: "+clave)
	}
	var in struct {
		Valor *int `json:"valor"`
	}
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	if in.Valor == nil || *in.Valor < def.Min || *in.Valor > def.Max {
		return malaPeticion(c, "VALIDACION", "Valor inválido: debe estar entre "+itoa(def.Min)+" y "+itoa(def.Max)+".")
	}
	ctx := c.UserContext()
	antes, _ := h.Repo.LeerAjuste(ctx, clave)
	if err := h.Repo.GuardarAjuste(ctx, clave, *in.Valor, middleware.Actual(c).Usuario.ID); err != nil {
		return h.errorInterno(c, err)
	}
	h.Cache.Eliminar(ctx, cache.ClaveAjuste+clave)
	h.Auditar(c, repository.Evento{Accion: "MODIFICACION", Entidad: "ajuste", EntidadID: clave, Detalle: map[string]any{"anterior": antes, "valor": *in.Valor}})
	return c.JSON(fiber.Map{"clave": clave, "anterior": antes, "valor": *in.Valor})
}

func itoa(n int) string { return strconv.Itoa(n) }

// ListarAuditoria — GET /api/auditoria?accion&entidad&desde&hasta&page&limit
func (h *Controllers) ListarAuditoria(c *fiber.Ctx) error {
	f := repository.FiltroAuditoria{Accion: c.Query("accion"), Entidad: c.Query("entidad"), EntidadID: c.Query("entidadId"),
		Page: queryInt(c, "page", 1), Limit: queryInt(c, "limit", 25)}
	if u := c.Query("usuarioId"); u != "" {
		if !validacion.EsUUID(u) {
			return malaPeticion(c, "VALIDACION", "usuarioId inválido.")
		}
		f.UsuarioID = u
	}
	for _, p := range []struct {
		clave string
		dest  **time.Time
	}{{"desde", &f.Desde}, {"hasta", &f.Hasta}} {
		if v := c.Query(p.clave); v != "" {
			t, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				return malaPeticion(c, "VALIDACION", "Fecha inválida en '"+p.clave+"' (ISO-8601).")
			}
			*p.dest = &t
		}
	}
	res, err := h.Repo.ListarAuditoria(c.UserContext(), f)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(res)
}

// VerificarAuditoria — POST /api/auditoria/verificar: recorre la cadena de hashes.
func (h *Controllers) VerificarAuditoria(c *fiber.Ctx) error {
	res, err := h.Repo.VerificarAuditoria(c.UserContext())
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "AUDITORIA_VERIFICADA", Entidad: "auditoria",
		Detalle: map[string]any{"valida": res.Valida, "verificados": res.Verificados}})
	return c.JSON(res)
}

// Panel — GET /api/panel: sólo cifras reales y sólo las que el rol puede ver.
func (h *Controllers) Panel(c *fiber.Ctx) error {
	ctx := c.UserContext()
	a := middleware.Actual(c)
	w := fiber.Map{}
	if a.Puede(permisos.UsuariosGestionar) {
		por, err := h.Repo.ResumenUsuarios(ctx)
		if err != nil {
			return h.errorInterno(c, err)
		}
		n, _ := h.Repo.SesionesAbiertas(ctx, "")
		w["usuarios"] = fiber.Map{"porRol": por, "sesionesAbiertas": n}
	}
	if a.Puede(permisos.AuditoriaLeer) {
		conteo, err := h.Repo.ContarEventosSeguridad(ctx, time.Now().Add(-24*time.Hour))
		if err != nil {
			return h.errorInterno(c, err)
		}
		recientes, err := h.Repo.ListarAuditoria(ctx, repository.FiltroAuditoria{Limit: 50})
		if err != nil {
			return h.errorInterno(c, err)
		}
		seg := []any{}
		for _, e := range recientes.Data {
			if (e.Accion == "LOGIN_FALLIDO" || e.Accion == "CUENTA_BLOQUEADA" || e.Accion == "MFA_FALLIDO" || e.Accion == "SESION_EXPIRADA") && len(seg) < 5 {
				seg = append(seg, e)
			}
		}
		w["seguridad"] = fiber.Map{"ultimas24h": conteo, "recientes": seg}
	}
	if a.Puede(permisos.CitasLeer) {
		desde, hasta := domain.DiaLocal(h.Ahora(), h.Cfg.ZonaHoraria)
		odo := ""
		if a.Usuario.Rol == permisos.Odontologo {
			odo = a.Usuario.ID // el odontólogo ve sólo su agenda
		}
		porEstado, err := h.Repo.ConteoCitasPorEstado(ctx, desde, hasta, odo)
		if err != nil {
			return h.errorInterno(c, err)
		}
		prox, err := h.Repo.ListarCitas(ctx, repository.FiltroCitas{Desde: desde, Hasta: hasta, OdontologoID: odo,
			Estados: []string{domain.Programado, domain.Confirmado, domain.EnSalaEspera, domain.EnAtencion}, Limite: 8})
		if err != nil {
			return h.errorInterno(c, err)
		}
		alcance := "TODAS"
		if odo != "" {
			alcance = "MIAS"
		}
		w["agendaHoy"] = fiber.Map{"alcance": alcance, "porEstado": porEstado, "proximas": prox}
	}
	if a.Puede(permisos.PacientesLeer) {
		total, sin, err := h.Repo.ConteoPacientes(ctx)
		if err != nil {
			return h.errorInterno(c, err)
		}
		w["pacientes"] = fiber.Map{"total": total, "anamnesisPendiente": sin}
	}
	n, _ := h.Repo.SesionesAbiertas(ctx, a.Usuario.ID)
	w["yo"] = fiber.Map{"ultimoAcceso": a.Usuario.UltimoAcceso, "sesionesAbiertas": n}
	return c.JSON(fiber.Map{"generadoEn": time.Now().UTC(), "widgets": w})
}

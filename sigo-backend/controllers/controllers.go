// Package controllers: manejadores HTTP. Validan la entrada, aplican reglas, llaman al
// repositorio y registran la auditoría. Las dependencias se inyectan en NewControllers.
package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/cache"
	"sigo-backend/config"
	"sigo-backend/middleware"
	"sigo-backend/models"
	"sigo-backend/repository"
	"sigo-backend/validacion"
)

type Controllers struct {
	Repo  repository.DBRepository
	Cache cache.CacheRepository
	Cfg   config.Config
	Ahora func() time.Time // inyectable en pruebas
}

func NewControllers(repo repository.DBRepository, c cache.CacheRepository, cfg config.Config) *Controllers {
	return &Controllers{Repo: repo, Cache: c, Cfg: cfg, Ahora: time.Now}
}

// ------------------------------------------------------------------ respuestas

func responder(c *fiber.Ctx, e models.APIError) error { return c.Status(e.Status).JSON(e) }

func malaPeticion(c *fiber.Ctx, codigo, msg string) error {
	return responder(c, models.NewAPIError(400, codigo, msg))
}

func noEncontrado(c *fiber.Ctx, msg string) error {
	return responder(c, models.NewAPIError(404, "NO_ENCONTRADO", msg))
}

func errValidacion(c *fiber.Ctx, errs validacion.Errores) error {
	e := models.NewAPIError(400, "VALIDACION", errs.Mensaje()).Con("campos", errs)
	return responder(c, e)
}

// errorRepo traduce los errores del repositorio a HTTP (404, 409, 400 o 500).
func (h *Controllers) errorRepo(c *fiber.Ctx, err error, msgNoEncontrado, msgDuplicado string) error {
	var ev *repository.ErrValidacion
	switch {
	case errors.Is(err, repository.ErrNoEncontrado):
		return noEncontrado(c, msgNoEncontrado)
	case errors.Is(err, repository.ErrDuplicado):
		return responder(c, models.NewAPIError(409, "DUPLICADO", msgDuplicado))
	case errors.Is(err, repository.ErrConflicto):
		return responder(c, models.NewAPIError(409, "CONFLICTO", "El registro cambió mientras lo editaba. Recargue e intente de nuevo."))
	case errors.As(err, &ev):
		return malaPeticion(c, ev.Codigo, ev.Mensaje)
	}
	return h.errorInterno(c, err)
}

func (h *Controllers) errorInterno(c *fiber.Ctx, err error) error {
	log.Printf("[error] %s %s: %v", c.Method(), c.Path(), err)
	e := models.NewAPIError(500, "ERROR_INTERNO", "Ocurrió un error inesperado.")
	if h.Cfg.Entorno == "dev" {
		e.Details = err.Error() // detalles sólo en desarrollo
	}
	return responder(c, e)
}

// ErrRespondido indica que el manejador ya escribió la respuesta de error y debe terminar.
// El manejador de errores de Fiber lo reconoce y no la sobrescribe.
var ErrRespondido = errors.New("respuesta ya enviada")

// leerJSON decodifica el cuerpo rechazando campos no declarados (evita asignación masiva,
// p. ej. enviar "rol" en el login). Si falla, responde 400 y devuelve ErrRespondido.
func leerJSON(c *fiber.Ctx, destino any) error {
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(destino); err != nil {
		_ = responder(c, models.NewAPIError(400, "VALIDACION", "Cuerpo de la petición inválido: "+simplificar(err)))
		return ErrRespondido
	}
	return nil
}

func simplificar(err error) string {
	var ut *json.UnmarshalTypeError
	if errors.As(err, &ut) {
		return "el campo \"" + ut.Field + "\" tiene un tipo incorrecto"
	}
	s := err.Error()
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

func queryInt(c *fiber.Ctx, clave string, defecto int) int {
	v, err := strconv.Atoi(c.Query(clave))
	if err != nil {
		return defecto
	}
	return v
}

// ------------------------------------------------------------------ auditoría

func (h *Controllers) Auditar(c *fiber.Ctx, e repository.Evento) {
	if e.IP == "" {
		e.IP = c.IP()
	}
	if e.UserAgent == "" {
		e.UserAgent = c.Get(fiber.HeaderUserAgent)
	}
	if e.UsuarioID == "" {
		if a := middleware.Actual(c); a != nil {
			e.UsuarioID = a.Usuario.ID
		}
	}
	// Un fallo de auditoría se reporta en el log; no se pierde la operación del usuario.
	if err := h.Repo.RegistrarAuditoria(context.Background(), e); err != nil {
		log.Printf("[auditoria] no se pudo registrar %s: %v", e.Accion, err)
	}
}

// ------------------------------------------------------------------ ajustes con caché

func (h *Controllers) Ajuste(ctx context.Context, clave string) int {
	var v int
	if h.Cache.Obtener(ctx, cache.ClaveAjuste+clave, &v) {
		return v
	}
	v, err := h.Repo.LeerAjuste(ctx, clave)
	if err != nil {
		def, _ := repository.DefAjustePor(clave)
		return def.Defecto
	}
	h.Cache.Guardar(ctx, cache.ClaveAjuste+clave, v, cache.TTLAjustes)
	return v
}

func (h *Controllers) Inactividad(ctx context.Context) time.Duration {
	return time.Duration(h.Ajuste(ctx, repository.AjusteInactividad)) * time.Minute
}

// Hoy en la zona horaria de la clínica.
func (h *Controllers) Hoy() time.Time {
	l := h.Ahora().In(h.Cfg.ZonaHoraria)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC)
}

// MiddlewareDeps arma las dependencias del middleware de autenticación.
func (h *Controllers) MiddlewareDeps() middleware.Deps {
	return middleware.Deps{Repo: h.Repo, JWTSecret: h.Cfg.JWTSecret, Inactividad: h.Inactividad, Auditar: h.Auditar}
}

// Package routes: arma la aplicación Fiber. Toda ruta se registra con una POLÍTICA
// (pública, sesión, completa o permisos); no hay otra forma de agregar rutas, y una
// prueba automática verifica que ninguna quede sin política (denegar por defecto).
package routes

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"sigo-backend/controllers"
	"sigo-backend/middleware"
	"sigo-backend/models"
	"sigo-backend/permisos"
)

// Registro de una ruta y su política (se usa en pruebas y en la documentación).
type Registro struct {
	Metodo, Ruta, Politica string
}

type enrutador struct {
	api        fiber.Router
	auth       fiber.Handler
	mwCompleta fiber.Handler
	registro   []Registro
}

func (e *enrutador) add(metodo, ruta, politica string, hs ...fiber.Handler) {
	e.api.Add(metodo, ruta, hs...)
	e.registro = append(e.registro, Registro{metodo, "/api" + ruta, politica})
}

// publica: sin sesión (login, health).
func (e *enrutador) publica(m, r string, h ...fiber.Handler) { e.add(m, r, "publica", h...) }

// sesion: con token válido aunque falte MFA o el cambio de contraseña (pasos del ingreso).
func (e *enrutador) sesion(m, r string, h fiber.Handler) { e.add(m, r, "sesion", e.auth, h) }

// completa: sesión completa (MFA verificado), cualquier rol.
func (e *enrutador) completa(m, r string, h fiber.Handler) {
	e.add(m, r, "completa", e.auth, e.mwCompleta, h)
}

// permiso: sesión completa + TODOS los permisos.
func (e *enrutador) permiso(m, r string, ps []string, h fiber.Handler) {
	e.add(m, r, "permisos:"+strings.Join(ps, "+"), e.auth, e.mwCompleta, middleware.RequirePermisos(ps...), h)
}

// alguno: sesión completa + AL MENOS UNO de los permisos.
func (e *enrutador) alguno(m, r string, ps []string, h fiber.Handler) {
	e.add(m, r, "alguno:"+strings.Join(ps, "|"), e.auth, e.mwCompleta, middleware.RequireAlguno(ps...), h)
}

func p(ps ...string) []string { return ps }

// manejadorErrores: todo error no controlado sale como APIError.
func manejadorErrores(c *fiber.Ctx, err error) error {
	if errors.Is(err, controllers.ErrRespondido) {
		return nil // el controlador ya respondió (p. ej. 400 por JSON inválido)
	}
	var fe *fiber.Error
	if errors.As(err, &fe) {
		codigo := "ERROR"
		switch fe.Code {
		case 404:
			codigo = "NO_ENCONTRADO"
		case 405:
			codigo = "METODO_NO_PERMITIDO"
		case 413:
			codigo = "CUERPO_DEMASIADO_GRANDE"
		}
		msg := fe.Message
		if fe.Code == 404 {
			msg = "Recurso no encontrado."
		}
		return c.Status(fe.Code).JSON(models.NewAPIError(fe.Code, codigo, msg))
	}
	return c.Status(500).JSON(models.NewAPIError(500, "ERROR_INTERNO", "Ocurrió un error inesperado."))
}

// NuevaApp crea la aplicación con middlewares globales y todas las rutas.
func NuevaApp(h *controllers.Controllers) (*fiber.App, []Registro) {
	cfgFiber := fiber.Config{AppName: "SIGO-Dental API", ErrorHandler: manejadorErrores, BodyLimit: 2 * 1024 * 1024}
	if h.Cfg.EsProduccion() {
		cfgFiber.ProxyHeader = fiber.HeaderXForwardedFor // detrás de Render / proxy inverso
	}
	app := fiber.New(cfgFiber)
	app.Use(recover.New())
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{AllowOrigins: h.Cfg.WebOrigin, AllowHeaders: "Authorization, Content-Type", AllowMethods: "GET,POST,PUT,PATCH,DELETE"}))

	e := &enrutador{api: app.Group("/api"), auth: middleware.RequireAuth(h.MiddlewareDeps()), mwCompleta: middleware.RequireCompleta()}

	// Fuerza bruta: 10 intentos por minuto e IP en login y MFA (desactivado en pruebas).
	limite := limiter.New(limiter.Config{
		Max: 10, Expiration: time.Minute,
		Next: func(*fiber.Ctx) bool { return h.Cfg.Entorno == "test" },
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(models.NewAPIError(429, "DEMASIADOS_INTENTOS", "Demasiados intentos. Espere un minuto."))
		},
	})

	// ---------------------------------------------------------------- Etapa 1
	e.publica("GET", "/health", h.Health)
	e.publica("POST", "/login", limite, h.Login)
	e.sesion("POST", "/mfa/verificar", h.VerificarMFA)
	e.sesion("POST", "/mfa/configurar", h.ConfigurarMFA)
	e.sesion("POST", "/mfa/activar", h.ActivarMFA)
	e.sesion("POST", "/password", h.CambiarPassword)
	e.sesion("POST", "/logout", h.Logout)
	e.sesion("GET", "/me", h.Me)
	e.completa("POST", "/heartbeat", h.Heartbeat)
	e.completa("GET", "/panel", h.Panel)

	e.permiso("GET", "/usuarios", p(permisos.UsuariosGestionar), h.ListarUsuarios)
	e.permiso("GET", "/usuarios/roles", p(permisos.UsuariosGestionar), h.Roles)
	e.permiso("POST", "/usuarios", p(permisos.UsuariosGestionar), h.CrearUsuario)
	e.permiso("PATCH", "/usuarios/:id", p(permisos.UsuariosGestionar), h.ActualizarUsuario)
	e.permiso("POST", "/usuarios/:id/restablecer-password", p(permisos.UsuariosGestionar), h.RestablecerPassword)
	e.permiso("POST", "/usuarios/:id/restablecer-mfa", p(permisos.UsuariosGestionar), h.RestablecerMFA)

	e.permiso("GET", "/ajustes", p(permisos.AjustesGestionar), h.ListarAjustes)
	e.permiso("PUT", "/ajustes/:clave", p(permisos.AjustesGestionar), h.GuardarAjuste)

	e.permiso("GET", "/auditoria", p(permisos.AuditoriaLeer), h.ListarAuditoria)
	e.permiso("POST", "/auditoria/verificar", p(permisos.AuditoriaLeer), h.VerificarAuditoria)

	// ---------------------------------------------------------------- Etapa 2
	lectoresCatalogo := p(permisos.CatalogosGestionar, permisos.CitasLeer, permisos.PacientesLeer)
	e.alguno("GET", "/catalogos/estructura", lectoresCatalogo, h.Estructura)
	e.alguno("GET", "/catalogos/sillones", lectoresCatalogo, h.Sillones)
	e.alguno("GET", "/catalogos/procedimientos", lectoresCatalogo, h.ListarProcedimientos)
	e.alguno("GET", "/catalogos/convenios", lectoresCatalogo, h.ListarConvenios)
	e.alguno("GET", "/catalogos/odontologos", lectoresCatalogo, h.ListarOdontologos)
	e.permiso("POST", "/catalogos/:tipo", p(permisos.CatalogosGestionar), h.CrearCatalogo)
	e.permiso("PATCH", "/catalogos/:tipo/:id", p(permisos.CatalogosGestionar), h.ActualizarCatalogo)

	e.permiso("GET", "/pacientes", p(permisos.PacientesLeer), h.ListarPacientes)
	e.permiso("GET", "/pacientes/:id", p(permisos.PacientesLeer), h.ObtenerPaciente)
	e.permiso("POST", "/pacientes", p(permisos.PacientesEscribir), h.CrearPaciente)
	e.permiso("PUT", "/pacientes/:id", p(permisos.PacientesEscribir), h.ActualizarPaciente)
	e.alguno("GET", "/pacientes/:id/anamnesis", p(permisos.ClinicaLeer, permisos.AnamnesisEscribir), h.HistorialAnamnesis)
	e.permiso("POST", "/pacientes/:id/anamnesis", p(permisos.AnamnesisEscribir), h.AgregarAnamnesis)

	e.permiso("GET", "/citas", p(permisos.CitasLeer), h.ListarCitas)
	e.permiso("GET", "/citas/:id", p(permisos.CitasLeer), h.ObtenerCita)
	e.permiso("POST", "/citas", p(permisos.CitasEscribir), h.CrearCita)
	e.permiso("PATCH", "/citas/:id/reprogramar", p(permisos.CitasEscribir), h.ReprogramarCita)
	e.permiso("POST", "/citas/:id/estado", p(permisos.CitasEscribir), h.CambiarEstadoCita)
	e.permiso("GET", "/notificaciones", p(permisos.CitasLeer), h.Bandeja)
	e.permiso("POST", "/notificaciones/procesar", p(permisos.CitasEscribir), h.Procesar)

	// Cualquier otra ruta /api → 404 JSON.
	app.Use("/api", func(c *fiber.Ctx) error {
		return c.Status(404).JSON(models.NewAPIError(404, "NO_ENCONTRADO", "Recurso no encontrado."))
	})
	if h.Cfg.StaticDir != "" {
		ServirFrontend(app, h.Cfg.StaticDir)
	}
	return app, e.registro
}

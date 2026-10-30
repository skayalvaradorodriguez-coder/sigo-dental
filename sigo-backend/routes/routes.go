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
	cfgFiber := fiber.Config{AppName: "SIGO-Dental API", ErrorHandler: manejadorErrores, BodyLimit: 11 * 1024 * 1024} // 10 MB de archivo + formulario; el JSON se limita a 1 MB en leerJSON
	if h.Cfg.EsProduccion() {
		cfgFiber.ProxyHeader = fiber.HeaderXForwardedFor // detrás de Render / proxy inverso
	}
	app := fiber.New(cfgFiber)
	app.Use(recover.New())
	app.Use(helmet.New(helmet.Config{
		// CSP: sólo recursos propios, sin scripts ni estilos en línea (los estilos de React se aplican por
		// CSSOM, que la CSP no restringe); blob:/data: para la firma, los adjuntos y el visor de PDF.
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; " +
			"font-src 'self'; connect-src 'self'; frame-src 'self' blob:; worker-src 'self'; manifest-src 'self'; object-src 'none'; " +
			"base-uri 'self'; form-action 'self'; frame-ancestors 'self'",
		PermissionPolicy: "camera=(), microphone=(), geolocation=(), payment=()",
		XFrameOptions:    "SAMEORIGIN",
		ReferrerPolicy:   "no-referrer",
	}))
	if h.Cfg.EsProduccion() {
		// HSTS: el proxy de la plataforma termina TLS; el navegador debe usar siempre HTTPS (RNF-04).
		app.Use(func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderStrictTransportSecurity, "max-age=31536000; includeSubDomains")
			return c.Next()
		})
	}
	app.Use(cors.New(cors.Config{AllowOrigins: h.Cfg.WebOrigin, AllowHeaders: "Authorization, Content-Type", AllowMethods: "GET,POST,PUT,PATCH,DELETE"}))

	// Datos clínicos y financieros: ninguna respuesta de la API se guarda en cachés del navegador ni
	// intermedias (la caché del servidor es Redis).
	app.Use("/api", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.Next()
	})
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
	lectoresCatalogo := p(permisos.CatalogosGestionar, permisos.CitasLeer, permisos.PacientesLeer, permisos.InventarioLeer, permisos.HonorariosTodos)
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

	// ---------------------------------------------------------------- Etapa 3
	// Expediente clínico: lectura con clinica.leer; registro con clinica.escribir; firmar con clinica.firmar.
	e.permiso("GET", "/pacientes/:id/odontograma", p(permisos.ClinicaLeer), h.ObtenerOdontograma)
	e.permiso("POST", "/pacientes/:id/odontograma", p(permisos.ClinicaEscribir), h.MarcarOdontograma)
	e.permiso("GET", "/pacientes/:id/periodontogramas", p(permisos.ClinicaLeer), h.ListarPeriodontogramas)
	e.permiso("POST", "/pacientes/:id/periodontogramas", p(permisos.ClinicaEscribir), h.CrearPeriodontograma)
	e.permiso("GET", "/pacientes/:id/notas", p(permisos.ClinicaLeer), h.ListarNotas)
	e.permiso("POST", "/pacientes/:id/notas", p(permisos.ClinicaEscribir), h.CrearNota)
	e.permiso("PUT", "/notas/:id", p(permisos.ClinicaEscribir), h.ActualizarNota)
	e.permiso("POST", "/notas/:id/firmar", p(permisos.ClinicaFirmar), h.FirmarNota)
	e.permiso("POST", "/notas/:id/adendas", p(permisos.ClinicaFirmar), h.AdendaNota)
	e.permiso("GET", "/pacientes/:id/procedimientos", p(permisos.ClinicaLeer), h.ListarRealizados)
	e.permiso("POST", "/pacientes/:id/procedimientos", p(permisos.ClinicaFirmar), h.RegistrarRealizado)
	e.permiso("POST", "/procedimientos-realizados/:id/adendas", p(permisos.ClinicaFirmar), h.AdendaRealizado)

	// ---------------------------------------------------------------- Etapa 4
	// Presupuestos (RF-07, RF-08) → tareas agendables.
	e.permiso("GET", "/presupuestos", p(permisos.PresupuestosLeer), h.ListarPresupuestos)
	e.permiso("GET", "/presupuestos/:id", p(permisos.PresupuestosLeer), h.ObtenerPresupuesto)
	e.permiso("GET", "/pacientes/:id/presupuestos", p(permisos.PresupuestosLeer), h.ListarPresupuestosPaciente)
	e.permiso("POST", "/pacientes/:id/presupuestos", p(permisos.PresupuestosEscribir), h.CrearPresupuesto)
	e.permiso("PUT", "/presupuestos/:id", p(permisos.PresupuestosEscribir), h.ActualizarPresupuesto)
	// Descuento sobre el tope: lo autoriza OTRO odontólogo (cuatro ojos) con su código MFA.
	e.permiso("POST", "/presupuestos/:id/autorizar-descuentos", p(permisos.PresupuestosEscribir, permisos.ClinicaFirmar), h.AutorizarDescuentos)
	e.permiso("POST", "/presupuestos/:id/presentar", p(permisos.PresupuestosEscribir), h.PresentarPresupuesto)
	e.permiso("POST", "/presupuestos/:id/decision", p(permisos.PresupuestosAprobar), h.DecidirPresupuesto)
	e.alguno("GET", "/pacientes/:id/tareas", p(permisos.CitasLeer, permisos.PresupuestosLeer), h.ListarTareas)
	// Consentimientos (RF-06): firma manuscrita en pantalla u OTP.
	e.alguno("GET", "/consentimientos/plantillas", p(permisos.ConsentimientosGestionar, permisos.CatalogosGestionar), h.ListarPlantillas)
	e.permiso("POST", "/consentimientos/plantillas", p(permisos.CatalogosGestionar), h.CrearPlantilla)
	e.permiso("GET", "/consentimientos", p(permisos.ConsentimientosGestionar), h.ListarConsentimientos)
	e.permiso("GET", "/consentimientos/:id", p(permisos.ConsentimientosGestionar), h.ObtenerConsentimiento)
	e.permiso("POST", "/consentimientos/:id/verificar-otp", p(permisos.ConsentimientosGestionar), h.VerificarOTPConsentimiento)
	e.alguno("GET", "/pacientes/:id/consentimientos", p(permisos.ConsentimientosGestionar, permisos.ClinicaLeer), h.ListarConsentimientosPaciente)
	e.permiso("POST", "/pacientes/:id/consentimientos/vista-previa", p(permisos.ConsentimientosGestionar), h.VistaPreviaConsentimiento)
	e.permiso("POST", "/pacientes/:id/consentimientos", p(permisos.ConsentimientosGestionar), h.CrearConsentimiento)
	// Recetas (propuesta P-01) y archivos adjuntos.
	e.permiso("GET", "/pacientes/:id/recetas", p(permisos.ClinicaLeer), h.ListarRecetas)
	e.permiso("POST", "/pacientes/:id/recetas", p(permisos.RecetasEscribir), h.CrearReceta)
	e.permiso("GET", "/pacientes/:id/archivos", p(permisos.ClinicaLeer), h.ListarArchivos)
	e.permiso("POST", "/pacientes/:id/archivos", p(permisos.ClinicaEscribir), h.SubirArchivo)
	e.permiso("GET", "/archivos/:id/contenido", p(permisos.ClinicaLeer), h.DescargarArchivo)

	// ---------------------------------------------------------------- Etapa 5
	// Caja (RF-12) con cierre ciego (RN-02): el recepcionista (caja.operar) nunca recibe el teórico;
	// el Auditor (caja.revisar) concilia. Comprobantes por el proveedor de facturación (SIMULADO).
	e.permiso("GET", "/caja/sesion", p(permisos.CajaOperar), h.MiCaja)
	e.permiso("POST", "/caja/sesion", p(permisos.CajaOperar), h.AbrirCaja)
	e.permiso("POST", "/caja/sesion/cerrar", p(permisos.CajaOperar), h.CerrarCaja)
	e.permiso("GET", "/caja/cobros", p(permisos.CajaOperar), h.MisCobros)
	e.permiso("GET", "/pacientes/:id/por-cobrar", p(permisos.CajaOperar), h.PorCobrar)
	e.alguno("GET", "/pacientes/:id/cobros", p(permisos.CajaOperar, permisos.CajaRevisar), h.CobrosPaciente)
	e.permiso("POST", "/cobros", p(permisos.CajaOperar, permisos.FacturacionEmitir), h.CrearCobro)
	e.alguno("GET", "/cobros/:id", p(permisos.CajaOperar, permisos.CajaRevisar), h.ObtenerCobro)
	e.permiso("POST", "/cobros/:id/anular", p(permisos.CajaOperar, permisos.FacturacionEmitir), h.AnularCobro)
	e.permiso("POST", "/cobros/:id/comprobante", p(permisos.FacturacionEmitir), h.ReintentarComprobante)
	e.alguno("GET", "/facturacion/proveedor", p(permisos.CajaOperar, permisos.CajaRevisar), h.EstadoFacturacion)
	e.permiso("GET", "/caja/cierres", p(permisos.CajaRevisar), h.ListarCierres)
	e.permiso("GET", "/caja/cierres/:id", p(permisos.CajaRevisar), h.ObtenerCierre)
	e.permiso("POST", "/caja/cierres/:id/revisar", p(permisos.CajaRevisar), h.RevisarCierre)
	e.permiso("GET", "/caja/cuentas-por-cobrar", p(permisos.CajaRevisar), h.CuentasPorCobrar)

	// ---------------------------------------------------------------- Etapa 6
	// Inventario (RF-14): stock = suma de movimientos; descargo automático al registrar un procedimiento.
	e.permiso("GET", "/inventario/ubicaciones", p(permisos.InventarioLeer), h.ListarUbicaciones)
	e.permiso("POST", "/inventario/ubicaciones", p(permisos.InventarioConfigurar), h.CrearUbicacion)
	e.permiso("GET", "/inventario/insumos", p(permisos.InventarioLeer), h.ListarInsumos)
	e.permiso("POST", "/inventario/insumos", p(permisos.InventarioConfigurar), h.CrearInsumo)
	e.permiso("PATCH", "/inventario/insumos/:id", p(permisos.InventarioConfigurar), h.ActualizarInsumo)
	e.permiso("GET", "/inventario/stock", p(permisos.InventarioLeer), h.Stock)
	e.permiso("GET", "/inventario/movimientos", p(permisos.InventarioLeer), h.Kardex)
	e.permiso("POST", "/inventario/movimientos", p(permisos.InventarioEscribir), h.RegistrarMovimiento)
	e.permiso("GET", "/inventario/recetas", p(permisos.InventarioLeer), h.RecetasConsumo)
	e.permiso("PUT", "/inventario/recetas/:procedimientoId", p(permisos.InventarioConfigurar), h.GuardarReceta)
	// Honorarios (RF-13): el odontólogo ve lo suyo; el Auditor configura reglas y liquida.
	honorarios := p(permisos.HonorariosPropios, permisos.HonorariosTodos)
	e.alguno("GET", "/honorarios/reglas", p(permisos.HonorariosPropios, permisos.HonorariosTodos, permisos.HonorariosConfigurar), h.ListarReglas)
	e.permiso("POST", "/honorarios/reglas", p(permisos.HonorariosConfigurar), h.CrearRegla)
	e.alguno("GET", "/honorarios/calculo", honorarios, h.CalcularHonorarios)
	e.alguno("GET", "/honorarios/liquidaciones", honorarios, h.ListarLiquidaciones)
	e.alguno("GET", "/honorarios/liquidaciones/:id", honorarios, h.ObtenerLiquidacion)
	e.permiso("POST", "/honorarios/liquidaciones", p(permisos.HonorariosTodos), h.CrearLiquidacion)

	// Cualquier otra ruta /api → 404 JSON.
	app.Use("/api", func(c *fiber.Ctx) error {
		return c.Status(404).JSON(models.NewAPIError(404, "NO_ENCONTRADO", "Recurso no encontrado."))
	})
	if h.Cfg.StaticDir != "" {
		ServirFrontend(app, h.Cfg.StaticDir)
	}
	return app, e.registro
}

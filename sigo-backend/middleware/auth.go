// Package middleware: autenticación (JWT + sesión de servidor), pasos pendientes del ingreso
// (MFA, cambio de contraseña) y autorización por permisos.
package middleware

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/models"
	"sigo-backend/permisos"
	"sigo-backend/repository"
	"sigo-backend/seguridad"
)

// Pasos pendientes del inicio de sesión.
const (
	PasoVerificarMFA    = "VERIFICAR_MFA"
	PasoCambiarPassword = "CAMBIAR_PASSWORD"
	PasoConfigurarMFA   = "CONFIGURAR_MFA"
)

// SiguientePaso: "" = sesión completa.
func SiguientePaso(u *models.Usuario, s *models.Sesion) string {
	switch {
	case u.MFAActivo && !s.MFAVerificado:
		return PasoVerificarMFA
	case u.DebeCambiarPassword:
		return PasoCambiarPassword
	case !u.MFAActivo:
		return PasoConfigurarMFA
	}
	return ""
}

// Contexto de autenticación guardado en c.Locals.
type Auth struct {
	Usuario  *models.Usuario
	Sesion   *models.Sesion
	Permisos []string
}

func (a *Auth) Puede(ps ...string) bool {
	for _, p := range ps {
		if slices.Contains(a.Permisos, p) {
			return true
		}
	}
	return false
}

func Actual(c *fiber.Ctx) *Auth {
	a, _ := c.Locals("auth").(*Auth)
	return a
}

// Deps que necesita el middleware.
type Deps struct {
	Repo        repository.DBRepository
	JWTSecret   string
	Inactividad func(ctx context.Context) time.Duration
	Auditar     func(c *fiber.Ctx, e repository.Evento)
}

func responder(c *fiber.Ctx, e models.APIError) error { return c.Status(e.Status).JSON(e) }

var (
	ErrNoAutenticado  = models.NewAPIError(401, "NO_AUTENTICADO", "Debe iniciar sesión.")
	ErrSesionExpirada = models.NewAPIError(401, "SESION_EXPIRADA", "Su sesión se cerró por inactividad.")
	ErrProhibido      = models.NewAPIError(403, "PROHIBIDO", "No tiene permiso para realizar esta acción.")
	ErrMFARequerido   = models.NewAPIError(403, "MFA_REQUERIDO", "Debe completar la verificación en dos pasos para acceder a esta información.")
	ErrCambioPassword = models.NewAPIError(403, "CAMBIO_PASSWORD_REQUERIDO", "Debe cambiar su contraseña antes de continuar.")
	ErrConfigurarMFA  = models.NewAPIError(403, "MFA_CONFIGURACION_REQUERIDA", "Debe configurar la verificación en dos pasos antes de continuar.")
)

// No se escribe ultima_actividad en cada petición, sólo si pasaron más de 15 s.
const intervaloToque = 15 * time.Second

// RequireAuth valida el JWT (firma, emisor, vencimiento) y la sesión de servidor asociada:
// revocada, vencida, usuario inactivo o inactividad > N minutos ⇒ 401 (RNF-05).
func RequireAuth(d Deps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get(fiber.HeaderAuthorization)
		token, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || token == "" {
			return responder(c, ErrNoAutenticado)
		}
		claims, err := seguridad.ValidarToken(d.JWTSecret, token)
		if err != nil {
			return responder(c, ErrNoAutenticado)
		}
		ctx := c.UserContext()
		s, err := d.Repo.SesionPorJTI(ctx, claims.ID)
		if err != nil || s.RevocadaEn != nil || s.UsuarioID != claims.Subject {
			return responder(c, ErrNoAutenticado)
		}
		u, err := d.Repo.UsuarioPorID(ctx, s.UsuarioID)
		ahora := time.Now()
		if err != nil || !u.Activo || !s.ExpiraEn.After(ahora) {
			_ = d.Repo.RevocarSesion(ctx, s.JTI, "VENCIDA_O_INACTIVO")
			return responder(c, ErrNoAutenticado)
		}
		limite := d.Inactividad(ctx)
		if ahora.Sub(s.UltimaActividad) > limite {
			_ = d.Repo.RevocarSesion(ctx, s.JTI, "INACTIVIDAD")
			d.Auditar(c, repository.Evento{Accion: "SESION_EXPIRADA", Entidad: "auth", UsuarioID: u.ID,
				Detalle: map[string]any{"minutosInactividad": int(limite.Minutes())}})
			return responder(c, ErrSesionExpirada)
		}
		if ahora.Sub(s.UltimaActividad) > intervaloToque {
			_ = d.Repo.TocarSesion(ctx, s.JTI)
			s.UltimaActividad = ahora
		}
		c.Locals("auth", &Auth{Usuario: u, Sesion: s, Permisos: permisos.PorRol[u.Rol]})
		return c.Next()
	}
}

// RequireCompleta exige que no quede ningún paso pendiente (MFA verificado, contraseña definitiva).
// Como todos los permisos dan acceso a información clínica, financiera o de seguridad,
// MFA es obligatorio para todos (supuesto S-07, RNF-05).
func RequireCompleta() fiber.Handler {
	return func(c *fiber.Ctx) error {
		a := Actual(c)
		switch SiguientePaso(a.Usuario, a.Sesion) {
		case PasoVerificarMFA:
			return responder(c, ErrMFARequerido)
		case PasoCambiarPassword:
			return responder(c, ErrCambioPassword)
		case PasoConfigurarMFA:
			return responder(c, ErrConfigurarMFA)
		}
		return c.Next()
	}
}

// RequirePermisos exige TODOS los permisos indicados.
func RequirePermisos(ps ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		a := Actual(c)
		for _, p := range ps {
			if !slices.Contains(a.Permisos, p) {
				return responder(c, ErrProhibido)
			}
		}
		return c.Next()
	}
}

// RequireAlguno exige AL MENOS UNO de los permisos indicados.
func RequireAlguno(ps ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !Actual(c).Puede(ps...) {
			return responder(c, ErrProhibido)
		}
		return c.Next()
	}
}

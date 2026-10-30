// Package repository: acceso a PostgreSQL con SQL explícito (pgx). Los controladores
// dependen de la interfaz DBRepository, no de la implementación (inyección de dependencias).
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"sigo-backend/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNoEncontrado = errors.New("no encontrado")
	ErrDuplicado    = errors.New("duplicado")
	ErrConflicto    = errors.New("conflicto")
	// ErrSuperposicion: la restricción EXCLUDE rechazó una cita superpuesta (RN-03).
	ErrSuperposicion = errors.New("superposición de citas en el sillón")
)

// ErrValidacion lleva un mensaje para el usuario (regla de negocio no cumplida).
type ErrValidacion struct{ Mensaje, Codigo string }

func (e *ErrValidacion) Error() string { return e.Mensaje }

func Validacion(codigo, mensaje string) error {
	return &ErrValidacion{Mensaje: mensaje, Codigo: codigo}
}

// Códigos SQLSTATE usados.
const (
	pgUnico      = "23505"
	pgFK         = "23503"
	pgExclusion  = "23P01"
	pgDisparador = "P0001"
)

func codigoPG(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// traducir convierte errores de PostgreSQL en errores del dominio.
func traducir(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNoEncontrado
	}
	switch codigoPG(err) {
	case pgUnico:
		return ErrDuplicado
	case pgExclusion:
		return ErrSuperposicion
	case pgFK:
		return Validacion("REFERENCIA_INVALIDA", "Un registro relacionado no existe.")
	}
	return err
}

// Postgres implementa DBRepository sobre un pool de pgx.
type Postgres struct{ pool *pgxpool.Pool }

func NuevoPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (r *Postgres) Ping(ctx context.Context) error { return r.pool.Ping(ctx) }

// escaparLike evita que % y _ del usuario actúen como comodines en ILIKE.
func escaparLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func paginar(page, limit int) (int, int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit, (page - 1) * limit
}

// DBRepository es la interfaz que usan los controladores (inyección de dependencias):
// en pruebas o en otra base de datos se puede sustituir la implementación.
type DBRepository interface {
	Ping(ctx context.Context) error
	// identidad
	UsuarioPorEmail(ctx context.Context, email string) (*models.Usuario, error)
	UsuarioPorID(ctx context.Context, id string) (*models.Usuario, error)
	ListarUsuarios(ctx context.Context) ([]models.Usuario, error)
	CrearUsuario(ctx context.Context, n NuevoUsuario) (*models.Usuario, error)
	ActualizarUsuario(ctx context.Context, id string, c CambiosUsuario) (*models.Usuario, error)
	ContarAdminsActivos(ctx context.Context, excepto string) (int, error)
	RegistrarFalloLogin(ctx context.Context, id string, maximo int, bloqueo time.Duration) (*time.Time, int, error)
	RegistrarLoginExitoso(ctx context.Context, id string) error
	GuardarSecretoMFA(ctx context.Context, id, secreto string) error
	ActivarMFA(ctx context.Context, id string, paso int64) error
	GuardarPasoMFA(ctx context.Context, id string, paso int64) error
	ReiniciarMFA(ctx context.Context, id string) error
	CambiarPassword(ctx context.Context, id, hash string, debeCambiar bool) error
	CrearSesion(ctx context.Context, jti, usuarioID string, mfa bool, ip, ua string, expira time.Time) error
	SesionPorJTI(ctx context.Context, jti string) (*models.Sesion, error)
	TocarSesion(ctx context.Context, jti string) error
	RevocarSesion(ctx context.Context, jti, motivo string) error
	RevocarSesionesUsuario(ctx context.Context, usuarioID, exceptoJTI, motivo string) error
	SesionesAbiertas(ctx context.Context, usuarioID string) (int, error)
	// ajustes y auditoría
	LeerAjuste(ctx context.Context, clave string) (int, error)
	GuardarAjuste(ctx context.Context, clave string, valor int, usuarioID string) error
	RegistrarAuditoria(ctx context.Context, e Evento) error
	ListarAuditoria(ctx context.Context, f FiltroAuditoria) (models.Pagina[models.EventoAuditoria], error)
	VerificarAuditoria(ctx context.Context) (ResultadoVerificacion, error)
	ContarEventosSeguridad(ctx context.Context, desde time.Time) (map[string]int, error)
	ResumenUsuarios(ctx context.Context) ([]UsuariosPorRol, error)
	// catálogos
}

var _ DBRepository = (*Postgres)(nil)

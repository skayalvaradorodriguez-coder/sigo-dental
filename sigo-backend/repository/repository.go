// Package repository: acceso a PostgreSQL con SQL explícito (pgx). Los controladores
// dependen de la interfaz DBRepository, no de la implementación (inyección de dependencias).
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"sigo-backend/models"
	"sigo-backend/notificaciones"

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
	EstructuraSedes(ctx context.Context, soloActivos bool) ([]models.Sede, error)
	SillonesActivos(ctx context.Context) ([]models.Sillon, error)
	SillonActivo(ctx context.Context, id string) (bool, error)
	Procedimientos(ctx context.Context, soloActivos bool) ([]models.Procedimiento, error)
	Convenios(ctx context.Context, soloActivos bool) ([]models.Convenio, error)
	Odontologos(ctx context.Context) ([]models.Odontologo, error)
	EsOdontologoActivo(ctx context.Context, id string) (bool, error)
	ProcedimientoActivo(ctx context.Context, id string) (bool, error)
	CrearCatalogo(ctx context.Context, cat string, valores map[string]any) (map[string]any, error)
	ActualizarCatalogo(ctx context.Context, cat, id string, valores map[string]any) (map[string]any, map[string]any, error)
	// pacientes
	BuscarPacientes(ctx context.Context, q string, page, limit int, hoy time.Time) (models.Pagina[models.Paciente], error)
	PacientePorID(ctx context.Context, id string, hoy time.Time) (*models.Paciente, error)
	CrearPaciente(ctx context.Context, d DatosPaciente, tutor *models.Tutor, creadoPor string) (string, error)
	ActualizarPaciente(ctx context.Context, id string, d DatosPaciente, tutor *models.Tutor, tutorAccion string) error
	HistorialAnamnesis(ctx context.Context, pacienteID string) ([]models.Anamnesis, error)
	AgregarAnamnesis(ctx context.Context, pacienteID string, a models.Anamnesis, usuarioID string) (*models.Anamnesis, error)
	ContactoPaciente(ctx context.Context, id string) (*DatosContacto, error)
	ResumenAlertas(ctx context.Context, ids []string) (map[string][]string, map[string]bool, error)
	ConteoPacientes(ctx context.Context) (int, int, error)
	// agenda y notificaciones
	ListarCitas(ctx context.Context, f FiltroCitas) ([]models.Cita, error)
	CitaPorID(ctx context.Context, id string) (*models.Cita, error)
	BuscarSuperposicion(ctx context.Context, sillonID string, inicio, fin time.Time, excluir string) (*Conflicto, error)
	CrearCita(ctx context.Context, n NuevaCita, msgs []notificaciones.Planificada) (string, error)
	CitaBase(ctx context.Context, id string) (*CitaBase, error)
	Reprogramar(ctx context.Context, actual *CitaBase, sillonID, odontologoID string, inicio, fin time.Time, usuarioID string, msgs []notificaciones.Planificada) error
	CambiarEstadoCita(ctx context.Context, actual *CitaBase, nuevo string, nota *string, usuarioID string) error
	ConteoCitasPorEstado(ctx context.Context, desde, hasta time.Time, odontologoID string) (map[string]int, error)
	NotificacionesDeCita(ctx context.Context, citaID string) ([]models.Notificacion, error)
	BandejaNotificaciones(ctx context.Context, page, limit int) ([]models.Notificacion, error)
	ProcesarVencidas(ctx context.Context, limite int, ahora time.Time, enviar func(EnvioPendiente) ResultadoEnvio) (int, int, error)
	// Etapa 3 — expediente clínico
	EventosOdontograma(ctx context.Context, pacienteID string) ([]models.EventoOdontograma, error)
	AgregarEventoOdontograma(ctx context.Context, pacienteID string, diente int, superficie *string, hallazgo, accion string, nota *string, usuarioID string) (int64, error)
	CrearPeriodontograma(ctx context.Context, pacienteID string, obs *string, dientes []models.PerioDiente, usuarioID string) (string, error)
	Periodontogramas(ctx context.Context, pacienteID string) ([]models.Periodontograma, error)
	NotasPaciente(ctx context.Context, pacienteID string) ([]models.NotaSOEP, error)
	NotaPorID(ctx context.Context, id string) (*models.NotaSOEP, error)
	CrearNota(ctx context.Context, pacienteID string, citaID *string, autorID string, c ContenidoSOEP, horasBloqueo int) (string, error)
	ActualizarNota(ctx context.Context, id string, c ContenidoSOEP) error
	FirmarNota(ctx context.Context, id, firmanteID string, firmadaEn time.Time, hash string) error
	AgregarAdendaNota(ctx context.Context, notaID, autorID, texto string, creadoEn time.Time, hash string) error
	RegistrarRealizado(ctx context.Context, n NuevoRealizado) (string, error)
	RealizadosPaciente(ctx context.Context, pacienteID string) ([]models.ProcedimientoRealizado, error)
	PacienteDeRealizado(ctx context.Context, id string) (string, error)
	AgregarAdendaRealizado(ctx context.Context, realizadoID, autorID, texto string, creadoEn time.Time, hash string) error
	CitaDelPaciente(ctx context.Context, citaID, pacienteID string) (bool, error)
}

var _ DBRepository = (*Postgres)(nil)

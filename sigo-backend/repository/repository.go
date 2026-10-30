// Package repository: acceso a PostgreSQL con SQL explícito (pgx). Los controladores
// dependen de la interfaz DBRepository, no de la implementación (inyección de dependencias).
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"sigo-backend/domain"
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
	AnamnesisVigentes(ctx context.Context, ids []string) (map[string]*models.Anamnesis, error)
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
	// Etapa 4 — presupuestos, tareas, consentimientos, recetas y archivos
	PresupuestosPaciente(ctx context.Context, pacienteID string) ([]models.Presupuesto, error)
	PresupuestosPorEstado(ctx context.Context, estado string) ([]models.Presupuesto, error)
	PresupuestoPorID(ctx context.Context, id string) (*models.Presupuesto, error)
	CrearPresupuesto(ctx context.Context, pacienteID, odontologoID, titulo string, obs *string, items []ItemNuevo) (string, error)
	ReemplazarBorrador(ctx context.Context, id, titulo string, obs *string, items []ItemNuevo) error
	AutorizarDescuentos(ctx context.Context, id string, itemIDs []string, usuarioID string) error
	PresentarPresupuesto(ctx context.Context, id, hash string) error
	DecidirPresupuesto(ctx context.Context, id string, decisiones map[string]string, estado, usuarioID string, nota *string) error
	TareasPaciente(ctx context.Context, pacienteID string) ([]models.Tarea, error)
	TareaPorID(ctx context.Context, id string) (*models.Tarea, string, error)
	PlantillasVigentes(ctx context.Context) ([]models.PlantillaConsentimiento, error)
	PlantillaPorID(ctx context.Context, id string) (*models.PlantillaConsentimiento, error)
	NuevaVersionPlantilla(ctx context.Context, codigo, titulo, texto string, procedimientoID *string, usuarioID string) (string, error)
	ConsentimientosPaciente(ctx context.Context, pacienteID string) ([]models.Consentimiento, error)
	ConsentimientosRecientes(ctx context.Context, estado string) ([]models.Consentimiento, error)
	ConsentimientoPorID(ctx context.Context, id string) (*models.Consentimiento, error)
	CrearConsentimiento(ctx context.Context, n NuevoConsentimiento) (string, error)
	SumarIntentoOTP(ctx context.Context, id string) (int, error)
	FirmarConsentimiento(ctx context.Context, id string, firmado time.Time, hash string) error
	CrearReceta(ctx context.Context, pacienteID, odontologoID string, indicaciones *string, alertas []domain.AlertaReceta, items []models.RecetaItem, emitida time.Time, hash string) (string, error)
	RecetasPaciente(ctx context.Context, pacienteID string) ([]models.Receta, error)
	RegistrarArchivo(ctx context.Context, a models.Archivo, usuarioID string) (string, error)
	ArchivosPaciente(ctx context.Context, pacienteID string) ([]models.Archivo, error)
	ArchivoPorID(ctx context.Context, id string) (*models.Archivo, error)

	// Etapa 5 — caja y facturación
	CajaAbierta(ctx context.Context, cajeroID string) (*models.CajaSesion, error)
	AbrirCaja(ctx context.Context, cajeroID, fondo string) (string, error)
	CerrarCaja(ctx context.Context, sesionID string, declarado map[string]string, nota *string) error
	Cierres(ctx context.Context, estado string) ([]models.CierreCaja, error)
	CierrePorID(ctx context.Context, id string) (*models.CierreCaja, error)
	RevisarCierre(ctx context.Context, id, usuarioID string, nota *string) error
	PorCobrar(ctx context.Context, pacienteID string) ([]models.PorCobrar, error)
	CrearCobro(ctx context.Context, n NuevoCobro) (string, error)
	AnularCobro(ctx context.Context, cobroID, motivo, usuarioID string) error
	CobrosSesion(ctx context.Context, sesionID string) ([]models.Cobro, error)
	CobrosPaciente(ctx context.Context, pacienteID string) ([]models.Cobro, error)
	CobroPorID(ctx context.Context, id string) (*models.Cobro, error)
	EstadoCajaDeCobro(ctx context.Context, cobroID string) (estado, cajeroID string, err error)
	SiguienteSecuencial(ctx context.Context, tipo string) (int64, error)
	RegistrarComprobante(ctx context.Context, n NuevoComprobante) error
	CuentasPorCobrarSeguros(ctx context.Context) ([]models.CuentaSeguro, error)
	CierresPorRevisar(ctx context.Context) (int, error)

	// Etapa 6 — inventario y honorarios
	Ubicaciones(ctx context.Context) ([]models.Ubicacion, error)
	Insumos(ctx context.Context, todos bool) ([]models.Insumo, error)
	InsumoActivo(ctx context.Context, id string) (bool, error)
	UbicacionActiva(ctx context.Context, id string) (bool, error)
	CrearInsumo(ctx context.Context, d InsumoDatos) (string, error)
	ActualizarInsumo(ctx context.Context, id string, d InsumoDatos) error
	Stock(ctx context.Context, ubicacionID string) ([]models.StockItem, error)
	Movimientos(ctx context.Context, insumoID, realizadoID string) ([]models.Movimiento, error)
	RegistrarMovimiento(ctx context.Context, n NuevoMovimiento) (string, error)
	Transferir(ctx context.Context, insumoID, origen, destino, cantidad, usuarioID string, motivo *string) (string, error)
	RecetasConsumo(ctx context.Context) ([]models.RecetaConsumo, error)
	ReemplazarReceta(ctx context.Context, procedimientoID string, lineas []LineaRecetaNueva) error
	ReglasHonorario(ctx context.Context) ([]models.ReglaHonorario, error)
	CrearRegla(ctx context.Context, odontologoID, procedimientoID *string, tipo, valor, vigente, usuarioID string) (string, error)
	LineasPorLiquidar(ctx context.Context, odontologoID string, desde, hasta time.Time) ([]models.LineaHonorario, error)
	CrearLiquidacion(ctx context.Context, odontologoID, desde, hasta, total, usuarioID string, lineas []models.LineaHonorario) (string, error)
	Liquidaciones(ctx context.Context, odontologoID string) ([]models.Liquidacion, error)
	LiquidacionPorID(ctx context.Context, id string) (*models.Liquidacion, error)
}

var _ DBRepository = (*Postgres)(nil)

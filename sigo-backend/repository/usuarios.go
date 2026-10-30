package repository

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"sigo-backend/models"
)

const colsUsuario = `id, email, nombre, rol, registro_profesional, activo, mfa_activo, debe_cambiar_password,
	bloqueado_hasta, ultimo_acceso, creado_en, password_hash, mfa_secreto, mfa_ultimo_paso, intentos_fallidos`

func scanUsuario(row pgx.Row) (*models.Usuario, error) {
	u := &models.Usuario{}
	err := row.Scan(&u.ID, &u.Email, &u.Nombre, &u.Rol, &u.RegistroProfesional, &u.Activo, &u.MFAActivo,
		&u.DebeCambiarPassword, &u.BloqueadoHasta, &u.UltimoAcceso, &u.CreadoEn, &u.PasswordHash,
		&u.MFASecreto, &u.MFAUltimoPaso, &u.IntentosFallidos)
	if err != nil {
		return nil, traducir(err)
	}
	return u, nil
}

func (r *Postgres) UsuarioPorEmail(ctx context.Context, email string) (*models.Usuario, error) {
	return scanUsuario(r.pool.QueryRow(ctx, `SELECT `+colsUsuario+` FROM usuarios WHERE email=$1`, strings.ToLower(strings.TrimSpace(email))))
}

func (r *Postgres) UsuarioPorID(ctx context.Context, id string) (*models.Usuario, error) {
	return scanUsuario(r.pool.QueryRow(ctx, `SELECT `+colsUsuario+` FROM usuarios WHERE id=$1`, id))
}

func (r *Postgres) ListarUsuarios(ctx context.Context) ([]models.Usuario, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+colsUsuario+` FROM usuarios ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Usuario{}
	for rows.Next() {
		u, err := scanUsuario(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

type NuevoUsuario struct {
	Email, Nombre, Rol, PasswordHash string
	RegistroProfesional              *string
	DebeCambiarPassword, MFAActivo   bool
	MFASecreto                       *string
}

func (r *Postgres) CrearUsuario(ctx context.Context, n NuevoUsuario) (*models.Usuario, error) {
	return scanUsuario(r.pool.QueryRow(ctx, `INSERT INTO usuarios (email, nombre, rol, registro_profesional, password_hash,
		debe_cambiar_password, mfa_activo, mfa_secreto) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+colsUsuario,
		strings.ToLower(strings.TrimSpace(n.Email)), n.Nombre, n.Rol, n.RegistroProfesional, n.PasswordHash,
		n.DebeCambiarPassword, n.MFAActivo, n.MFASecreto))
}

type CambiosUsuario struct {
	Nombre              *string
	Rol                 *string
	RegistroProfesional **string
	Activo              *bool
}

func (r *Postgres) ActualizarUsuario(ctx context.Context, id string, c CambiosUsuario) (*models.Usuario, error) {
	actual, err := r.UsuarioPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	nombre, rol, reg, activo := actual.Nombre, actual.Rol, actual.RegistroProfesional, actual.Activo
	if c.Nombre != nil {
		nombre = *c.Nombre
	}
	if c.Rol != nil {
		rol = *c.Rol
	}
	if c.RegistroProfesional != nil {
		reg = *c.RegistroProfesional
	}
	if c.Activo != nil {
		activo = *c.Activo
	}
	return scanUsuario(r.pool.QueryRow(ctx, `UPDATE usuarios SET nombre=$2, rol=$3, registro_profesional=$4, activo=$5,
		actualizado_en=now() WHERE id=$1 RETURNING `+colsUsuario, id, nombre, rol, reg, activo))
}

func (r *Postgres) ContarAdminsActivos(ctx context.Context, excepto string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM usuarios WHERE rol='ADMIN_TI' AND activo AND id<>$1`, excepto).Scan(&n)
	return n, err
}

// RegistrarFalloLogin incrementa el contador; si llega al máximo, bloquea la cuenta.
// Devuelve el bloqueo aplicado (nil si no se bloqueó).
func (r *Postgres) RegistrarFalloLogin(ctx context.Context, id string, maximo int, bloqueo time.Duration) (*time.Time, int, error) {
	var intentos int
	var hasta *time.Time
	err := r.pool.QueryRow(ctx, `UPDATE usuarios SET
		intentos_fallidos = CASE WHEN intentos_fallidos + 1 >= $2 THEN 0 ELSE intentos_fallidos + 1 END,
		bloqueado_hasta = CASE WHEN intentos_fallidos + 1 >= $2 THEN now() + $3::interval ELSE bloqueado_hasta END,
		actualizado_en = now()
		WHERE id=$1 RETURNING intentos_fallidos, CASE WHEN intentos_fallidos = 0 THEN bloqueado_hasta END`,
		id, maximo, bloqueo.String()).Scan(&intentos, &hasta)
	return hasta, intentos, err
}

func (r *Postgres) RegistrarLoginExitoso(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE usuarios SET intentos_fallidos=0, bloqueado_hasta=NULL, ultimo_acceso=now(), actualizado_en=now() WHERE id=$1`, id)
	return err
}

func (r *Postgres) GuardarSecretoMFA(ctx context.Context, id, secreto string) error {
	_, err := r.pool.Exec(ctx, `UPDATE usuarios SET mfa_secreto=$2, actualizado_en=now() WHERE id=$1`, id, secreto)
	return err
}

func (r *Postgres) ActivarMFA(ctx context.Context, id string, paso int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE usuarios SET mfa_activo=TRUE, mfa_ultimo_paso=$2, actualizado_en=now() WHERE id=$1`, id, paso)
	return err
}

func (r *Postgres) GuardarPasoMFA(ctx context.Context, id string, paso int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE usuarios SET mfa_ultimo_paso=$2 WHERE id=$1`, id, paso)
	return err
}

func (r *Postgres) ReiniciarMFA(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE usuarios SET mfa_activo=FALSE, mfa_secreto=NULL, mfa_ultimo_paso=NULL, actualizado_en=now() WHERE id=$1`, id)
	return err
}

func (r *Postgres) CambiarPassword(ctx context.Context, id, hash string, debeCambiar bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE usuarios SET password_hash=$2, debe_cambiar_password=$3, intentos_fallidos=0,
		bloqueado_hasta=NULL, actualizado_en=now() WHERE id=$1`, id, hash, debeCambiar)
	return err
}

// ------------------------------------------------------------------ sesiones

func (r *Postgres) CrearSesion(ctx context.Context, jti, usuarioID string, mfa bool, ip, ua string, expira time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO sesiones (jti, usuario_id, mfa_verificado, ip, user_agent, expira_en)
		VALUES ($1,$2,$3,$4,$5,$6)`, jti, usuarioID, mfa, ip, ua, expira)
	return err
}

func (r *Postgres) SesionPorJTI(ctx context.Context, jti string) (*models.Sesion, error) {
	s := &models.Sesion{}
	err := r.pool.QueryRow(ctx, `SELECT jti, usuario_id, mfa_verificado, creada_en, ultima_actividad, expira_en, revocada_en
		FROM sesiones WHERE jti=$1`, jti).Scan(&s.JTI, &s.UsuarioID, &s.MFAVerificado, &s.CreadaEn, &s.UltimaActividad, &s.ExpiraEn, &s.RevocadaEn)
	if err != nil {
		return nil, traducir(err)
	}
	return s, nil
}

func (r *Postgres) TocarSesion(ctx context.Context, jti string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sesiones SET ultima_actividad=now() WHERE jti=$1`, jti)
	return err
}

func (r *Postgres) RevocarSesion(ctx context.Context, jti, motivo string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sesiones SET revocada_en=now(), motivo_revocacion=$2 WHERE jti=$1 AND revocada_en IS NULL`, jti, motivo)
	return err
}

func (r *Postgres) RevocarSesionesUsuario(ctx context.Context, usuarioID, exceptoJTI, motivo string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sesiones SET revocada_en=now(), motivo_revocacion=$3
		WHERE usuario_id=$1 AND jti<>$2 AND revocada_en IS NULL`, usuarioID, exceptoJTI, motivo)
	return err
}

func (r *Postgres) SesionesAbiertas(ctx context.Context, usuarioID string) (int, error) {
	var n int
	q := `SELECT count(*) FROM sesiones WHERE revocada_en IS NULL AND expira_en > now()`
	args := []any{}
	if usuarioID != "" {
		q += ` AND usuario_id=$1`
		args = append(args, usuarioID)
	}
	err := r.pool.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

// UsuariosPorRol: resumen para el panel del administrador.
type UsuariosPorRol struct {
	Rol        string `json:"rol"`
	Activos    int    `json:"activos"`
	Inactivos  int    `json:"inactivos"`
	SinMFA     int    `json:"sinMfa"`
	Bloqueados int    `json:"bloqueados"`
}

func (r *Postgres) ResumenUsuarios(ctx context.Context) ([]UsuariosPorRol, error) {
	rows, err := r.pool.Query(ctx, `SELECT rol, count(*) FILTER (WHERE activo), count(*) FILTER (WHERE NOT activo),
		count(*) FILTER (WHERE activo AND NOT mfa_activo), count(*) FILTER (WHERE bloqueado_hasta > now()) FROM usuarios GROUP BY rol ORDER BY rol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UsuariosPorRol{}
	for rows.Next() {
		var u UsuariosPorRol
		if err := rows.Scan(&u.Rol, &u.Activos, &u.Inactivos, &u.SinMFA, &u.Bloqueados); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

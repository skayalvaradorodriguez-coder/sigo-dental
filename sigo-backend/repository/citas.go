package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"sigo-backend/domain"
	"sigo-backend/models"
	"sigo-backend/notificaciones"
)

const selectCita = `SELECT ci.id, ci.inicio, ci.fin, ci.estado, ci.motivo, ci.motivo_cancelacion,
	p.id, p.nombres || ' ' || p.apellidos, p.numero_documento, p.telefono,
	u.id, u.nombre, s.id, s.nombre, co.nombre, se.nombre, pr.id::text, pr.nombre, ci.creado_en, cr.nombre
	FROM citas ci
	JOIN pacientes p ON p.id = ci.paciente_id
	JOIN usuarios u ON u.id = ci.odontologo_id
	JOIN sillones s ON s.id = ci.sillon_id
	JOIN consultorios co ON co.id = s.consultorio_id
	JOIN sedes se ON se.id = co.sede_id
	LEFT JOIN procedimientos pr ON pr.id = ci.procedimiento_id
	JOIN usuarios cr ON cr.id = ci.creado_por`

func scanCita(row pgx.Row) (*models.Cita, error) {
	c := &models.Cita{}
	err := row.Scan(&c.ID, &c.Inicio, &c.Fin, &c.Estado, &c.Motivo, &c.MotivoCancelacion, &c.PacienteID, &c.Paciente,
		&c.PacienteDocumento, &c.PacienteTelefono, &c.OdontologoID, &c.Odontologo, &c.SillonID, &c.Sillon, &c.Consultorio,
		&c.Sede, &c.ProcedimientoID, &c.Procedimiento, &c.CreadoEn, &c.CreadoPor)
	if err != nil {
		return nil, traducir(err)
	}
	return c, nil
}

func (r *Postgres) conAlertas(ctx context.Context, cs []models.Cita) error {
	ids := make([]string, len(cs))
	for i := range cs {
		ids[i] = cs[i].PacienteID
	}
	crit, pend, err := r.ResumenAlertas(ctx, ids)
	if err != nil {
		return err
	}
	for i := range cs {
		cs[i].AlertasCriticas = crit[cs[i].PacienteID]
		cs[i].AnamnesisPendiente = pend[cs[i].PacienteID]
	}
	return nil
}

type FiltroCitas struct {
	Desde, Hasta                       time.Time
	SillonID, OdontologoID, PacienteID string
	Estados                            []string
	Limite                             int
}

func (r *Postgres) ListarCitas(ctx context.Context, f FiltroCitas) ([]models.Cita, error) {
	conds := []string{"ci.inicio < $1", "ci.fin > $2"}
	args := []any{f.Hasta, f.Desde}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.SillonID != "" {
		add("ci.sillon_id = $%d", f.SillonID)
	}
	if f.OdontologoID != "" {
		add("ci.odontologo_id = $%d", f.OdontologoID)
	}
	if f.PacienteID != "" {
		add("ci.paciente_id = $%d", f.PacienteID)
	}
	if len(f.Estados) > 0 {
		add("ci.estado = ANY($%d)", f.Estados)
	}
	q := selectCita + ` WHERE ` + strings.Join(conds, " AND ") + ` ORDER BY ci.inicio`
	if f.Limite > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limite)
	}
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out := []models.Cita{}
	for rows.Next() {
		c, err := scanCita(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *c)
	}
	rows.Close()
	return out, r.conAlertas(ctx, out)
}

func (r *Postgres) CitaPorID(ctx context.Context, id string) (*models.Cita, error) {
	c, err := scanCita(r.pool.QueryRow(ctx, selectCita+` WHERE ci.id=$1`, id))
	if err != nil {
		return nil, err
	}
	lista := []models.Cita{*c}
	if err := r.conAlertas(ctx, lista); err != nil {
		return nil, err
	}
	c = &lista[0]
	c.Eventos = []models.EventoCita{}
	rows, err := r.pool.Query(ctx, `SELECT e.id, e.estado_anterior, e.estado_nuevo, e.nota, u.nombre, e.ocurrido_en
		FROM citas_eventos e JOIN usuarios u ON u.id = e.usuario_id WHERE e.cita_id=$1 ORDER BY e.id`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e models.EventoCita
		if err := rows.Scan(&e.ID, &e.EstadoAnterior, &e.EstadoNuevo, &e.Nota, &e.Usuario, &e.OcurridoEn); err != nil {
			rows.Close()
			return nil, err
		}
		c.Eventos = append(c.Eventos, e)
	}
	rows.Close()
	c.Notificaciones, err = r.NotificacionesDeCita(ctx, id)
	return c, err
}

// Conflicto: la cita que ya ocupa el sillón (para un mensaje claro).
type Conflicto struct {
	ID       string    `json:"id"`
	Inicio   time.Time `json:"inicio"`
	Fin      time.Time `json:"fin"`
	Paciente string    `json:"paciente"`
}

// BuscarSuperposicion es la comprobación previa; la garantía real es la restricción EXCLUDE.
func (r *Postgres) BuscarSuperposicion(ctx context.Context, sillonID string, inicio, fin time.Time, excluir string) (*Conflicto, error) {
	c := &Conflicto{}
	err := r.pool.QueryRow(ctx, `SELECT ci.id, ci.inicio, ci.fin, p.nombres || ' ' || p.apellidos FROM citas ci
		JOIN pacientes p ON p.id = ci.paciente_id
		WHERE ci.sillon_id=$1 AND ci.estado NOT IN ('CANCELADO','NO_ASISTIO')
		AND tstzrange(ci.inicio, ci.fin, '[)') && tstzrange($2, $3, '[)') AND ($4 = '' OR ci.id::text <> $4) LIMIT 1`,
		sillonID, inicio, fin, excluir).Scan(&c.ID, &c.Inicio, &c.Fin, &c.Paciente)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

type NuevaCita struct {
	PacienteID, OdontologoID, SillonID string
	ProcedimientoID, Motivo            *string
	TareaID                            *string // tarea agendable de un presupuesto aprobado (RF-08)
	Inicio, Fin                        time.Time
	CreadoPor                          string
}

func insertarNotificaciones(ctx context.Context, tx pgx.Tx, citaID string, ps []notificaciones.Planificada) error {
	for _, p := range ps {
		if _, err := tx.Exec(ctx, `INSERT INTO notificaciones (cita_id, tipo, canal, destinatario, programada_para, estado, motivo)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`, citaID, p.Tipo, p.Canal, p.Destinatario, p.ProgramadaPara, p.Estado, p.Motivo); err != nil {
			return err
		}
	}
	return nil
}

// CrearCita inserta la cita, su primer evento y sus mensajes en UNA transacción (ACID).
// Si dos recepcionistas agendan a la vez el mismo sillón, PostgreSQL deja pasar sólo una.
func (r *Postgres) CrearCita(ctx context.Context, n NuevaCita, msgs []notificaciones.Planificada) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO citas (paciente_id, odontologo_id, sillon_id, procedimiento_id, inicio, fin, motivo, creado_por, tarea_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, n.PacienteID, n.OdontologoID, n.SillonID, n.ProcedimientoID,
		n.Inicio, n.Fin, n.Motivo, n.CreadoPor, n.TareaID).Scan(&id); err != nil {
		return "", traducir(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO citas_eventos (cita_id, estado_nuevo, usuario_id) VALUES ($1,'PROGRAMADO',$2)`, id, n.CreadoPor); err != nil {
		return "", err
	}
	if err := insertarNotificaciones(ctx, tx, id, msgs); err != nil {
		return "", err
	}
	return id, traducir(tx.Commit(ctx))
}

// CitaBase: datos crudos para validar transiciones.
type CitaBase struct {
	ID, PacienteID, OdontologoID, SillonID, Estado string
	Inicio, Fin                                    time.Time
	MotivoCancelacion                              *string
}

func (r *Postgres) CitaBase(ctx context.Context, id string) (*CitaBase, error) {
	c := &CitaBase{}
	err := r.pool.QueryRow(ctx, `SELECT id, paciente_id, odontologo_id, sillon_id, estado, inicio, fin, motivo_cancelacion FROM citas WHERE id=$1`, id).
		Scan(&c.ID, &c.PacienteID, &c.OdontologoID, &c.SillonID, &c.Estado, &c.Inicio, &c.Fin, &c.MotivoCancelacion)
	if err != nil {
		return nil, traducir(err)
	}
	return c, nil
}

// Reprogramar mueve la cita (bloqueo optimista: sólo si su estado no cambió) y rehace los recordatorios.
func (r *Postgres) Reprogramar(ctx context.Context, actual *CitaBase, sillonID, odontologoID string, inicio, fin time.Time,
	usuarioID string, msgs []notificaciones.Planificada) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	tag, err := tx.Exec(ctx, `UPDATE citas SET sillon_id=$3, odontologo_id=$4, inicio=$5, fin=$6, estado='PROGRAMADO', actualizado_en=now()
		WHERE id=$1 AND estado=$2`, actual.ID, actual.Estado, sillonID, odontologoID, inicio, fin)
	if err != nil {
		return traducir(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflicto
	}
	nota := "Reprogramada (antes: " + actual.Inicio.UTC().Format(time.RFC3339) + ")"
	if _, err := tx.Exec(ctx, `INSERT INTO citas_eventos (cita_id, estado_anterior, estado_nuevo, nota, usuario_id) VALUES ($1,$2,'PROGRAMADO',$3,$4)`,
		actual.ID, actual.Estado, nota, usuarioID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE notificaciones SET estado='CANCELADO', motivo='Cita reprogramada' WHERE cita_id=$1 AND estado='PENDIENTE'`, actual.ID); err != nil {
		return err
	}
	if err := insertarNotificaciones(ctx, tx, actual.ID, msgs); err != nil {
		return err
	}
	return traducir(tx.Commit(ctx))
}

func (r *Postgres) CambiarEstadoCita(ctx context.Context, actual *CitaBase, nuevo string, nota *string, usuarioID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	motivoCanc := actual.MotivoCancelacion
	if nuevo == domain.Cancelado {
		motivoCanc = nota
	}
	tag, err := tx.Exec(ctx, `UPDATE citas SET estado=$3, motivo_cancelacion=$4, actualizado_en=now() WHERE id=$1 AND estado=$2`,
		actual.ID, actual.Estado, nuevo, motivoCanc)
	if err != nil {
		return traducir(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflicto
	}
	if _, err := tx.Exec(ctx, `INSERT INTO citas_eventos (cita_id, estado_anterior, estado_nuevo, nota, usuario_id) VALUES ($1,$2,$3,$4,$5)`,
		actual.ID, actual.Estado, nuevo, nota, usuarioID); err != nil {
		return err
	}
	if !domain.OcupaSillon(nuevo) {
		if _, err := tx.Exec(ctx, `UPDATE notificaciones SET estado='CANCELADO', motivo=$2 WHERE cita_id=$1 AND estado='PENDIENTE'`,
			actual.ID, "Cita en estado "+domain.EtiquetaEstado[nuevo]); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Postgres) ConteoCitasPorEstado(ctx context.Context, desde, hasta time.Time, odontologoID string) (map[string]int, error) {
	q := `SELECT estado, count(*) FROM citas WHERE inicio >= $1 AND inicio < $2`
	args := []any{desde, hasta}
	if odontologoID != "" {
		q += ` AND odontologo_id = $3`
		args = append(args, odontologoID)
	}
	rows, err := r.pool.Query(ctx, q+` GROUP BY estado`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var e string
		var n int
		if err := rows.Scan(&e, &n); err != nil {
			return nil, err
		}
		out[e] = n
	}
	return out, rows.Err()
}

func (r *Postgres) ConteoPacientes(ctx context.Context) (total, sinAnamnesis int, err error) {
	err = r.pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE NOT EXISTS (SELECT 1 FROM anamnesis a WHERE a.paciente_id = p.id)) FROM pacientes p`).
		Scan(&total, &sinAnamnesis)
	return
}

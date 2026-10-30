package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"sigo-backend/models"
)

// ---------------------------------------------------------------- Presupuestos (RF-07, RF-08)

type ItemNuevo struct {
	Fase, Sesion                      int
	FaseNombre, ProcedimientoID       string
	Diente                            *int
	Superficies                       []string
	Honorario, Laboratorio, Descuento string // NUMERIC como texto ("45.00")
}

const colsPresupuesto = `p.id, p.paciente_id, pa.nombres || ' ' || pa.apellidos, p.odontologo_id, u.nombre, p.titulo, p.observaciones,
	p.estado, p.creado_en, p.actualizado_en, p.presentado_en, p.hash_presentado, p.decidido_en, ud.nombre, p.decision_nota`

const fromPresupuesto = ` FROM presupuestos p JOIN pacientes pa ON pa.id = p.paciente_id JOIN usuarios u ON u.id = p.odontologo_id
	LEFT JOIN usuarios ud ON ud.id = p.decision_registrada_por`

func scanPresupuesto(row pgx.Row) (*models.Presupuesto, error) {
	p := &models.Presupuesto{Items: []models.PresupuestoItem{}}
	err := row.Scan(&p.ID, &p.PacienteID, &p.Paciente, &p.OdontologoID, &p.Odontologo, &p.Titulo, &p.Observaciones, &p.Estado,
		&p.CreadoEn, &p.ActualizadoEn, &p.PresentadoEn, &p.HashPresentado, &p.DecididoEn, &p.DecisionRegistradaPor, &p.DecisionNota)
	if err != nil {
		return nil, traducir(err)
	}
	return p, nil
}

func (r *Postgres) itemsDe(ctx context.Context, ids []string) (map[string][]models.PresupuestoItem, error) {
	rows, err := r.pool.Query(ctx, `SELECT i.presupuesto_id::text, i.id, i.orden, i.fase, i.fase_nombre, i.sesion, i.procedimiento_id, pr.codigo, pr.nombre,
		i.diente, i.superficies, i.honorario::text, i.costo_laboratorio::text, i.descuento::text,
		(i.honorario + i.costo_laboratorio - i.descuento)::text, ua.nombre, i.descuento_autorizado_en, i.decision
		FROM presupuesto_items i JOIN procedimientos pr ON pr.id = i.procedimiento_id LEFT JOIN usuarios ua ON ua.id = i.descuento_autorizado_por
		WHERE i.presupuesto_id = ANY($1::uuid[]) ORDER BY i.presupuesto_id, i.orden`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]models.PresupuestoItem{}
	for rows.Next() {
		var pid string
		var it models.PresupuestoItem
		var diente *int16
		if err := rows.Scan(&pid, &it.ID, &it.Orden, &it.Fase, &it.FaseNombre, &it.Sesion, &it.ProcedimientoID, &it.Codigo, &it.Procedimiento,
			&diente, &it.Superficies, &it.Honorario, &it.CostoLaboratorio, &it.Descuento, &it.Subtotal, &it.DescuentoAutorizadoPor,
			&it.DescuentoAutorizadoEn, &it.Decision); err != nil {
			return nil, err
		}
		if diente != nil {
			d := int(*diente)
			it.Diente = &d
		}
		out[pid] = append(out[pid], it)
	}
	return out, rows.Err()
}

func (r *Postgres) listarPresupuestos(ctx context.Context, where string, args ...any) ([]models.Presupuesto, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+colsPresupuesto+fromPresupuesto+` WHERE `+where+` ORDER BY p.actualizado_en DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	out := []models.Presupuesto{}
	ids := []string{}
	for rows.Next() {
		p, err := scanPresupuesto(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *p)
		ids = append(ids, p.ID)
	}
	rows.Close()
	if len(ids) == 0 {
		return out, nil
	}
	items, err := r.itemsDe(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if v, ok := items[out[i].ID]; ok {
			out[i].Items = v
		}
	}
	return out, nil
}

func (r *Postgres) PresupuestosPaciente(ctx context.Context, pacienteID string) ([]models.Presupuesto, error) {
	return r.listarPresupuestos(ctx, `p.paciente_id = $1`, pacienteID)
}

func (r *Postgres) PresupuestosPorEstado(ctx context.Context, estado string) ([]models.Presupuesto, error) {
	if estado == "" {
		return r.listarPresupuestos(ctx, `TRUE`)
	}
	return r.listarPresupuestos(ctx, `p.estado = $1`, estado)
}

func (r *Postgres) PresupuestoPorID(ctx context.Context, id string) (*models.Presupuesto, error) {
	ps, err := r.listarPresupuestos(ctx, `p.id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return nil, ErrNoEncontrado
	}
	return &ps[0], nil
}

func insertarItems(ctx context.Context, tx pgx.Tx, presupuestoID string, items []ItemNuevo) error {
	for i, it := range items {
		if _, err := tx.Exec(ctx, `INSERT INTO presupuesto_items (presupuesto_id, orden, fase, fase_nombre, sesion, procedimiento_id, diente,
			superficies, honorario, costo_laboratorio, descuento) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::numeric,$10::numeric,$11::numeric)`,
			presupuestoID, i+1, it.Fase, it.FaseNombre, it.Sesion, it.ProcedimientoID, it.Diente, it.Superficies,
			it.Honorario, it.Laboratorio, it.Descuento); err != nil {
			return err
		}
	}
	return nil
}

func (r *Postgres) CrearPresupuesto(ctx context.Context, pacienteID, odontologoID, titulo string, obs *string, items []ItemNuevo) (string, error) {
	var id string
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO presupuestos (paciente_id, odontologo_id, titulo, observaciones) VALUES ($1,$2,$3,$4) RETURNING id`,
			pacienteID, odontologoID, titulo, obs).Scan(&id); err != nil {
			return err
		}
		return insertarItems(ctx, tx, id, items)
	})
	return id, traducir(err)
}

// ReemplazarBorrador: cambia título, observaciones e ítems de un borrador (las autorizaciones de
// descuento previas se pierden con los ítems: hay que volver a autorizar).
func (r *Postgres) ReemplazarBorrador(ctx context.Context, id, titulo string, obs *string, items []ItemNuevo) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE presupuestos SET titulo=$2, observaciones=$3, actualizado_en=now() WHERE id=$1 AND estado='BORRADOR'`, id, titulo, obs)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrConflicto
		}
		if _, err := tx.Exec(ctx, `DELETE FROM presupuesto_items WHERE presupuesto_id=$1`, id); err != nil {
			return err
		}
		return insertarItems(ctx, tx, id, items)
	})
	return traducir(err)
}

func (r *Postgres) AutorizarDescuentos(ctx context.Context, id string, itemIDs []string, usuarioID string) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE presupuesto_items SET descuento_autorizado_por=$3, descuento_autorizado_en=now()
			WHERE presupuesto_id=$1 AND id = ANY($2::uuid[])`, id, itemIDs, usuarioID)
		if err != nil {
			return err
		}
		if int(tag.RowsAffected()) != len(itemIDs) {
			return ErrConflicto
		}
		_, err = tx.Exec(ctx, `UPDATE presupuestos SET actualizado_en=now() WHERE id=$1`, id)
		return err
	})
	return traducir(err)
}

func (r *Postgres) PresentarPresupuesto(ctx context.Context, id, hash string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE presupuestos SET estado='PRESENTADO', presentado_en=now(), hash_presentado=$2, actualizado_en=now()
		WHERE id=$1 AND estado='BORRADOR'`, id, hash)
	if err != nil {
		return traducir(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflicto
	}
	return nil
}

// DecidirPresupuesto registra la decisión del paciente sobre cada ítem y crea las tareas
// agendables de los ítems aprobados, todo en una transacción (RF-08).
func (r *Postgres) DecidirPresupuesto(ctx context.Context, id string, decisiones map[string]string, estado, usuarioID string, nota *string) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var actual string
		if err := tx.QueryRow(ctx, `SELECT estado FROM presupuestos WHERE id=$1 FOR UPDATE`, id).Scan(&actual); err != nil {
			return err
		}
		if actual != "PRESENTADO" {
			return ErrConflicto
		}
		for itemID, d := range decisiones {
			tag, err := tx.Exec(ctx, `UPDATE presupuesto_items SET decision=$3 WHERE presupuesto_id=$1 AND id=$2`, id, itemID, d)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return ErrConflicto
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO tareas_tratamiento (presupuesto_item_id, paciente_id, procedimiento_id, diente, superficies, fase, sesion)
			SELECT i.id, p.paciente_id, i.procedimiento_id, i.diente, i.superficies, i.fase, i.sesion
			FROM presupuesto_items i JOIN presupuestos p ON p.id = i.presupuesto_id
			WHERE i.presupuesto_id=$1 AND i.decision='APROBADO'`, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE presupuestos SET estado=$2, decidido_en=now(), decision_registrada_por=$3, decision_nota=$4, actualizado_en=now()
			WHERE id=$1`, id, estado, usuarioID, nota)
		return err
	})
	return traducir(err)
}

// ---------------------------------------------------------------- Tareas agendables

const colsTarea = `t.id, i.presupuesto_id, p.titulo, t.procedimiento_id, pr.codigo, pr.nombre, pr.duracion_min, t.diente, t.superficies, t.fase, t.sesion,
	c.id, c.inicio, r.realizado_en`

const fromTarea = ` FROM tareas_tratamiento t
	JOIN presupuesto_items i ON i.id = t.presupuesto_item_id JOIN presupuestos p ON p.id = i.presupuesto_id
	JOIN procedimientos pr ON pr.id = t.procedimiento_id
	LEFT JOIN citas c ON c.tarea_id = t.id AND c.estado NOT IN ('CANCELADO','NO_ASISTIO')
	LEFT JOIN procedimientos_realizados r ON r.tarea_id = t.id`

func scanTarea(row pgx.Row) (*models.Tarea, error) {
	t := &models.Tarea{}
	var diente *int16
	if err := row.Scan(&t.ID, &t.PresupuestoID, &t.Presupuesto, &t.ProcedimientoID, &t.Codigo, &t.Procedimiento, &t.DuracionMin, &diente,
		&t.Superficies, &t.Fase, &t.Sesion, &t.CitaID, &t.CitaInicio, &t.RealizadoEn); err != nil {
		return nil, traducir(err)
	}
	if diente != nil {
		d := int(*diente)
		t.Diente = &d
	}
	switch {
	case t.RealizadoEn != nil:
		t.Estado = "REALIZADA"
	case t.CitaID != nil:
		t.Estado = "AGENDADA"
	default:
		t.Estado = "PENDIENTE"
	}
	return t, nil
}

func (r *Postgres) TareasPaciente(ctx context.Context, pacienteID string) ([]models.Tarea, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+colsTarea+fromTarea+` WHERE t.paciente_id=$1 ORDER BY p.creado_en, t.fase, t.sesion, i.orden`, pacienteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Tarea{}
	for rows.Next() {
		t, err := scanTarea(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (r *Postgres) TareaPorID(ctx context.Context, id string) (*models.Tarea, string, error) {
	var pid string
	t, err := scanTarea(r.pool.QueryRow(ctx, `SELECT `+colsTarea+fromTarea+` WHERE t.id=$1`, id))
	if err != nil {
		return nil, "", err
	}
	if err := r.pool.QueryRow(ctx, `SELECT paciente_id FROM tareas_tratamiento WHERE id=$1`, id).Scan(&pid); err != nil {
		return nil, "", traducir(err)
	}
	return t, pid, nil
}

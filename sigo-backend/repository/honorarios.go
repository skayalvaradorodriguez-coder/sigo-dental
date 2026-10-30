package repository

import (
	"context"
	"time"

	"sigo-backend/models"
)

// ---------------------------------------------------------------- Honorarios (RF-13)

func (r *Postgres) ReglasHonorario(ctx context.Context) ([]models.ReglaHonorario, error) {
	rows, err := r.pool.Query(ctx, `SELECT g.id, g.odontologo_id, u.nombre, g.procedimiento_id, p.codigo || ' · ' || p.nombre, g.tipo, g.valor::text,
		to_char(g.vigente_desde, 'YYYY-MM-DD'), c.nombre, g.creado_en
		FROM reglas_honorario g LEFT JOIN usuarios u ON u.id = g.odontologo_id LEFT JOIN procedimientos p ON p.id = g.procedimiento_id
		JOIN usuarios c ON c.id = g.creado_por ORDER BY g.creado_en`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ReglaHonorario{}
	for rows.Next() {
		var x models.ReglaHonorario
		if err := rows.Scan(&x.ID, &x.OdontologoID, &x.Odontologo, &x.ProcedimientoID, &x.Procedimiento, &x.Tipo, &x.Valor, &x.VigenteDesde, &x.CreadoPor, &x.CreadoEn); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *Postgres) CrearRegla(ctx context.Context, odontologoID, procedimientoID *string, tipo, valor, vigente, usuarioID string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `INSERT INTO reglas_honorario (odontologo_id, procedimiento_id, tipo, valor, vigente_desde, creado_por)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, odontologoID, procedimientoID, tipo, valor, vigente, usuarioID).Scan(&id)
	return id, traducir(err)
}

// LineasPorLiquidar: conceptos cobrados (no anulados, no liquidados) de procedimientos realizados por
// el odontólogo, con cobro en [desde, hasta). Laboratorio: el del ítem del presupuesto; materiales: el
// costo de los insumos descargados para ese procedimiento.
func (r *Postgres) LineasPorLiquidar(ctx context.Context, odontologoID string, desde, hasta time.Time) ([]models.LineaHonorario, error) {
	rows, err := r.pool.Query(ctx, `SELECT cc.id, pr.id, pr.procedimiento_id, cc.descripcion, c.numero, c.registrado_en, cc.monto::text,
		COALESCE(i.costo_laboratorio, 0)::text,
		COALESCE(round(-(SELECT sum(m.cantidad * m.costo_unitario) FROM movimientos_stock m
		                 WHERE m.procedimiento_realizado_id = pr.id AND m.tipo = 'SALIDA_PROCEDIMIENTO'), 2), 0)::text
		FROM cobro_conceptos cc JOIN cobros c ON c.id = cc.cobro_id JOIN procedimientos_realizados pr ON pr.id = cc.procedimiento_realizado_id
		LEFT JOIN tareas_tratamiento t ON t.id = pr.tarea_id LEFT JOIN presupuesto_items i ON i.id = t.presupuesto_item_id
		WHERE pr.odontologo_id = $1 AND c.registrado_en >= $2 AND c.registrado_en < $3
		  AND NOT EXISTS (SELECT 1 FROM cobros_anulaciones a WHERE a.cobro_id = c.id)
		  AND NOT EXISTS (SELECT 1 FROM liquidacion_lineas l WHERE l.cobro_concepto_id = cc.id)
		ORDER BY c.registrado_en`, odontologoID, desde, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.LineaHonorario{}
	for rows.Next() {
		var x models.LineaHonorario
		if err := rows.Scan(&x.CobroConceptoID, &x.ProcedimientoRealizadoID, &x.ProcedimientoID, &x.Descripcion, &x.Recibo, &x.CobradoEn,
			&x.Monto, &x.Laboratorio, &x.Materiales); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// CrearLiquidacion guarda la liquidación y sus líneas; un concepto ya liquidado ⇒ ErrDuplicado.
func (r *Postgres) CrearLiquidacion(ctx context.Context, odontologoID, desde, hasta, total, usuarioID string, lineas []models.LineaHonorario) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO liquidaciones_honorarios (odontologo_id, desde, hasta, total, creado_por) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		odontologoID, desde, hasta, total, usuarioID).Scan(&id); err != nil {
		return "", traducir(err)
	}
	for _, l := range lineas {
		if _, err := tx.Exec(ctx, `INSERT INTO liquidacion_lineas (liquidacion_id, cobro_concepto_id, procedimiento_realizado_id, descripcion, cobrado_en,
			monto, laboratorio, materiales, regla_id, regla, honorario) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			id, l.CobroConceptoID, l.ProcedimientoRealizadoID, l.Descripcion, l.CobradoEn, l.Monto, l.Laboratorio, l.Materiales, l.ReglaID, l.Regla, l.Honorario); err != nil {
			return "", traducir(err)
		}
	}
	return id, traducir(tx.Commit(ctx))
}

const colsLiquidacion = `l.id, l.odontologo_id, u.nombre, to_char(l.desde,'YYYY-MM-DD'), to_char(l.hasta,'YYYY-MM-DD'), l.total::text, c.nombre, l.creado_en,
	(SELECT count(*) FROM liquidacion_lineas x WHERE x.liquidacion_id = l.id)`

func (r *Postgres) Liquidaciones(ctx context.Context, odontologoID string) ([]models.Liquidacion, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+colsLiquidacion+` FROM liquidaciones_honorarios l JOIN usuarios u ON u.id = l.odontologo_id
		JOIN usuarios c ON c.id = l.creado_por WHERE ($1 = '' OR l.odontologo_id::text = $1) ORDER BY l.creado_en DESC LIMIT 200`, odontologoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Liquidacion{}
	for rows.Next() {
		var x models.Liquidacion
		if err := rows.Scan(&x.ID, &x.OdontologoID, &x.Odontologo, &x.Desde, &x.Hasta, &x.Total, &x.CreadoPor, &x.CreadoEn, &x.CantLineas); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *Postgres) LiquidacionPorID(ctx context.Context, id string) (*models.Liquidacion, error) {
	x := &models.Liquidacion{}
	if err := r.pool.QueryRow(ctx, `SELECT `+colsLiquidacion+` FROM liquidaciones_honorarios l JOIN usuarios u ON u.id = l.odontologo_id
		JOIN usuarios c ON c.id = l.creado_por WHERE l.id = $1`, id).Scan(&x.ID, &x.OdontologoID, &x.Odontologo, &x.Desde, &x.Hasta, &x.Total,
		&x.CreadoPor, &x.CreadoEn, &x.CantLineas); err != nil {
		return nil, traducir(err)
	}
	rows, err := r.pool.Query(ctx, `SELECT x.cobro_concepto_id, x.procedimiento_realizado_id, pr.procedimiento_id, x.descripcion, c.numero, x.cobrado_en,
		x.monto::text, x.laboratorio::text, x.materiales::text, x.regla_id, x.regla, x.honorario::text
		FROM liquidacion_lineas x JOIN procedimientos_realizados pr ON pr.id = x.procedimiento_realizado_id
		JOIN cobro_conceptos cc ON cc.id = x.cobro_concepto_id JOIN cobros c ON c.id = cc.cobro_id
		WHERE x.liquidacion_id = $1 ORDER BY x.cobrado_en`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	x.Lineas = []models.LineaHonorario{}
	for rows.Next() {
		var l models.LineaHonorario
		if err := rows.Scan(&l.CobroConceptoID, &l.ProcedimientoRealizadoID, &l.ProcedimientoID, &l.Descripcion, &l.Recibo, &l.CobradoEn,
			&l.Monto, &l.Laboratorio, &l.Materiales, &l.ReglaID, &l.Regla, &l.Honorario); err != nil {
			return nil, err
		}
		x.Lineas = append(x.Lineas, l)
	}
	return x, rows.Err()
}

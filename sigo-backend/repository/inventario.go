package repository

import (
	"context"

	"github.com/google/uuid"

	"sigo-backend/models"
)

// ---------------------------------------------------------------- Inventario (RF-14)

func (r *Postgres) Ubicaciones(ctx context.Context) ([]models.Ubicacion, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, nombre, consultorio_id, por_defecto, activo FROM inventario_ubicaciones ORDER BY por_defecto DESC, nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Ubicacion{}
	for rows.Next() {
		var u models.Ubicacion
		if err := rows.Scan(&u.ID, &u.Nombre, &u.ConsultorioID, &u.PorDefecto, &u.Activo); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *Postgres) Insumos(ctx context.Context, todos bool) ([]models.Insumo, error) {
	rows, err := r.pool.Query(ctx, `SELECT i.id, i.codigo, i.nombre, i.unidad, i.costo_unitario::text, i.stock_minimo::text,
		COALESCE((SELECT sum(m.cantidad) FROM movimientos_stock m WHERE m.insumo_id = i.id), 0)::numeric(12,3)::text, i.activo
		FROM insumos i WHERE $1 OR i.activo ORDER BY i.nombre`, todos)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Insumo{}
	for rows.Next() {
		var x models.Insumo
		if err := rows.Scan(&x.ID, &x.Codigo, &x.Nombre, &x.Unidad, &x.CostoUnitario, &x.StockMinimo, &x.StockTotal, &x.Activo); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *Postgres) InsumoActivo(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM insumos WHERE id=$1 AND activo)`, id).Scan(&ok)
	return ok, err
}

func (r *Postgres) UbicacionActiva(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM inventario_ubicaciones WHERE id=$1 AND activo)`, id).Scan(&ok)
	return ok, err
}

type InsumoDatos struct {
	Codigo, Nombre, Unidad, Costo, Minimo string
	Activo                                bool
}

func (r *Postgres) CrearInsumo(ctx context.Context, d InsumoDatos) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `INSERT INTO insumos (codigo, nombre, unidad, costo_unitario, stock_minimo) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		d.Codigo, d.Nombre, d.Unidad, d.Costo, d.Minimo).Scan(&id)
	return id, traducir(err)
}

func (r *Postgres) ActualizarInsumo(ctx context.Context, id string, d InsumoDatos) error {
	tag, err := r.pool.Exec(ctx, `UPDATE insumos SET nombre=$2, unidad=$3, costo_unitario=$4, stock_minimo=$5, activo=$6, actualizado_en=now() WHERE id=$1`,
		id, d.Nombre, d.Unidad, d.Costo, d.Minimo, d.Activo)
	if err != nil {
		return traducir(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return nil
}

// Stock por insumo y ubicación (ubicacionID vacío = todas).
func (r *Postgres) Stock(ctx context.Context, ubicacionID string) ([]models.StockItem, error) {
	rows, err := r.pool.Query(ctx, `WITH s AS (
			SELECT insumo_id, ubicacion_id, sum(cantidad) AS cantidad FROM movimientos_stock GROUP BY insumo_id, ubicacion_id),
		t AS (SELECT insumo_id, sum(cantidad) AS total FROM movimientos_stock GROUP BY insumo_id)
		SELECT i.id, i.codigo, i.nombre, i.unidad, u.id, u.nombre, s.cantidad::numeric(12,3)::text, i.stock_minimo::text,
		       COALESCE(t.total, 0) < i.stock_minimo, s.cantidad < 0
		FROM s JOIN insumos i ON i.id = s.insumo_id JOIN inventario_ubicaciones u ON u.id = s.ubicacion_id LEFT JOIN t ON t.insumo_id = i.id
		WHERE ($1 = '' OR u.id::text = $1) AND i.activo
		ORDER BY i.nombre, u.nombre`, ubicacionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.StockItem{}
	for rows.Next() {
		var x models.StockItem
		if err := rows.Scan(&x.InsumoID, &x.Codigo, &x.Insumo, &x.Unidad, &x.UbicacionID, &x.Ubicacion, &x.Cantidad, &x.StockMinimo, &x.Bajo, &x.Negativo); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *Postgres) Movimientos(ctx context.Context, insumoID, realizadoID string) ([]models.Movimiento, error) {
	rows, err := r.pool.Query(ctx, `SELECT m.id, i.id, i.nombre, i.unidad, u.nombre, m.tipo, m.cantidad::text, m.costo_unitario::text,
		m.procedimiento_realizado_id, p.nombre, m.motivo, us.nombre, m.registrado_en
		FROM movimientos_stock m JOIN insumos i ON i.id = m.insumo_id JOIN inventario_ubicaciones u ON u.id = m.ubicacion_id
		JOIN usuarios us ON us.id = m.registrado_por
		LEFT JOIN procedimientos_realizados pr ON pr.id = m.procedimiento_realizado_id LEFT JOIN procedimientos p ON p.id = pr.procedimiento_id
		WHERE ($1 = '' OR m.insumo_id::text = $1) AND ($2 = '' OR m.procedimiento_realizado_id::text = $2)
		ORDER BY m.registrado_en DESC LIMIT 300`, insumoID, realizadoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Movimiento{}
	for rows.Next() {
		var x models.Movimiento
		if err := rows.Scan(&x.ID, &x.InsumoID, &x.Insumo, &x.Unidad, &x.Ubicacion, &x.Tipo, &x.Cantidad, &x.CostoUnitario,
			&x.ProcedimientoRealizadoID, &x.Procedimiento, &x.Motivo, &x.RegistradoPor, &x.RegistradoEn); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

type NuevoMovimiento struct {
	Tipo, InsumoID, UbicacionID, Cantidad, UsuarioID string
	CostoUnitario                                    *string // sólo ENTRADA: actualiza el costo del insumo (último costo)
	Motivo                                           *string
}

func (r *Postgres) RegistrarMovimiento(ctx context.Context, n NuevoMovimiento) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if n.CostoUnitario != nil {
		if _, err := tx.Exec(ctx, `UPDATE insumos SET costo_unitario=$2, actualizado_en=now() WHERE id=$1`, n.InsumoID, *n.CostoUnitario); err != nil {
			return "", traducir(err)
		}
	}
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO movimientos_stock (insumo_id, ubicacion_id, tipo, cantidad, costo_unitario, motivo, registrado_por)
		SELECT $1, $2, $3, $4, i.costo_unitario, $5, $6 FROM insumos i WHERE i.id = $1 RETURNING id`,
		n.InsumoID, n.UbicacionID, n.Tipo, n.Cantidad, n.Motivo, n.UsuarioID).Scan(&id); err != nil {
		return "", traducir(err)
	}
	return id, traducir(tx.Commit(ctx))
}

// Transferir: dos movimientos que suman cero (la BD lo verifica al confirmar).
func (r *Postgres) Transferir(ctx context.Context, insumoID, origen, destino, cantidad, usuarioID string, motivo *string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tid := uuid.NewString()
	for _, p := range []struct {
		ubic, signo string
	}{{origen, "-"}, {destino, ""}} {
		if _, err := tx.Exec(ctx, `INSERT INTO movimientos_stock (insumo_id, ubicacion_id, tipo, cantidad, costo_unitario, transferencia_id, motivo, registrado_por)
			SELECT $1, $2, 'TRANSFERENCIA', ($3)::numeric * CASE WHEN $4 = '-' THEN -1 ELSE 1 END, i.costo_unitario, $5, $6, $7 FROM insumos i WHERE i.id = $1`,
			insumoID, p.ubic, cantidad, p.signo, tid, motivo, usuarioID); err != nil {
			return "", traducir(err)
		}
	}
	return tid, traducir(tx.Commit(ctx))
}

func (r *Postgres) RecetasConsumo(ctx context.Context) ([]models.RecetaConsumo, error) {
	rows, err := r.pool.Query(ctx, `SELECT p.id, p.codigo, p.nombre, i.id, i.codigo, i.nombre, i.unidad, rc.cantidad::text,
		round(rc.cantidad * i.costo_unitario, 2)::text
		FROM procedimientos p LEFT JOIN recetas_consumo rc ON rc.procedimiento_id = p.id LEFT JOIN insumos i ON i.id = rc.insumo_id
		WHERE p.activo ORDER BY p.codigo, i.nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.RecetaConsumo{}
	idx := map[string]int{}
	for rows.Next() {
		var pid, cod, nom string
		var iid, icod, inom, uni, cant, costo *string
		if err := rows.Scan(&pid, &cod, &nom, &iid, &icod, &inom, &uni, &cant, &costo); err != nil {
			return nil, err
		}
		k, ok := idx[pid]
		if !ok {
			out = append(out, models.RecetaConsumo{ProcedimientoID: pid, Codigo: cod, Procedimiento: nom, Lineas: []models.LineaReceta{}})
			k = len(out) - 1
			idx[pid] = k
		}
		if iid != nil {
			out[k].Lineas = append(out[k].Lineas, models.LineaReceta{InsumoID: *iid, Codigo: *icod, Insumo: *inom, Unidad: *uni, Cantidad: *cant, Costo: *costo})
		}
	}
	return out, rows.Err()
}

type LineaRecetaNueva struct{ InsumoID, Cantidad string }

// ReemplazarReceta cambia la receta de consumo de un procedimiento (la anterior queda en la auditoría).
func (r *Postgres) ReemplazarReceta(ctx context.Context, procedimientoID string, lineas []LineaRecetaNueva) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM recetas_consumo WHERE procedimiento_id=$1`, procedimientoID); err != nil {
		return err
	}
	for _, l := range lineas {
		if _, err := tx.Exec(ctx, `INSERT INTO recetas_consumo (procedimiento_id, insumo_id, cantidad) VALUES ($1,$2,$3)`, procedimientoID, l.InsumoID, l.Cantidad); err != nil {
			return traducir(err)
		}
	}
	return tx.Commit(ctx)
}

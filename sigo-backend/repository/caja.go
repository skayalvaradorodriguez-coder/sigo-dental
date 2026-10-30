package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"sigo-backend/domain"
	"sigo-backend/models"
)

// ---------------------------------------------------------------- Caja (RF-12, RN-02)

// CajaAbierta devuelve la caja abierta del recepcionista (sin montos) o ErrNoEncontrado.
func (r *Postgres) CajaAbierta(ctx context.Context, cajeroID string) (*models.CajaSesion, error) {
	s := &models.CajaSesion{}
	err := r.pool.QueryRow(ctx, `SELECT s.id, s.abierta_en, s.fondo_inicial::text, s.estado, (SELECT count(*) FROM cobros c WHERE c.sesion_id = s.id)
		FROM caja_sesiones s WHERE s.cajero_id = $1 AND s.estado = 'ABIERTA'`, cajeroID).Scan(&s.ID, &s.AbiertaEn, &s.FondoInicial, &s.Estado, &s.Cobros)
	if err != nil {
		return nil, traducir(err)
	}
	return s, nil
}

func (r *Postgres) AbrirCaja(ctx context.Context, cajeroID, fondo string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `INSERT INTO caja_sesiones (cajero_id, fondo_inicial) VALUES ($1, $2) RETURNING id`, cajeroID, fondo).Scan(&id)
	return id, traducir(err)
}

// CerrarCaja guarda lo declarado y, en la misma transacción y con la sesión bloqueada, el teórico.
func (r *Postgres) CerrarCaja(ctx context.Context, sesionID string, declarado map[string]string, nota *string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var estado string
	if err := tx.QueryRow(ctx, `SELECT estado FROM caja_sesiones WHERE id=$1 FOR UPDATE`, sesionID).Scan(&estado); err != nil {
		return traducir(err)
	}
	if estado != "ABIERTA" {
		return ErrConflicto
	}
	_, err = tx.Exec(ctx, `UPDATE caja_sesiones s SET estado='CERRADA', cerrada_en=now(),
		declarado_efectivo=$2, declarado_tarjeta=$3, declarado_transferencia=$4, nota_cierre=$5,
		teorico_efectivo = s.fondo_inicial + t.efectivo, teorico_tarjeta = t.tarjeta, teorico_transferencia = t.transferencia
		FROM (SELECT COALESCE(sum(p.monto) FILTER (WHERE p.medio='EFECTIVO'), 0) AS efectivo,
		             COALESCE(sum(p.monto) FILTER (WHERE p.medio='TARJETA'), 0) AS tarjeta,
		             COALESCE(sum(p.monto) FILTER (WHERE p.medio='TRANSFERENCIA'), 0) AS transferencia
		      FROM cobros c JOIN cobro_pagos p ON p.cobro_id = c.id
		      WHERE c.sesion_id = $1 AND NOT EXISTS (SELECT 1 FROM cobros_anulaciones a WHERE a.cobro_id = c.id)) t
		WHERE s.id = $1`, sesionID, declarado[domain.MedioEfectivo], declarado[domain.MedioTarjeta], declarado[domain.MedioTransferencia], nota)
	if err != nil {
		return traducir(err)
	}
	return tx.Commit(ctx)
}

const colsCierre = `s.id, s.cajero_id, u.nombre, s.abierta_en, s.cerrada_en, s.estado, s.fondo_inicial::text, s.nota_cierre,
	s.teorico_efectivo::text, s.teorico_tarjeta::text, s.teorico_transferencia::text,
	s.declarado_efectivo::text, s.declarado_tarjeta::text, s.declarado_transferencia::text,
	(SELECT count(*) FROM cobros c WHERE c.sesion_id = s.id),
	(SELECT count(*) FROM cobros c JOIN cobros_anulaciones a ON a.cobro_id = c.id WHERE c.sesion_id = s.id),
	(SELECT COALESCE(sum(p.monto), 0)::text FROM cobros c JOIN cobro_pagos p ON p.cobro_id = c.id
	   WHERE c.sesion_id = s.id AND p.medio = 'SEGURO' AND NOT EXISTS (SELECT 1 FROM cobros_anulaciones a WHERE a.cobro_id = c.id)),
	ur.nombre, s.revisada_en, s.nota_revision`

func scanCierre(row pgx.Row) (*models.CierreCaja, error) {
	c := &models.CierreCaja{Teorico: map[string]string{}, Declarado: map[string]string{}}
	var te, tt, ttr, de, dt, dtr *string
	if err := row.Scan(&c.ID, &c.CajeroID, &c.Cajero, &c.AbiertaEn, &c.CerradaEn, &c.Estado, &c.FondoInicial, &c.NotaCierre,
		&te, &tt, &ttr, &de, &dt, &dtr, &c.Cobros, &c.Anulados, &c.Seguro, &c.RevisadaPor, &c.RevisadaEn, &c.NotaRevision); err != nil {
		return nil, traducir(err)
	}
	if te != nil {
		c.Teorico = map[string]string{domain.MedioEfectivo: *te, domain.MedioTarjeta: *tt, domain.MedioTransferencia: *ttr}
		c.Declarado = map[string]string{domain.MedioEfectivo: *de, domain.MedioTarjeta: *dt, domain.MedioTransferencia: *dtr}
	}
	return c, nil
}

// Cierres: sesiones para el Auditor (estado vacío = todas, las más recientes primero).
func (r *Postgres) Cierres(ctx context.Context, estado string) ([]models.CierreCaja, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+colsCierre+` FROM caja_sesiones s JOIN usuarios u ON u.id = s.cajero_id
		LEFT JOIN usuarios ur ON ur.id = s.revisada_por WHERE ($1 = '' OR s.estado = $1)
		ORDER BY COALESCE(s.cerrada_en, s.abierta_en) DESC LIMIT 200`, estado)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CierreCaja{}
	for rows.Next() {
		c, err := scanCierre(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (r *Postgres) CierrePorID(ctx context.Context, id string) (*models.CierreCaja, error) {
	return scanCierre(r.pool.QueryRow(ctx, `SELECT `+colsCierre+` FROM caja_sesiones s JOIN usuarios u ON u.id = s.cajero_id
		LEFT JOIN usuarios ur ON ur.id = s.revisada_por WHERE s.id = $1`, id))
}

func (r *Postgres) RevisarCierre(ctx context.Context, id, usuarioID string, nota *string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE caja_sesiones SET estado='REVISADA', revisada_por=$2, revisada_en=now(), nota_revision=$3
		WHERE id=$1 AND estado='CERRADA'`, id, usuarioID, nota)
	if err != nil {
		return traducir(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflicto
	}
	return nil
}

// ---------------------------------------------------------------- Cobros

// PorCobrar: procedimientos realizados del paciente sin un cobro vigente. Monto: el subtotal del
// ítem del presupuesto aprobado si se hizo desde una tarea; si no, el arancel del catálogo.
func (r *Postgres) PorCobrar(ctx context.Context, pacienteID string) ([]models.PorCobrar, error) {
	rows, err := r.pool.Query(ctx, `SELECT pr.id, pr.procedimiento_id, p.codigo, p.nombre, pr.diente, pr.realizado_en, u.nombre,
		COALESCE((i.honorario + i.costo_laboratorio - i.descuento), p.arancel)::text, CASE WHEN i.id IS NULL THEN 'ARANCEL' ELSE 'PRESUPUESTO' END
		FROM procedimientos_realizados pr JOIN procedimientos p ON p.id = pr.procedimiento_id JOIN usuarios u ON u.id = pr.odontologo_id
		LEFT JOIN tareas_tratamiento t ON t.id = pr.tarea_id LEFT JOIN presupuesto_items i ON i.id = t.presupuesto_item_id
		WHERE pr.paciente_id = $1
		  AND COALESCE((i.honorario + i.costo_laboratorio - i.descuento), p.arancel) > 0
		  AND NOT EXISTS (SELECT 1 FROM cobro_conceptos cc WHERE cc.procedimiento_realizado_id = pr.id
		                  AND NOT EXISTS (SELECT 1 FROM cobros_anulaciones a WHERE a.cobro_id = cc.cobro_id))
		ORDER BY pr.realizado_en`, pacienteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.PorCobrar{}
	for rows.Next() {
		var x models.PorCobrar
		var d *int16
		if err := rows.Scan(&x.ProcedimientoRealizadoID, &x.ProcedimientoID, &x.Codigo, &x.Procedimiento, &d, &x.RealizadoEn, &x.Odontologo, &x.Monto, &x.Origen); err != nil {
			return nil, err
		}
		if d != nil {
			v := int(*d)
			x.Diente = &v
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

type NuevoCobro struct {
	SesionID, PacienteID, UsuarioID string
	Total                           string
	Conceptos                       []models.CobroConcepto
	Pagos                           []models.CobroPago
}

// CrearCobro: todo o nada. La BD verifica al confirmar que conceptos = pagos = total, que la caja
// esté abierta y que ningún procedimiento realizado se cobre dos veces.
func (r *Postgres) CrearCobro(ctx context.Context, n NuevoCobro) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO cobros (sesion_id, paciente_id, total, registrado_por) VALUES ($1,$2,$3,$4) RETURNING id`,
		n.SesionID, n.PacienteID, n.Total, n.UsuarioID).Scan(&id); err != nil {
		return "", traducirCaja(err)
	}
	for _, c := range n.Conceptos {
		if _, err := tx.Exec(ctx, `INSERT INTO cobro_conceptos (cobro_id, procedimiento_realizado_id, procedimiento_id, descripcion, monto) VALUES ($1,$2,$3,$4,$5)`,
			id, c.ProcedimientoRealizadoID, c.ProcedimientoID, c.Descripcion, c.Monto); err != nil {
			return "", traducirCaja(err)
		}
	}
	for _, p := range n.Pagos {
		if _, err := tx.Exec(ctx, `INSERT INTO cobro_pagos (cobro_id, medio, monto, referencia, convenio_id) VALUES ($1,$2,$3,$4,$5)`,
			id, p.Medio, p.Monto, p.Referencia, p.ConvenioID); err != nil {
			return "", traducirCaja(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", traducirCaja(err)
	}
	return id, nil
}

// ErrCajaCerrada: la caja se cerró (o no está abierta) cuando se intentó cobrar o anular.
var ErrCajaCerrada = errors.New("la caja no está abierta")

func traducirCaja(err error) error {
	if err != nil && codigoPG(err) == "P0001" && strings.Contains(err.Error(), "la caja no está abierta") {
		return ErrCajaCerrada
	}
	return traducir(err)
}

func (r *Postgres) AnularCobro(ctx context.Context, cobroID, motivo, usuarioID string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO cobros_anulaciones (cobro_id, motivo, anulado_por) VALUES ($1,$2,$3)`, cobroID, motivo, usuarioID)
	return traducirCaja(err)
}

const colsCobro = `c.id, c.numero, c.sesion_id, c.paciente_id, pa.nombres || ' ' || pa.apellidos, c.total::text, u.nombre, c.registrado_en,
	a.motivo, ua.nombre, a.anulado_en`

const fromCobro = ` FROM cobros c JOIN pacientes pa ON pa.id = c.paciente_id JOIN usuarios u ON u.id = c.registrado_por
	LEFT JOIN cobros_anulaciones a ON a.cobro_id = c.id LEFT JOIN usuarios ua ON ua.id = a.anulado_por`

func (r *Postgres) listarCobros(ctx context.Context, where string, args ...any) ([]models.Cobro, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+colsCobro+fromCobro+` WHERE `+where+` ORDER BY c.registrado_en DESC LIMIT 500`, args...)
	if err != nil {
		return nil, err
	}
	out := []models.Cobro{}
	ids := []string{}
	for rows.Next() {
		var c models.Cobro
		var motivo, por *string
		var en *time.Time
		if err := rows.Scan(&c.ID, &c.Numero, &c.SesionID, &c.PacienteID, &c.Paciente, &c.Total, &c.RegistradoPor, &c.RegistradoEn, &motivo, &por, &en); err != nil {
			rows.Close()
			return nil, err
		}
		if motivo != nil {
			c.Anulacion = &models.Anulacion{Motivo: *motivo, AnuladoPor: *por, AnuladoEn: *en}
		}
		c.Conceptos, c.Pagos, c.Comprobantes = []models.CobroConcepto{}, []models.CobroPago{}, []models.Comprobante{}
		out = append(out, c)
		ids = append(ids, c.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(ids) == 0 {
		return out, err
	}
	idx := map[string]*models.Cobro{}
	for i := range out {
		idx[out[i].ID] = &out[i]
	}
	rc, err := r.pool.Query(ctx, `SELECT cobro_id, procedimiento_realizado_id, procedimiento_id, descripcion, monto::text FROM cobro_conceptos
		WHERE cobro_id = ANY($1::uuid[]) ORDER BY cobro_id, descripcion`, ids)
	if err != nil {
		return nil, err
	}
	for rc.Next() {
		var cid string
		var x models.CobroConcepto
		if err := rc.Scan(&cid, &x.ProcedimientoRealizadoID, &x.ProcedimientoID, &x.Descripcion, &x.Monto); err != nil {
			rc.Close()
			return nil, err
		}
		idx[cid].Conceptos = append(idx[cid].Conceptos, x)
	}
	rc.Close()
	rp, err := r.pool.Query(ctx, `SELECT p.cobro_id, p.medio, p.monto::text, p.referencia, p.convenio_id, cv.nombre FROM cobro_pagos p
		LEFT JOIN convenios cv ON cv.id = p.convenio_id WHERE p.cobro_id = ANY($1::uuid[]) ORDER BY p.cobro_id, p.medio`, ids)
	if err != nil {
		return nil, err
	}
	for rp.Next() {
		var cid string
		var x models.CobroPago
		if err := rp.Scan(&cid, &x.Medio, &x.Monto, &x.Referencia, &x.ConvenioID, &x.Convenio); err != nil {
			rp.Close()
			return nil, err
		}
		idx[cid].Pagos = append(idx[cid].Pagos, x)
	}
	rp.Close()
	rk, err := r.pool.Query(ctx, `SELECT cobro_id, id, tipo, estado, numero, autorizacion, proveedor, simulado, mensaje, emitido_en, hash_documento
		FROM comprobantes WHERE cobro_id = ANY($1::uuid[]) ORDER BY cobro_id, emitido_en`, ids)
	if err != nil {
		return nil, err
	}
	defer rk.Close()
	for rk.Next() {
		var cid string
		var x models.Comprobante
		if err := rk.Scan(&cid, &x.ID, &x.Tipo, &x.Estado, &x.Numero, &x.Autorizacion, &x.Proveedor, &x.Simulado, &x.Mensaje, &x.EmitidoEn, &x.HashDocumento); err != nil {
			return nil, err
		}
		idx[cid].Comprobantes = append(idx[cid].Comprobantes, x)
	}
	return out, rk.Err()
}

func (r *Postgres) CobrosSesion(ctx context.Context, sesionID string) ([]models.Cobro, error) {
	return r.listarCobros(ctx, `c.sesion_id = $1`, sesionID)
}

func (r *Postgres) CobrosPaciente(ctx context.Context, pacienteID string) ([]models.Cobro, error) {
	return r.listarCobros(ctx, `c.paciente_id = $1`, pacienteID)
}

func (r *Postgres) CobroPorID(ctx context.Context, id string) (*models.Cobro, error) {
	cs, err := r.listarCobros(ctx, `c.id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(cs) == 0 {
		return nil, ErrNoEncontrado
	}
	return &cs[0], nil
}

// EstadoCajaDeCobro: estado de la sesión del cobro y su cajero (para anular sólo en caja abierta y propia).
func (r *Postgres) EstadoCajaDeCobro(ctx context.Context, cobroID string) (estado, cajeroID string, err error) {
	err = r.pool.QueryRow(ctx, `SELECT s.estado, s.cajero_id FROM cobros c JOIN caja_sesiones s ON s.id = c.sesion_id WHERE c.id = $1`, cobroID).Scan(&estado, &cajeroID)
	return estado, cajeroID, traducir(err)
}

// ---------------------------------------------------------------- Comprobantes

func (r *Postgres) SiguienteSecuencial(ctx context.Context, tipo string) (int64, error) {
	seq := map[string]string{"FACTURA": "comprobantes_factura", "NOTA_CREDITO": "comprobantes_nota_credito"}[tipo]
	if seq == "" {
		return 0, ErrNoEncontrado
	}
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT nextval($1::regclass)`, seq).Scan(&n)
	return n, err
}

type NuevoComprobante struct {
	CobroID, Tipo, Estado, Proveedor, UsuarioID, Hash string
	Secuencial                                        int64
	Numero, Autorizacion, Mensaje                     *string
	Simulado                                          bool
	Documento                                         any
}

func (r *Postgres) RegistrarComprobante(ctx context.Context, n NuevoComprobante) error {
	doc, err := json.Marshal(n.Documento)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO comprobantes (cobro_id, tipo, estado, secuencial, numero, autorizacion, proveedor, simulado, mensaje, documento, hash_documento, emitido_por)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, n.CobroID, n.Tipo, n.Estado, n.Secuencial, n.Numero, n.Autorizacion, n.Proveedor, n.Simulado,
		n.Mensaje, doc, n.Hash, n.UsuarioID)
	return traducir(err)
}

// ---------------------------------------------------------------- Seguros y panel

func (r *Postgres) CuentasPorCobrarSeguros(ctx context.Context) ([]models.CuentaSeguro, error) {
	rows, err := r.pool.Query(ctx, `SELECT cv.id, cv.nombre, cv.aseguradora, count(DISTINCT c.id), sum(p.monto)::text
		FROM cobro_pagos p JOIN cobros c ON c.id = p.cobro_id JOIN convenios cv ON cv.id = p.convenio_id
		WHERE p.medio = 'SEGURO' AND NOT EXISTS (SELECT 1 FROM cobros_anulaciones a WHERE a.cobro_id = c.id)
		GROUP BY cv.id, cv.nombre, cv.aseguradora ORDER BY cv.nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CuentaSeguro{}
	for rows.Next() {
		var x models.CuentaSeguro
		if err := rows.Scan(&x.ConvenioID, &x.Convenio, &x.Aseguradora, &x.Cobros, &x.Total); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// ResumenCierres para el panel del Auditor: cierres por revisar.
func (r *Postgres) CierresPorRevisar(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM caja_sesiones WHERE estado='CERRADA'`).Scan(&n)
	return n, err
}

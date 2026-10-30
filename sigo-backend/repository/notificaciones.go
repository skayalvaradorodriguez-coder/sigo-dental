package repository

import (
	"context"
	"fmt"
	"time"

	"sigo-backend/models"
)

const colsNotif = `n.id, n.cita_id, n.tipo, n.canal, n.destinatario, n.programada_para, n.estado, n.motivo, n.proveedor,
	n.simulado, n.mensaje, n.id_proveedor, n.enviada_en`

func (r *Postgres) NotificacionesDeCita(ctx context.Context, citaID string) ([]models.Notificacion, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+colsNotif+` FROM notificaciones n WHERE n.cita_id=$1 ORDER BY n.programada_para, n.tipo`, citaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Notificacion{}
	for rows.Next() {
		var n models.Notificacion
		if err := rows.Scan(&n.ID, &n.CitaID, &n.Tipo, &n.Canal, &n.Destinatario, &n.ProgramadaPara, &n.Estado, &n.Motivo,
			&n.Proveedor, &n.Simulado, &n.Mensaje, &n.IDProveedor, &n.EnviadaEn); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *Postgres) BandejaNotificaciones(ctx context.Context, page, limit int) ([]models.Notificacion, error) {
	_, limit, off := paginar(page, limit)
	rows, err := r.pool.Query(ctx, `SELECT `+colsNotif+`, p.nombres || ' ' || p.apellidos, ci.inicio FROM notificaciones n
		JOIN citas ci ON ci.id = n.cita_id JOIN pacientes p ON p.id = ci.paciente_id
		ORDER BY n.programada_para DESC`+fmt.Sprintf(` LIMIT %d OFFSET %d`, limit, off))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Notificacion{}
	for rows.Next() {
		var n models.Notificacion
		var ini time.Time
		if err := rows.Scan(&n.ID, &n.CitaID, &n.Tipo, &n.Canal, &n.Destinatario, &n.ProgramadaPara, &n.Estado, &n.Motivo,
			&n.Proveedor, &n.Simulado, &n.Mensaje, &n.IDProveedor, &n.EnviadaEn, &n.Paciente, &ini); err != nil {
			return nil, err
		}
		n.InicioCita = &ini
		out = append(out, n)
	}
	return out, rows.Err()
}

// EnvioPendiente: lo que el despachador necesita para armar y enviar un mensaje.
type EnvioPendiente struct {
	ID, Tipo, Canal, Nombres, Odontologo, Sede, EstadoCita string
	Destinatario                                           *string
	Inicio                                                 time.Time
	Intentos                                               int
}

// ResultadoEnvio lo devuelve la función que envía.
type ResultadoEnvio struct {
	Texto, Proveedor, IDProveedor string
	Simulado                      bool
	Err                           error
}

// ProcesarVencidas toma los mensajes vencidos con FOR UPDATE SKIP LOCKED (varias réplicas
// de la API no envían dos veces el mismo) y registra el resultado de cada envío.
func (r *Postgres) ProcesarVencidas(ctx context.Context, limite int, ahora time.Time, enviar func(EnvioPendiente) ResultadoEnvio) (enviados, fallidos int, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	rows, err := tx.Query(ctx, `SELECT n.id, n.tipo, n.canal, n.destinatario, n.intentos, p.nombres, u.nombre, se.nombre, ci.inicio, ci.estado
		FROM notificaciones n JOIN citas ci ON ci.id = n.cita_id JOIN pacientes p ON p.id = ci.paciente_id
		JOIN usuarios u ON u.id = ci.odontologo_id JOIN sillones s ON s.id = ci.sillon_id
		JOIN consultorios co ON co.id = s.consultorio_id JOIN sedes se ON se.id = co.sede_id
		WHERE n.estado='PENDIENTE' AND n.programada_para <= $1 ORDER BY n.programada_para LIMIT $2
		FOR UPDATE OF n SKIP LOCKED`, ahora, limite)
	if err != nil {
		return 0, 0, err
	}
	var lote []EnvioPendiente
	for rows.Next() {
		var e EnvioPendiente
		if err := rows.Scan(&e.ID, &e.Tipo, &e.Canal, &e.Destinatario, &e.Intentos, &e.Nombres, &e.Odontologo, &e.Sede, &e.Inicio, &e.EstadoCita); err != nil {
			rows.Close()
			return 0, 0, err
		}
		lote = append(lote, e)
	}
	rows.Close()
	for _, e := range lote {
		if e.EstadoCita == "CANCELADO" || e.EstadoCita == "NO_ASISTIO" {
			if _, err := tx.Exec(ctx, `UPDATE notificaciones SET estado='CANCELADO', motivo='La cita ya no está activa.' WHERE id=$1`, e.ID); err != nil {
				return 0, 0, err
			}
			continue
		}
		res := enviar(e)
		if res.Err == nil {
			_, err = tx.Exec(ctx, `UPDATE notificaciones SET estado='ENVIADO', mensaje=$2, proveedor=$3, simulado=$4, id_proveedor=$5,
				intentos=intentos+1, enviada_en=now() WHERE id=$1`, e.ID, res.Texto, res.Proveedor, res.Simulado, res.IDProveedor)
			enviados++
		} else {
			estado := "PENDIENTE"
			if e.Intentos+1 >= 3 { // hasta 3 intentos; luego queda FALLIDO para revisión manual
				estado = "FALLIDO"
			}
			_, err = tx.Exec(ctx, `UPDATE notificaciones SET estado=$2, motivo=$3, mensaje=$4, proveedor=$5, simulado=$6, intentos=intentos+1 WHERE id=$1`,
				e.ID, estado, res.Err.Error(), res.Texto, res.Proveedor, res.Simulado)
			fallidos++
		}
		if err != nil {
			return 0, 0, err
		}
	}
	return enviados, fallidos, tx.Commit(ctx)
}

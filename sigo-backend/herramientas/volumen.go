// Package herramientas: utilidades de medición de la Etapa 7 (nunca en producción).
//   - Volumen: crea un paciente con 10 años de historia para medir RNF-01 (A-15).
//   - MedirExpediente: tiempos de carga del expediente por la API (RNF-01).
//   - Carga: usuarios virtuales concurrentes con un uso realista (RNF-03).
//
// Todos los datos son FICTICIOS. Se recomienda correrlas sobre una base aparte (DB_NAME=sigo_rnf).
package herramientas

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const DocumentoVolumen = "VOL000001"

// Protocolo de volumen (docs/05 §5.3): 10 años de historia, 120 citas, 200 notas SOEP, odontograma
// completo (~650 eventos), 20 periodontogramas completos (3 360 sitios), 300 procedimientos realizados,
// 10 presupuestos con 5 ítems y 3 versiones de anamnesis.
func Volumen(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var id string
	if err := pool.QueryRow(ctx, `SELECT id FROM pacientes WHERE numero_documento=$1`, DocumentoVolumen).Scan(&id); err == nil {
		return id, nil // ya existe
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	pasos := []struct{ desc, sql string }{
		{"paciente", `INSERT INTO pacientes (tipo_documento, numero_documento, nombres, apellidos, fecha_nacimiento, sexo, telefono, emergencia_nombre,
			emergencia_telefono, canal_preferido, autoriza_mensajes, creado_por)
			VALUES ('CEDULA', 'VOL000001', 'Paciente Volumen', 'RNF-01 (ficticio)', '1970-05-20', 'OTRO', '+000 0000 9999', 'Contacto (ficticio)',
			'+000 0000 9998', 'SMS', false, (SELECT id FROM usuarios WHERE email='recepcion@demo.sigo'))`},
		{"anamnesis", `INSERT INTO anamnesis (paciente_id, version, enfermedades_sistemicas, alergias, coagulopatia, anticoagulado, registrado_por, registrado_en)
			SELECT p.id, v, '["Hipertensión arterial"]'::jsonb, '[{"sustancia":"Penicilina","tipo":"MEDICAMENTO"}]'::jsonb, false, v = 3,
			(SELECT id FROM usuarios WHERE email='odontologo@demo.sigo'), now() - (4 - v) * interval '3 years'
			FROM pacientes p, generate_series(1, 3) v WHERE p.numero_documento = 'VOL000001'`},
		{"citas", `INSERT INTO citas (paciente_id, odontologo_id, sillon_id, inicio, fin, estado, motivo, creado_por)
			SELECT p.id, (SELECT id FROM usuarios WHERE email='odontologo@demo.sigo'), (SELECT id FROM sillones WHERE nombre='Sillón 2A'),
			date_trunc('day', now()) - (n * interval '30 days') + interval '7 hours 15 minutes',
			date_trunc('day', now()) - (n * interval '30 days') + interval '7 hours 45 minutes', 'ATENDIDO', 'Control (volumen)',
			(SELECT id FROM usuarios WHERE email='recepcion@demo.sigo')
			FROM pacientes p, generate_series(1, 120) n WHERE p.numero_documento = 'VOL000001'`},
		{"notas", `INSERT INTO notas_soep (paciente_id, autor_id, subjetivo, objetivo, evaluacion, plan, creado_en, bloqueo_en)
			SELECT p.id, (SELECT id FROM usuarios WHERE email='odontologo@demo.sigo'),
			'Refiere molestias leves en la consulta ' || n || '. ' || repeat('Texto de relleno ficticio. ', 10),
			'Examen intraoral sin cambios significativos. ' || repeat('Hallazgo de ejemplo. ', 10),
			'Evolución favorable (nota ' || n || ').', 'Control en 6 meses. ' || repeat('Indicación de ejemplo. ', 5),
			now() - (n * interval '18 days'), now() - (n * interval '18 days') + interval '24 hours'
			FROM pacientes p, generate_series(1, 200) n WHERE p.numero_documento = 'VOL000001'`},
		{"odontograma", `INSERT INTO odontograma_eventos (paciente_id, diente, superficie, hallazgo, accion, registrado_por, registrado_en)
			SELECT p.id, d, s, CASE WHEN (d + ascii(s)) % 3 = 0 THEN 'CARIES' ELSE 'RESTAURACION' END,
			CASE WHEN k = 1 THEN 'AGREGAR' WHEN k = 2 THEN 'QUITAR' ELSE 'AGREGAR' END,
			(SELECT id FROM usuarios WHERE email='odontologo@demo.sigo'), now() - ((4 - k) * interval '3 years') + d * interval '1 hour'
			FROM pacientes p, unnest(ARRAY[11,12,13,14,15,16,17,21,22,23,24,25,26,27,31,32,33,34,35,36,37,41,42,43,44,45,46,47]) d,
			     unnest(ARRAY['O','M','D','V','L']) s, generate_series(1, CASE WHEN true THEN 3 ELSE 1 END) k
			WHERE p.numero_documento = 'VOL000001' AND (d + ascii(s) + k) % 2 = 0`},
		{"periodontogramas", `INSERT INTO periodontogramas (paciente_id, observaciones, registrado_por, registrado_en)
			SELECT p.id, 'Control periodontal ' || n || ' (volumen)', (SELECT id FROM usuarios WHERE email='odontologo@demo.sigo'), now() - n * interval '180 days'
			FROM pacientes p, generate_series(1, 20) n WHERE p.numero_documento = 'VOL000001'`},
		{"perio_dientes", `INSERT INTO perio_dientes (periodontograma_id, diente, ausente, movilidad, furca)
			SELECT g.id, d, false, 0, 0 FROM periodontogramas g JOIN pacientes p ON p.id = g.paciente_id,
			unnest(ARRAY[11,12,13,14,15,16,17,21,22,23,24,25,26,27,31,32,33,34,35,36,37,41,42,43,44,45,46,47]) d WHERE p.numero_documento = 'VOL000001'`},
		{"perio_sitios", `INSERT INTO perio_sitios (periodontograma_id, diente, sitio, profundidad, margen, sangrado, supuracion)
			SELECT d.periodontograma_id, d.diente, s, 2 + (d.diente + ascii(s)) % 3, 0, (d.diente + ascii(s)) % 5 = 0, false
			FROM perio_dientes d JOIN periodontogramas g ON g.id = d.periodontograma_id JOIN pacientes p ON p.id = g.paciente_id,
			unnest(ARRAY['MV','V','DV','ML','L','DL']) s WHERE p.numero_documento = 'VOL000001'`},
		{"procedimientos", `INSERT INTO procedimientos_realizados (paciente_id, procedimiento_id, diente, observaciones, odontologo_id, realizado_en, hash_firma)
			SELECT p.id, (SELECT id FROM procedimientos WHERE codigo = (ARRAY['PRO-01','RES-01','SEL-01','VAL-01'])[1 + n % 4]),
			CASE WHEN n % 4 IN (1, 2) THEN (ARRAY[16,26,36,46,11,21])[1 + n % 6] END, 'Registro de volumen ' || n,
			(SELECT id FROM usuarios WHERE email='odontologo@demo.sigo'), now() - n * interval '12 days', 'volumen-rnf01'
			FROM pacientes p, generate_series(1, 300) n WHERE p.numero_documento = 'VOL000001'`},
		{"presupuestos", `INSERT INTO presupuestos (paciente_id, odontologo_id, titulo, creado_en)
			SELECT p.id, (SELECT id FROM usuarios WHERE email='odontologo@demo.sigo'), 'Plan ' || n || ' (volumen)', now() - n * interval '1 year'
			FROM pacientes p, generate_series(1, 10) n WHERE p.numero_documento = 'VOL000001'`},
		{"presupuesto_items", `INSERT INTO presupuesto_items (presupuesto_id, orden, fase, fase_nombre, sesion, procedimiento_id, honorario)
			SELECT b.id, k, 1 + k / 3, 'Fase ' || (1 + k / 3), k, (SELECT id FROM procedimientos WHERE codigo='RES-01'), 45.00
			FROM presupuestos b JOIN pacientes p ON p.id = b.paciente_id, generate_series(1, 5) k WHERE p.numero_documento = 'VOL000001'`},
	}
	for _, x := range pasos {
		if _, err := tx.Exec(ctx, x.sql); err != nil {
			return "", fmt.Errorf("volumen (%s): %w", x.desc, err)
		}
	}
	if err := tx.QueryRow(ctx, `SELECT id FROM pacientes WHERE numero_documento=$1`, DocumentoVolumen).Scan(&id); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"sigo-backend/domain"
	"sigo-backend/models"
)

// ---------------------------------------------------------------- Odontograma (RF-03)

func (r *Postgres) EventosOdontograma(ctx context.Context, pacienteID string) ([]models.EventoOdontograma, error) {
	rows, err := r.pool.Query(ctx, `SELECT e.id, e.diente, e.superficie, e.hallazgo, e.accion, e.nota, u.nombre, e.registrado_en
		FROM odontograma_eventos e JOIN usuarios u ON u.id = e.registrado_por WHERE e.paciente_id=$1 ORDER BY e.id`, pacienteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.EventoOdontograma{}
	for rows.Next() {
		var e models.EventoOdontograma
		if err := rows.Scan(&e.ID, &e.Diente, &e.Superficie, &e.Hallazgo, &e.Accion, &e.Nota, &e.RegistradoPor, &e.RegistradoEn); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Postgres) AgregarEventoOdontograma(ctx context.Context, pacienteID string, diente int, superficie *string, hallazgo, accion string, nota *string, usuarioID string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `INSERT INTO odontograma_eventos (paciente_id, diente, superficie, hallazgo, accion, nota, registrado_por)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`, pacienteID, diente, superficie, hallazgo, accion, nota, usuarioID).Scan(&id)
	return id, traducir(err)
}

// ---------------------------------------------------------------- Periodontograma (RF-04)

func (r *Postgres) CrearPeriodontograma(ctx context.Context, pacienteID string, obs *string, dientes []models.PerioDiente, usuarioID string) (string, error) {
	var id string
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO periodontogramas (paciente_id, observaciones, registrado_por) VALUES ($1,$2,$3) RETURNING id`,
			pacienteID, obs, usuarioID).Scan(&id); err != nil {
			return err
		}
		for _, d := range dientes {
			if _, err := tx.Exec(ctx, `INSERT INTO perio_dientes (periodontograma_id, diente, ausente, movilidad, furca) VALUES ($1,$2,$3,$4,$5)`,
				id, d.Diente, d.Ausente, d.Movilidad, d.Furca); err != nil {
				return err
			}
			for _, s := range d.Sitios {
				if _, err := tx.Exec(ctx, `INSERT INTO perio_sitios (periodontograma_id, diente, sitio, profundidad, margen, sangrado, supuracion)
					VALUES ($1,$2,$3,$4,$5,$6,$7)`, id, d.Diente, s.Sitio, s.Profundidad, s.Margen, s.Sangrado, s.Supuracion); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return id, traducir(err)
}

func (r *Postgres) Periodontogramas(ctx context.Context, pacienteID string) ([]models.Periodontograma, error) {
	rows, err := r.pool.Query(ctx, `SELECT p.id, p.observaciones, u.nombre, p.registrado_en FROM periodontogramas p
		JOIN usuarios u ON u.id = p.registrado_por WHERE p.paciente_id=$1 ORDER BY p.registrado_en DESC`, pacienteID)
	if err != nil {
		return nil, err
	}
	out := []models.Periodontograma{}
	idx := map[string]int{}
	for rows.Next() {
		var p models.Periodontograma
		if err := rows.Scan(&p.ID, &p.Observaciones, &p.RegistradoPor, &p.RegistradoEn); err != nil {
			rows.Close()
			return nil, err
		}
		p.Dientes = []models.PerioDiente{}
		idx[p.ID] = len(out)
		out = append(out, p)
	}
	rows.Close()
	if len(out) == 0 {
		return out, rows.Err()
	}
	ids := make([]string, 0, len(out))
	for _, p := range out {
		ids = append(ids, p.ID)
	}
	// Dientes y sitios de todos los exámenes en una sola consulta.
	rows, err = r.pool.Query(ctx, `SELECT d.periodontograma_id::text, d.diente, d.ausente, d.movilidad, d.furca,
		s.sitio, s.profundidad, s.margen, s.nic, s.sangrado, s.supuracion
		FROM perio_dientes d LEFT JOIN perio_sitios s ON s.periodontograma_id = d.periodontograma_id AND s.diente = d.diente
		WHERE d.periodontograma_id = ANY($1::uuid[])
		ORDER BY d.periodontograma_id, d.diente, array_position(ARRAY['MV','V','DV','ML','L','DL'], s.sitio)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sitiosResumen := map[string][]domain.SitioPerio{}
	for rows.Next() {
		var pid string
		var d models.PerioDiente
		var sitio *string
		var ps, mg, nic *int
		var sang, sup *bool
		if err := rows.Scan(&pid, &d.Diente, &d.Ausente, &d.Movilidad, &d.Furca, &sitio, &ps, &mg, &nic, &sang, &sup); err != nil {
			return nil, err
		}
		p := &out[idx[pid]]
		if n := len(p.Dientes); n == 0 || p.Dientes[n-1].Diente != d.Diente {
			d.Sitios = []models.PerioSitio{}
			p.Dientes = append(p.Dientes, d)
		}
		if sitio != nil {
			cur := &p.Dientes[len(p.Dientes)-1]
			cur.Sitios = append(cur.Sitios, models.PerioSitio{Sitio: *sitio, Profundidad: *ps, Margen: *mg, NIC: *nic, Sangrado: *sang, Supuracion: *sup})
			sitiosResumen[pid] = append(sitiosResumen[pid], domain.SitioPerio{Profundidad: *ps, Margen: *mg, Sangrado: *sang})
		}
	}
	for i := range out {
		out[i].Resumen = domain.Resumir(sitiosResumen[out[i].ID])
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- Notas SOEP (RF-05)

const colsNota = `n.id, n.paciente_id, n.cita_id, n.autor_id, au.nombre, n.subjetivo, n.objetivo, n.evaluacion, n.plan,
	n.creado_en, n.actualizado_en, n.bloqueo_en, n.firmada_en, n.firmada_por, fu.nombre, fu.registro_profesional, n.hash_firma`

const fromNota = ` FROM notas_soep n JOIN usuarios au ON au.id = n.autor_id LEFT JOIN usuarios fu ON fu.id = n.firmada_por`

func scanNota(row pgx.Row) (*models.NotaSOEP, error) {
	n := &models.NotaSOEP{Adendas: []models.Adenda{}}
	err := row.Scan(&n.ID, &n.PacienteID, &n.CitaID, &n.AutorID, &n.Autor, &n.Subjetivo, &n.Objetivo, &n.Evaluacion, &n.Plan,
		&n.CreadoEn, &n.ActualizadoEn, &n.BloqueoEn, &n.FirmadaEn, &n.FirmadaPorID, &n.FirmadaPor, &n.RegistroProfesional, &n.HashFirma)
	if err != nil {
		return nil, traducir(err)
	}
	return n, nil
}

func (r *Postgres) NotasPaciente(ctx context.Context, pacienteID string) ([]models.NotaSOEP, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+colsNota+fromNota+` WHERE n.paciente_id=$1 ORDER BY n.creado_en DESC`, pacienteID)
	if err != nil {
		return nil, err
	}
	out := []models.NotaSOEP{}
	idx := map[string]int{}
	for rows.Next() {
		n, err := scanNota(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		idx[n.ID] = len(out)
		out = append(out, *n)
	}
	rows.Close()
	adendas, err := r.adendas(ctx, `SELECT a.nota_id::text, a.id, u.nombre, a.texto, a.creado_en, a.hash_firma FROM notas_adendas a
		JOIN usuarios u ON u.id = a.autor_id JOIN notas_soep n ON n.id = a.nota_id WHERE n.paciente_id=$1 ORDER BY a.creado_en`, pacienteID)
	if err != nil {
		return nil, err
	}
	for nid, as := range adendas {
		out[idx[nid]].Adendas = as
	}
	return out, nil
}

func (r *Postgres) NotaPorID(ctx context.Context, id string) (*models.NotaSOEP, error) {
	n, err := scanNota(r.pool.QueryRow(ctx, `SELECT `+colsNota+fromNota+` WHERE n.id=$1`, id))
	if err != nil {
		return nil, err
	}
	as, err := r.adendas(ctx, `SELECT a.nota_id::text, a.id, u.nombre, a.texto, a.creado_en, a.hash_firma FROM notas_adendas a
		JOIN usuarios u ON u.id = a.autor_id WHERE a.nota_id=$1 ORDER BY a.creado_en`, id)
	if err != nil {
		return nil, err
	}
	if v, ok := as[id]; ok {
		n.Adendas = v
	}
	return n, nil
}

func (r *Postgres) adendas(ctx context.Context, q string, arg any) (map[string][]models.Adenda, error) {
	rows, err := r.pool.Query(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]models.Adenda{}
	for rows.Next() {
		var padre string
		var a models.Adenda
		if err := rows.Scan(&padre, &a.ID, &a.Autor, &a.Texto, &a.CreadoEn, &a.HashFirma); err != nil {
			return nil, err
		}
		out[padre] = append(out[padre], a)
	}
	return out, rows.Err()
}

type ContenidoSOEP struct{ Subjetivo, Objetivo, Evaluacion, Plan string }

func (r *Postgres) CrearNota(ctx context.Context, pacienteID string, citaID *string, autorID string, c ContenidoSOEP, horasBloqueo int) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `INSERT INTO notas_soep (paciente_id, cita_id, autor_id, subjetivo, objetivo, evaluacion, plan, bloqueo_en)
		VALUES ($1,$2,$3,$4,$5,$6,$7, now() + make_interval(hours => $8)) RETURNING id`,
		pacienteID, citaID, autorID, c.Subjetivo, c.Objetivo, c.Evaluacion, c.Plan, horasBloqueo).Scan(&id)
	return id, traducir(err)
}

// ActualizarNota: la guardia de la BD rechaza el cambio si la nota está firmada o bloqueada.
func (r *Postgres) ActualizarNota(ctx context.Context, id string, c ContenidoSOEP) error {
	tag, err := r.pool.Exec(ctx, `UPDATE notas_soep SET subjetivo=$2, objetivo=$3, evaluacion=$4, plan=$5, actualizado_en=now()
		WHERE id=$1 AND firmada_en IS NULL AND now() < bloqueo_en`, id, c.Subjetivo, c.Objetivo, c.Evaluacion, c.Plan)
	if err != nil {
		return traducir(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflicto
	}
	return nil
}

func (r *Postgres) FirmarNota(ctx context.Context, id, firmanteID string, firmadaEn time.Time, hash string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE notas_soep SET firmada_en=$2, firmada_por=$3, hash_firma=$4 WHERE id=$1 AND firmada_en IS NULL`,
		id, firmadaEn, firmanteID, hash)
	if err != nil {
		return traducir(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflicto
	}
	return nil
}

func (r *Postgres) AgregarAdendaNota(ctx context.Context, notaID, autorID, texto string, creadoEn time.Time, hash string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO notas_adendas (nota_id, autor_id, texto, creado_en, hash_firma) VALUES ($1,$2,$3,$4,$5)`,
		notaID, autorID, texto, creadoEn, hash)
	return traducir(err)
}

// ---------------------------------------------------------------- Procedimientos realizados (RN-01)

type NuevoRealizado struct {
	PacienteID, ProcedimientoID, OdontologoID string
	CitaID, Observaciones                     *string
	Diente                                    *int
	Superficies                               []string
	RealizadoEn                               time.Time
	Hash                                      string
}

func (r *Postgres) RegistrarRealizado(ctx context.Context, n NuevoRealizado) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `INSERT INTO procedimientos_realizados (paciente_id, procedimiento_id, cita_id, diente, superficies,
		observaciones, odontologo_id, realizado_en, hash_firma) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		n.PacienteID, n.ProcedimientoID, n.CitaID, n.Diente, n.Superficies, n.Observaciones, n.OdontologoID, n.RealizadoEn, n.Hash).Scan(&id)
	return id, traducir(err)
}

func (r *Postgres) RealizadosPaciente(ctx context.Context, pacienteID string) ([]models.ProcedimientoRealizado, error) {
	rows, err := r.pool.Query(ctx, `SELECT r.id, r.procedimiento_id, p.codigo, p.nombre, r.cita_id, r.diente, r.superficies, r.observaciones,
		r.odontologo_id, u.nombre, r.realizado_en, r.hash_firma
		FROM procedimientos_realizados r JOIN procedimientos p ON p.id = r.procedimiento_id JOIN usuarios u ON u.id = r.odontologo_id
		WHERE r.paciente_id=$1 ORDER BY r.realizado_en DESC`, pacienteID)
	if err != nil {
		return nil, err
	}
	out := []models.ProcedimientoRealizado{}
	idx := map[string]int{}
	for rows.Next() {
		var x models.ProcedimientoRealizado
		var diente *int16
		if err := rows.Scan(&x.ID, &x.ProcedimientoID, &x.Codigo, &x.Procedimiento, &x.CitaID, &diente, &x.Superficies, &x.Observaciones,
			&x.OdontologoID, &x.Odontologo, &x.RealizadoEn, &x.HashFirma); err != nil {
			rows.Close()
			return nil, err
		}
		if diente != nil {
			d := int(*diente)
			x.Diente = &d
		}
		x.Adendas = []models.Adenda{}
		idx[x.ID] = len(out)
		out = append(out, x)
	}
	rows.Close()
	as, err := r.adendas(ctx, `SELECT a.procedimiento_realizado_id::text, a.id, u.nombre, a.texto, a.creado_en, a.hash_firma
		FROM procedimientos_adendas a JOIN usuarios u ON u.id = a.autor_id
		JOIN procedimientos_realizados r ON r.id = a.procedimiento_realizado_id WHERE r.paciente_id=$1 ORDER BY a.creado_en`, pacienteID)
	if err != nil {
		return nil, err
	}
	for rid, v := range as {
		out[idx[rid]].Adendas = v
	}
	return out, nil
}

// PacienteDeRealizado devuelve el paciente de un procedimiento realizado (para auditar y validar).
func (r *Postgres) PacienteDeRealizado(ctx context.Context, id string) (string, error) {
	var pid string
	err := r.pool.QueryRow(ctx, `SELECT paciente_id FROM procedimientos_realizados WHERE id=$1`, id).Scan(&pid)
	return pid, traducir(err)
}

func (r *Postgres) AgregarAdendaRealizado(ctx context.Context, realizadoID, autorID, texto string, creadoEn time.Time, hash string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO procedimientos_adendas (procedimiento_realizado_id, autor_id, texto, creado_en, hash_firma)
		VALUES ($1,$2,$3,$4,$5)`, realizadoID, autorID, texto, creadoEn, hash)
	return traducir(err)
}

// CitaDelPaciente: la cita existe y pertenece al paciente.
func (r *Postgres) CitaDelPaciente(ctx context.Context, citaID, pacienteID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM citas WHERE id=$1 AND paciente_id=$2)`, citaID, pacienteID).Scan(&ok)
	return ok, err
}

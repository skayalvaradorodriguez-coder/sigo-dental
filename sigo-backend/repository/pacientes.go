package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"sigo-backend/domain"
	"sigo-backend/models"
)

const colsAnamnesis = `id, version, enfermedades_sistemicas, alergias, antecedentes_quirurgicos, coagulopatia, anticoagulado,
	medicacion_actual, embarazo, otra_condicion_critica, observaciones, registrado_por::text, registrado_en`

func scanAnamnesis(row pgx.Row, pacienteID *string) (*models.Anamnesis, error) {
	a := &models.Anamnesis{}
	var enf, al []byte
	dest := []any{&a.ID, &a.Version, &enf, &al, &a.AntecedentesQuirurgicos, &a.Coagulopatia, &a.Anticoagulado,
		&a.MedicacionActual, &a.Embarazo, &a.OtraCondicionCritica, &a.Observaciones, &a.RegistradoPor, &a.RegistradoEn}
	if pacienteID != nil {
		dest = append([]any{pacienteID}, dest...)
	}
	if err := row.Scan(dest...); err != nil {
		return nil, traducir(err)
	}
	_ = json.Unmarshal(enf, &a.EnfermedadesSistemicas)
	_ = json.Unmarshal(al, &a.Alergias)
	if a.EnfermedadesSistemicas == nil {
		a.EnfermedadesSistemicas = []string{}
	}
	if a.Alergias == nil {
		a.Alergias = []domain.Alergia{}
	}
	return a, nil
}

// AnamnesisVigentes devuelve la versión más alta de cada paciente (una sola consulta).
func (r *Postgres) AnamnesisVigentes(ctx context.Context, ids []string) (map[string]*models.Anamnesis, error) {
	out := map[string]*models.Anamnesis{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT ON (paciente_id) paciente_id::text, `+colsAnamnesis+`
		FROM anamnesis WHERE paciente_id = ANY($1::uuid[]) ORDER BY paciente_id, version DESC`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pid string
		a, err := scanAnamnesis(rows, &pid)
		if err != nil {
			return nil, err
		}
		out[pid] = a
	}
	return out, rows.Err()
}

// ResumenAlertas: para agenda y panel (alertas críticas y anamnesis pendiente).
func (r *Postgres) ResumenAlertas(ctx context.Context, ids []string) (map[string][]string, map[string]bool, error) {
	vig, err := r.AnamnesisVigentes(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	crit, pend := map[string][]string{}, map[string]bool{}
	for _, id := range ids {
		a := vig[id]
		pend[id] = a == nil
		crit[id] = domain.SoloCriticas(domain.CalcularAlertas(a.DatosAlerta()))
	}
	return crit, pend, nil
}

const colsPaciente = `p.id, p.tipo_documento, p.numero_documento, p.nombres, p.apellidos, to_char(p.fecha_nacimiento,'YYYY-MM-DD'),
	p.sexo, p.telefono, p.email, p.direccion, p.emergencia_nombre, p.emergencia_telefono, p.emergencia_parentesco,
	p.canal_preferido, p.autoriza_mensajes, p.convenio_id::text, c.nombre, p.numero_afiliado, p.creado_en`

func scanPaciente(row pgx.Row) (*models.Paciente, error) {
	p := &models.Paciente{}
	err := row.Scan(&p.ID, &p.TipoDocumento, &p.NumeroDocumento, &p.Nombres, &p.Apellidos, &p.FechaNacimiento, &p.Sexo,
		&p.Telefono, &p.Email, &p.Direccion, &p.EmergenciaNombre, &p.EmergenciaTelefono, &p.EmergenciaParentesco,
		&p.CanalPreferido, &p.AutorizaMensajes, &p.ConvenioID, &p.Convenio, &p.NumeroAfiliado, &p.CreadoEn)
	if err != nil {
		return nil, traducir(err)
	}
	return p, nil
}

// completar calcula edad, menor de edad, alertas y anamnesis pendiente.
func completar(p *models.Paciente, a *models.Anamnesis, hoy time.Time) {
	nac, _ := time.Parse("2006-01-02", p.FechaNacimiento)
	p.Edad = domain.Edad(nac, hoy)
	p.EsMenor = p.Edad < domain.MayoriaEdad
	p.AnamnesisPendiente = a == nil
	if a != nil {
		p.AnamnesisActualizada = &a.RegistradoEn
	}
	p.Alertas = domain.CalcularAlertas(a.DatosAlerta())
}

func (r *Postgres) BuscarPacientes(ctx context.Context, q string, page, limit int, hoy time.Time) (models.Pagina[models.Paciente], error) {
	page, limit, off := paginar(page, limit)
	res := models.Pagina[models.Paciente]{Data: []models.Paciente{}, Page: page, Limit: limit}
	where, args := "TRUE", []any{}
	if q = strings.TrimSpace(q); q != "" {
		args = append(args, "%"+escaparLike(q)+"%")
		where = `(p.nombres ILIKE $1 OR p.apellidos ILIKE $1 OR p.numero_documento ILIKE $1 OR (p.nombres || ' ' || p.apellidos) ILIKE $1)`
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM pacientes p WHERE `+where, args...).Scan(&res.Total); err != nil {
		return res, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+colsPaciente+` FROM pacientes p LEFT JOIN convenios c ON c.id = p.convenio_id
		WHERE `+where+fmt.Sprintf(` ORDER BY p.apellidos, p.nombres LIMIT %d OFFSET %d`, limit, off), args...)
	if err != nil {
		return res, err
	}
	ids := []string{}
	for rows.Next() {
		p, err := scanPaciente(rows)
		if err != nil {
			rows.Close()
			return res, err
		}
		res.Data = append(res.Data, *p)
		ids = append(ids, p.ID)
	}
	rows.Close()
	vig, err := r.AnamnesisVigentes(ctx, ids)
	if err != nil {
		return res, err
	}
	for i := range res.Data {
		completar(&res.Data[i], vig[res.Data[i].ID], hoy)
	}
	return res, nil
}

func (r *Postgres) PacientePorID(ctx context.Context, id string, hoy time.Time) (*models.Paciente, error) {
	p, err := scanPaciente(r.pool.QueryRow(ctx, `SELECT `+colsPaciente+` FROM pacientes p LEFT JOIN convenios c ON c.id = p.convenio_id WHERE p.id=$1`, id))
	if err != nil {
		return nil, err
	}
	t := &models.Tutor{}
	err = r.pool.QueryRow(ctx, `SELECT nombre, tipo_documento, numero_documento, parentesco, telefono, email FROM tutores WHERE paciente_id=$1`, id).
		Scan(&t.Nombre, &t.TipoDocumento, &t.NumeroDocumento, &t.Parentesco, &t.Telefono, &t.Email)
	if err == nil {
		p.Tutor = t
	} else if err != pgx.ErrNoRows {
		return nil, err
	}
	vig, err := r.AnamnesisVigentes(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	completar(p, vig[id], hoy)
	return p, nil
}

// DatosPaciente: valores ya validados que se guardan en la ficha.
type DatosPaciente struct {
	TipoDocumento, NumeroDocumento, Nombres, Apellidos, FechaNacimiento, Sexo string
	Telefono, Email, Direccion                                                *string
	EmergenciaNombre, EmergenciaTelefono                                      string
	EmergenciaParentesco                                                      *string
	CanalPreferido                                                            string
	AutorizaMensajes                                                          bool
	ConvenioID, NumeroAfiliado                                                *string
}

func (r *Postgres) guardarTutor(ctx context.Context, tx pgx.Tx, pacienteID string, t *models.Tutor) error {
	_, err := tx.Exec(ctx, `INSERT INTO tutores (paciente_id, nombre, tipo_documento, numero_documento, parentesco, telefono, email)
		VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (paciente_id) DO UPDATE SET nombre=EXCLUDED.nombre, tipo_documento=EXCLUDED.tipo_documento,
		numero_documento=EXCLUDED.numero_documento, parentesco=EXCLUDED.parentesco, telefono=EXCLUDED.telefono, email=EXCLUDED.email, actualizado_en=now()`,
		pacienteID, t.Nombre, t.TipoDocumento, t.NumeroDocumento, t.Parentesco, t.Telefono, t.Email)
	return err
}

func (r *Postgres) CrearPaciente(ctx context.Context, d DatosPaciente, tutor *models.Tutor, creadoPor string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO pacientes (tipo_documento, numero_documento, nombres, apellidos, fecha_nacimiento, sexo,
		telefono, email, direccion, emergencia_nombre, emergencia_telefono, emergencia_parentesco, canal_preferido, autoriza_mensajes,
		convenio_id, numero_afiliado, creado_por) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING id`,
		d.TipoDocumento, strings.ToUpper(d.NumeroDocumento), d.Nombres, d.Apellidos, d.FechaNacimiento, d.Sexo, d.Telefono, d.Email,
		d.Direccion, d.EmergenciaNombre, d.EmergenciaTelefono, d.EmergenciaParentesco, d.CanalPreferido, d.AutorizaMensajes,
		d.ConvenioID, d.NumeroAfiliado, creadoPor).Scan(&id)
	if err != nil {
		return "", traducir(err)
	}
	if tutor != nil {
		if err := r.guardarTutor(ctx, tx, id, tutor); err != nil {
			return "", traducir(err)
		}
	}
	return id, tx.Commit(ctx)
}

// ActualizarPaciente reemplaza la ficha. tutorAccion: "conservar" | "guardar" | "quitar".
func (r *Postgres) ActualizarPaciente(ctx context.Context, id string, d DatosPaciente, tutor *models.Tutor, tutorAccion string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	tag, err := tx.Exec(ctx, `UPDATE pacientes SET tipo_documento=$2, numero_documento=$3, nombres=$4, apellidos=$5, fecha_nacimiento=$6,
		sexo=$7, telefono=$8, email=$9, direccion=$10, emergencia_nombre=$11, emergencia_telefono=$12, emergencia_parentesco=$13,
		canal_preferido=$14, autoriza_mensajes=$15, convenio_id=$16, numero_afiliado=$17, actualizado_en=now() WHERE id=$1`,
		id, d.TipoDocumento, strings.ToUpper(d.NumeroDocumento), d.Nombres, d.Apellidos, d.FechaNacimiento, d.Sexo, d.Telefono, d.Email,
		d.Direccion, d.EmergenciaNombre, d.EmergenciaTelefono, d.EmergenciaParentesco, d.CanalPreferido, d.AutorizaMensajes,
		d.ConvenioID, d.NumeroAfiliado)
	if err != nil {
		return traducir(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	switch tutorAccion {
	case "guardar":
		if err := r.guardarTutor(ctx, tx, id, tutor); err != nil {
			return traducir(err)
		}
	case "quitar":
		if _, err := tx.Exec(ctx, `DELETE FROM tutores WHERE paciente_id=$1`, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Postgres) HistorialAnamnesis(ctx context.Context, pacienteID string) ([]models.Anamnesis, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+colsAnamnesis+` FROM anamnesis WHERE paciente_id=$1 ORDER BY version DESC`, pacienteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Anamnesis{}
	for rows.Next() {
		a, err := scanAnamnesis(rows, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// AgregarAnamnesis crea una versión nueva. El índice único (paciente, versión) hace fallar a la
// segunda de dos escrituras simultáneas en vez de perder información.
func (r *Postgres) AgregarAnamnesis(ctx context.Context, pacienteID string, a models.Anamnesis, usuarioID string) (*models.Anamnesis, error) {
	enf, _ := json.Marshal(a.EnfermedadesSistemicas)
	al, _ := json.Marshal(a.Alergias)
	res, err := scanAnamnesis(r.pool.QueryRow(ctx, `INSERT INTO anamnesis (paciente_id, version, enfermedades_sistemicas, alergias,
		antecedentes_quirurgicos, coagulopatia, anticoagulado, medicacion_actual, embarazo, otra_condicion_critica, observaciones, registrado_por)
		VALUES ($1, (SELECT coalesce(max(version),0)+1 FROM anamnesis WHERE paciente_id=$1), $2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING `+colsAnamnesis, pacienteID, enf, al, a.AntecedentesQuirurgicos, a.Coagulopatia, a.Anticoagulado,
		a.MedicacionActual, a.Embarazo, a.OtraCondicionCritica, a.Observaciones, usuarioID), nil)
	if err == ErrDuplicado {
		return nil, ErrConflicto
	}
	return res, err
}

// DatosContacto: lo que el planificador de recordatorios necesita del paciente.
type DatosContacto struct {
	Nombres, Canal  string
	Telefono, Email *string
	Autoriza        bool
}

func (r *Postgres) ContactoPaciente(ctx context.Context, id string) (*DatosContacto, error) {
	d := &DatosContacto{}
	err := r.pool.QueryRow(ctx, `SELECT nombres, canal_preferido, telefono, email, autoriza_mensajes FROM pacientes WHERE id=$1`, id).
		Scan(&d.Nombres, &d.Canal, &d.Telefono, &d.Email, &d.Autoriza)
	if err != nil {
		return nil, traducir(err)
	}
	return d, nil
}

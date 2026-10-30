package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"sigo-backend/domain"
	"sigo-backend/models"
)

// ---------------------------------------------------------------- Plantillas de consentimiento

const colsPlantilla = `t.id, t.codigo, t.version, t.titulo, t.texto, t.procedimiento_id, pr.nombre, t.creado_en`

// PlantillasVigentes: la última versión de cada código.
func (r *Postgres) PlantillasVigentes(ctx context.Context) ([]models.PlantillaConsentimiento, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT ON (t.codigo) `+colsPlantilla+`
		FROM consentimiento_plantillas t LEFT JOIN procedimientos pr ON pr.id = t.procedimiento_id
		ORDER BY t.codigo, t.version DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.PlantillaConsentimiento{}
	for rows.Next() {
		var p models.PlantillaConsentimiento
		if err := rows.Scan(&p.ID, &p.Codigo, &p.Version, &p.Titulo, &p.Texto, &p.ProcedimientoID, &p.Procedimiento, &p.CreadoEn); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Postgres) PlantillaPorID(ctx context.Context, id string) (*models.PlantillaConsentimiento, error) {
	var p models.PlantillaConsentimiento
	err := r.pool.QueryRow(ctx, `SELECT `+colsPlantilla+` FROM consentimiento_plantillas t LEFT JOIN procedimientos pr ON pr.id = t.procedimiento_id
		WHERE t.id=$1`, id).Scan(&p.ID, &p.Codigo, &p.Version, &p.Titulo, &p.Texto, &p.ProcedimientoID, &p.Procedimiento, &p.CreadoEn)
	if err != nil {
		return nil, traducir(err)
	}
	return &p, nil
}

// NuevaVersionPlantilla: "editar" una plantilla agrega una versión; las anteriores no cambian.
func (r *Postgres) NuevaVersionPlantilla(ctx context.Context, codigo, titulo, texto string, procedimientoID *string, usuarioID string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `INSERT INTO consentimiento_plantillas (codigo, version, titulo, texto, procedimiento_id, creado_por)
		VALUES ($1, (SELECT coalesce(max(version),0)+1 FROM consentimiento_plantillas WHERE codigo=$1), $2,$3,$4,$5) RETURNING id`,
		codigo, titulo, texto, procedimientoID, usuarioID).Scan(&id)
	return id, traducir(err)
}

// ---------------------------------------------------------------- Consentimientos (RF-06)

const colsConsentimiento = `c.id, c.paciente_id, pa.nombres || ' ' || pa.apellidos, c.plantilla_id, t.titulo, t.version, c.procedimiento_id, pr.nombre,
	c.odontologo_id, u.nombre, c.texto, c.hash_documento, c.firmante_relacion, c.firmante_nombre, c.firmante_documento, c.metodo, c.estado,
	c.creado_en, c.firma_imagen_sha256, c.otp_canal, c.otp_destinatario, c.otp_proveedor, c.otp_simulado, c.otp_expira, c.otp_intentos,
	c.firmado_en, c.hash_firma, c.otp_hash`

const fromConsentimiento = ` FROM consentimientos c JOIN pacientes pa ON pa.id = c.paciente_id JOIN consentimiento_plantillas t ON t.id = c.plantilla_id
	JOIN usuarios u ON u.id = c.odontologo_id LEFT JOIN procedimientos pr ON pr.id = c.procedimiento_id`

func scanConsentimiento(row pgx.Row) (*models.Consentimiento, error) {
	c := &models.Consentimiento{}
	err := row.Scan(&c.ID, &c.PacienteID, &c.Paciente, &c.PlantillaID, &c.Titulo, &c.Version, &c.ProcedimientoID, &c.Procedimiento,
		&c.OdontologoID, &c.Odontologo, &c.Texto, &c.HashDocumento, &c.FirmanteRelacion, &c.FirmanteNombre, &c.FirmanteDocumento, &c.Metodo,
		&c.Estado, &c.CreadoEn, &c.FirmaImagenSHA256, &c.OTPCanal, &c.OTPDestinatario, &c.OTPProveedor, &c.OTPSimulado, &c.OTPExpira,
		&c.OTPIntentos, &c.FirmadoEn, &c.HashFirma, &c.OTPHash)
	if err != nil {
		return nil, traducir(err)
	}
	return c, nil
}

func (r *Postgres) listarConsentimientos(ctx context.Context, where string, args ...any) ([]models.Consentimiento, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+colsConsentimiento+fromConsentimiento+` WHERE `+where+` ORDER BY c.creado_en DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Consentimiento{}
	for rows.Next() {
		c, err := scanConsentimiento(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (r *Postgres) ConsentimientosPaciente(ctx context.Context, pacienteID string) ([]models.Consentimiento, error) {
	return r.listarConsentimientos(ctx, `c.paciente_id=$1`, pacienteID)
}

func (r *Postgres) ConsentimientosRecientes(ctx context.Context, estado string) ([]models.Consentimiento, error) {
	if estado == "" {
		return r.listarConsentimientos(ctx, `TRUE`)
	}
	return r.listarConsentimientos(ctx, `c.estado=$1`, estado)
}

// ConsentimientoPorID incluye la imagen de la firma (bytes) para el detalle.
func (r *Postgres) ConsentimientoPorID(ctx context.Context, id string) (*models.Consentimiento, error) {
	c, err := scanConsentimiento(r.pool.QueryRow(ctx, `SELECT `+colsConsentimiento+fromConsentimiento+` WHERE c.id=$1`, id))
	if err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT firma_imagen FROM consentimientos WHERE id=$1`, id).Scan(&c.FirmaImagenRaw); err != nil {
		return nil, traducir(err)
	}
	return c, nil
}

type NuevoConsentimiento struct {
	ID, PacienteID, PlantillaID, OdontologoID, Texto, HashDocumento string
	ProcedimientoID                                                 *string
	Relacion, FirmanteNombre, FirmanteDocumento, Metodo, Estado     string
	FirmaImagen                                                     []byte
	FirmaImagenSHA256                                               *string
	OTPHash, OTPCanal, OTPDestinatario, OTPProveedor, OTPIDProv     *string
	OTPSimulado                                                     *bool
	OTPExpira, FirmadoEn                                            *time.Time
	HashFirma                                                       *string
}

func (r *Postgres) CrearConsentimiento(ctx context.Context, n NuevoConsentimiento) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `INSERT INTO consentimientos (id, paciente_id, plantilla_id, procedimiento_id, odontologo_id, texto, hash_documento,
		firmante_relacion, firmante_nombre, firmante_documento, metodo, estado, firma_imagen, firma_imagen_sha256, otp_hash, otp_expira,
		otp_canal, otp_destinatario, otp_proveedor, otp_simulado, otp_id_proveedor, firmado_en, hash_firma)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23) RETURNING id`,
		n.ID, n.PacienteID, n.PlantillaID, n.ProcedimientoID, n.OdontologoID, n.Texto, n.HashDocumento, n.Relacion, n.FirmanteNombre,
		n.FirmanteDocumento, n.Metodo, n.Estado, n.FirmaImagen, n.FirmaImagenSHA256, n.OTPHash, n.OTPExpira, n.OTPCanal, n.OTPDestinatario,
		n.OTPProveedor, n.OTPSimulado, n.OTPIDProv, n.FirmadoEn, n.HashFirma).Scan(&id)
	return id, traducir(err)
}

func (r *Postgres) SumarIntentoOTP(ctx context.Context, id string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `UPDATE consentimientos SET otp_intentos = otp_intentos + 1 WHERE id=$1 AND estado='PENDIENTE_OTP'
		RETURNING otp_intentos`, id).Scan(&n)
	return n, traducir(err)
}

func (r *Postgres) FirmarConsentimiento(ctx context.Context, id string, firmado time.Time, hash string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE consentimientos SET estado='FIRMADO', firmado_en=$2, hash_firma=$3 WHERE id=$1 AND estado='PENDIENTE_OTP'`,
		id, firmado, hash)
	if err != nil {
		return traducir(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflicto
	}
	return nil
}

// ---------------------------------------------------------------- Recetas (P-01)

func (r *Postgres) CrearReceta(ctx context.Context, pacienteID, odontologoID string, indicaciones *string, alertas []domain.AlertaReceta,
	items []models.RecetaItem, emitida time.Time, hash string) (string, error) {
	al, _ := json.Marshal(alertas)
	var id string
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO recetas (paciente_id, odontologo_id, indicaciones, alertas_confirmadas, emitida_en, hash_firma)
			VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, pacienteID, odontologoID, indicaciones, al, emitida, hash).Scan(&id); err != nil {
			return err
		}
		for i, it := range items {
			if _, err := tx.Exec(ctx, `INSERT INTO receta_items (receta_id, orden, medicamento, presentacion, dosis, via, frecuencia, duracion, indicaciones)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, i+1, it.Medicamento, it.Presentacion, it.Dosis, it.Via, it.Frecuencia, it.Duracion, it.Indicaciones); err != nil {
				return err
			}
		}
		return nil
	})
	return id, traducir(err)
}

func (r *Postgres) RecetasPaciente(ctx context.Context, pacienteID string) ([]models.Receta, error) {
	rows, err := r.pool.Query(ctx, `SELECT r.id, r.paciente_id, r.odontologo_id, u.nombre, u.registro_profesional, r.indicaciones, r.alertas_confirmadas,
		r.emitida_en, r.hash_firma FROM recetas r JOIN usuarios u ON u.id = r.odontologo_id WHERE r.paciente_id=$1 ORDER BY r.emitida_en DESC`, pacienteID)
	if err != nil {
		return nil, err
	}
	out := []models.Receta{}
	idx := map[string]int{}
	for rows.Next() {
		var x models.Receta
		var al []byte
		if err := rows.Scan(&x.ID, &x.PacienteID, &x.OdontologoID, &x.Odontologo, &x.RegistroProfesional, &x.Indicaciones, &al, &x.EmitidaEn, &x.HashFirma); err != nil {
			rows.Close()
			return nil, err
		}
		x.AlertasConfirmadas = []domain.AlertaReceta{}
		_ = json.Unmarshal(al, &x.AlertasConfirmadas)
		x.Items = []models.RecetaItem{}
		idx[x.ID] = len(out)
		out = append(out, x)
	}
	rows.Close()
	rows, err = r.pool.Query(ctx, `SELECT i.receta_id::text, i.medicamento, i.presentacion, i.dosis, i.via, i.frecuencia, i.duracion, coalesce(i.indicaciones,'')
		FROM receta_items i JOIN recetas r ON r.id = i.receta_id WHERE r.paciente_id=$1 ORDER BY i.receta_id, i.orden`, pacienteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rid string
		var it models.RecetaItem
		if err := rows.Scan(&rid, &it.Medicamento, &it.Presentacion, &it.Dosis, &it.Via, &it.Frecuencia, &it.Duracion, &it.Indicaciones); err != nil {
			return nil, err
		}
		out[idx[rid]].Items = append(out[idx[rid]].Items, it)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- Archivos del paciente

func (r *Postgres) RegistrarArchivo(ctx context.Context, a models.Archivo, usuarioID string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `INSERT INTO archivos_paciente (id, paciente_id, categoria, nombre, tipo_mime, tamano, sha256, clave, descripcion, subido_por)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, a.ID, a.PacienteID, a.Categoria, a.Nombre, a.TipoMime, a.Tamano, a.SHA256, a.Clave,
		a.Descripcion, usuarioID).Scan(&id)
	return id, traducir(err)
}

const colsArchivo = `a.id, a.paciente_id, a.categoria, a.nombre, a.tipo_mime, a.tamano, a.sha256, a.clave, a.descripcion, u.nombre, a.subido_en`

func scanArchivo(row pgx.Row) (*models.Archivo, error) {
	a := &models.Archivo{}
	if err := row.Scan(&a.ID, &a.PacienteID, &a.Categoria, &a.Nombre, &a.TipoMime, &a.Tamano, &a.SHA256, &a.Clave, &a.Descripcion, &a.SubidoPor, &a.SubidoEn); err != nil {
		return nil, traducir(err)
	}
	return a, nil
}

func (r *Postgres) ArchivosPaciente(ctx context.Context, pacienteID string) ([]models.Archivo, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+colsArchivo+` FROM archivos_paciente a JOIN usuarios u ON u.id = a.subido_por
		WHERE a.paciente_id=$1 ORDER BY a.subido_en DESC`, pacienteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Archivo{}
	for rows.Next() {
		a, err := scanArchivo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (r *Postgres) ArchivoPorID(ctx context.Context, id string) (*models.Archivo, error) {
	return scanArchivo(r.pool.QueryRow(ctx, `SELECT `+colsArchivo+` FROM archivos_paciente a JOIN usuarios u ON u.id = a.subido_por WHERE a.id=$1`, id))
}

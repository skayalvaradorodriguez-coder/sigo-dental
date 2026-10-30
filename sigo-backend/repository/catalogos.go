package repository

import (
	"context"
	"fmt"
	"strings"

	"sigo-backend/models"
)

// Estructura sede → consultorio → sillón. soloActivos=true para formularios y agenda.
func (r *Postgres) EstructuraSedes(ctx context.Context, soloActivos bool) ([]models.Sede, error) {
	filtro := ""
	if soloActivos {
		filtro = " WHERE activo"
	}
	sedes := []models.Sede{}
	rows, err := r.pool.Query(ctx, `SELECT id, nombre, direccion, activo FROM sedes`+filtro+` ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s models.Sede
		if err := rows.Scan(&s.ID, &s.Nombre, &s.Direccion, &s.Activo); err != nil {
			rows.Close()
			return nil, err
		}
		s.Consultorios = []models.Consultorio{}
		sedes = append(sedes, s)
	}
	rows.Close()
	cons := map[string][]models.Consultorio{}
	rows, err = r.pool.Query(ctx, `SELECT id, sede_id, nombre, activo FROM consultorios`+filtro+` ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c models.Consultorio
		if err := rows.Scan(&c.ID, &c.SedeID, &c.Nombre, &c.Activo); err != nil {
			rows.Close()
			return nil, err
		}
		c.Sillones = []models.Sillon{}
		cons[c.SedeID] = append(cons[c.SedeID], c)
	}
	rows.Close()
	sill := map[string][]models.Sillon{}
	rows, err = r.pool.Query(ctx, `SELECT id, consultorio_id, nombre, activo FROM sillones`+filtro+` ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s models.Sillon
		if err := rows.Scan(&s.ID, &s.ConsultorioID, &s.Nombre, &s.Activo); err != nil {
			rows.Close()
			return nil, err
		}
		sill[s.ConsultorioID] = append(sill[s.ConsultorioID], s)
	}
	rows.Close()
	for i := range sedes {
		for _, c := range cons[sedes[i].ID] {
			if ss, ok := sill[c.ID]; ok {
				c.Sillones = ss
			}
			sedes[i].Consultorios = append(sedes[i].Consultorios, c)
		}
	}
	return sedes, nil
}

// SillonesActivos: sillones activos con consultorio y sede activos (para la agenda).
func (r *Postgres) SillonesActivos(ctx context.Context) ([]models.Sillon, error) {
	rows, err := r.pool.Query(ctx, `SELECT s.id, s.consultorio_id, s.nombre, s.activo, c.nombre, se.id, se.nombre
		FROM sillones s JOIN consultorios c ON c.id = s.consultorio_id JOIN sedes se ON se.id = c.sede_id
		WHERE s.activo AND c.activo AND se.activo ORDER BY se.nombre, c.nombre, s.nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Sillon{}
	for rows.Next() {
		var s models.Sillon
		if err := rows.Scan(&s.ID, &s.ConsultorioID, &s.Nombre, &s.Activo, &s.Consultorio, &s.SedeID, &s.Sede); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Postgres) SillonActivo(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sillones s JOIN consultorios c ON c.id=s.consultorio_id
		JOIN sedes se ON se.id=c.sede_id WHERE s.id=$1 AND s.activo AND c.activo AND se.activo)`, id).Scan(&ok)
	return ok, err
}

func (r *Postgres) Procedimientos(ctx context.Context, soloActivos bool) ([]models.Procedimiento, error) {
	q := `SELECT id, codigo, nombre, duracion_min, arancel::text, activo FROM procedimientos`
	if soloActivos {
		q += ` WHERE activo`
	}
	rows, err := r.pool.Query(ctx, q+` ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Procedimiento{}
	for rows.Next() {
		var p models.Procedimiento
		if err := rows.Scan(&p.ID, &p.Codigo, &p.Nombre, &p.DuracionMin, &p.Arancel, &p.Activo); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Postgres) Convenios(ctx context.Context, soloActivos bool) ([]models.Convenio, error) {
	q := `SELECT id, nombre, aseguradora, activo FROM convenios`
	if soloActivos {
		q += ` WHERE activo`
	}
	rows, err := r.pool.Query(ctx, q+` ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Convenio{}
	for rows.Next() {
		var c models.Convenio
		if err := rows.Scan(&c.ID, &c.Nombre, &c.Aseguradora, &c.Activo); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Postgres) Odontologos(ctx context.Context) ([]models.Odontologo, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, nombre, registro_profesional FROM usuarios WHERE rol='ODONTOLOGO' AND activo ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Odontologo{}
	for rows.Next() {
		var o models.Odontologo
		if err := rows.Scan(&o.ID, &o.Nombre, &o.RegistroProfesional); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *Postgres) EsOdontologoActivo(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM usuarios WHERE id=$1 AND rol='ODONTOLOGO' AND activo)`, id).Scan(&ok)
	return ok, err
}

func (r *Postgres) ProcedimientoActivo(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM procedimientos WHERE id=$1 AND activo)`, id).Scan(&ok)
	return ok, err
}

// TablaCatalogo describe qué columnas acepta cada catálogo editable (lista blanca: evita SQL dinámico inseguro).
type TablaCatalogo struct {
	Tabla    string
	Columnas map[string]bool
}

var Catalogos = map[string]TablaCatalogo{
	"sedes":          {"sedes", map[string]bool{"nombre": true, "direccion": true, "activo": true}},
	"consultorios":   {"consultorios", map[string]bool{"sede_id": true, "nombre": true, "activo": true}},
	"sillones":       {"sillones", map[string]bool{"consultorio_id": true, "nombre": true, "activo": true}},
	"procedimientos": {"procedimientos", map[string]bool{"codigo": true, "nombre": true, "duracion_min": true, "arancel": true, "activo": true}},
	"convenios":      {"convenios", map[string]bool{"nombre": true, "aseguradora": true, "activo": true}},
}

// CrearCatalogo / ActualizarCatalogo reciben columnas ya validadas por el controlador.
func (r *Postgres) CrearCatalogo(ctx context.Context, cat string, valores map[string]any) (map[string]any, error) {
	t, ok := Catalogos[cat]
	if !ok {
		return nil, ErrNoEncontrado
	}
	cols, phs, args := []string{}, []string{}, []any{}
	for c, v := range valores {
		if !t.Columnas[c] {
			return nil, fmt.Errorf("columna no permitida: %s", c)
		}
		args = append(args, v)
		cols = append(cols, c)
		phs = append(phs, fmt.Sprintf("$%d", len(args)))
	}
	q := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s) RETURNING row_to_json(%s.*)`, t.Tabla, strings.Join(cols, ","), strings.Join(phs, ","), t.Tabla)
	var fila map[string]any
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&fila); err != nil {
		return nil, traducir(err)
	}
	return fila, nil
}

func (r *Postgres) ActualizarCatalogo(ctx context.Context, cat, id string, valores map[string]any) (antes, despues map[string]any, err error) {
	t, ok := Catalogos[cat]
	if !ok {
		return nil, nil, ErrNoEncontrado
	}
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT row_to_json(x.*) FROM %s x WHERE id=$1`, t.Tabla), id).Scan(&antes); err != nil {
		return nil, nil, traducir(err)
	}
	if len(valores) == 0 {
		return antes, antes, nil
	}
	sets, args := []string{}, []any{id}
	for c, v := range valores {
		if !t.Columnas[c] {
			return nil, nil, fmt.Errorf("columna no permitida: %s", c)
		}
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s=$%d", c, len(args)))
	}
	q := fmt.Sprintf(`UPDATE %s SET %s WHERE id=$1 RETURNING row_to_json(%s.*)`, t.Tabla, strings.Join(sets, ","), t.Tabla)
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&despues); err != nil {
		return nil, nil, traducir(err)
	}
	return antes, despues, nil
}

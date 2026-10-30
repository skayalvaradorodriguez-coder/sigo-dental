package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"sigo-backend/models"
)

// HashGenesis es el hash "anterior" de la primera fila de la cadena.
var HashGenesis = strings.Repeat("0", 64)

// Evento a registrar en la bitácora.
type Evento struct {
	Accion    string
	Entidad   string
	EntidadID string
	UsuarioID string
	IP        string
	UserAgent string
	Detalle   map[string]any
}

// JSONCanonico: claves ordenadas, para que el mismo contenido produzca siempre el mismo hash
// (PostgreSQL reordena las claves de jsonb).
func JSONCanonico(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case map[string]any:
		claves := make([]string, 0, len(x))
		for k := range x {
			claves = append(claves, k)
		}
		sort.Strings(claves)
		partes := make([]string, 0, len(claves))
		for _, k := range claves {
			kb, _ := json.Marshal(k)
			partes = append(partes, string(kb)+":"+JSONCanonico(x[k]))
		}
		return "{" + strings.Join(partes, ",") + "}"
	case []any:
		partes := make([]string, len(x))
		for i, e := range x {
			partes[i] = JSONCanonico(e)
		}
		return "[" + strings.Join(partes, ",") + "]"
	default:
		// Normaliza números/tipos pasando por JSON (igual que al leer de jsonb).
		b, _ := json.Marshal(x)
		var g any
		if json.Unmarshal(b, &g) == nil {
			if _, esMapa := g.(map[string]any); esMapa {
				return JSONCanonico(g)
			}
			if _, esLista := g.([]any); esLista {
				return JSONCanonico(g)
			}
			b, _ = json.Marshal(g)
		}
		return string(b)
	}
}

func nulo(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// HashAuditoria: hash_n = SHA-256(hash_{n-1} | contenido_n).
func HashAuditoria(anterior string, ocurrido time.Time, usuarioID, entidadID, ip, ua *string, accion, entidad string, detalle map[string]any) string {
	var det any
	if detalle != nil {
		det = detalle
	}
	contenido := JSONCanonico(map[string]any{
		"ocurridoEn": ocurrido.UTC().Format("2006-01-02T15:04:05.000000Z"),
		"usuarioId":  nulo(usuarioID),
		"accion":     accion,
		"entidad":    entidad,
		"entidadId":  nulo(entidadID),
		"ip":         nulo(ip),
		"userAgent":  nulo(ua),
		"detalle":    det,
	})
	h := sha256.Sum256([]byte(anterior + "|" + contenido))
	return hex.EncodeToString(h[:])
}

const llaveAuditoria = 74311001 // bloqueo consultivo que serializa la cadena

func opt(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// RegistrarAuditoria inserta un evento encadenado. Usa un bloqueo consultivo para que dos
// inserciones simultáneas no tomen el mismo hash anterior.
func (r *Postgres) RegistrarAuditoria(ctx context.Context, e Evento) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, llaveAuditoria); err != nil {
		return err
	}
	anterior := HashGenesis
	if err := tx.QueryRow(ctx, `SELECT hash FROM auditoria ORDER BY id DESC LIMIT 1`).Scan(&anterior); err != nil && err != pgx.ErrNoRows {
		return err
	}
	ahora := time.Now().UTC().Truncate(time.Microsecond)
	uid, eid, ip, ua := opt(e.UsuarioID), opt(e.EntidadID), opt(e.IP), opt(e.UserAgent)
	if len(e.UserAgent) > 300 {
		ua = opt(e.UserAgent[:300])
	}
	// Normalizar el detalle como quedará en jsonb.
	var detalle map[string]any
	if e.Detalle != nil {
		b, _ := json.Marshal(e.Detalle)
		_ = json.Unmarshal(b, &detalle)
	}
	hash := HashAuditoria(anterior, ahora, uid, eid, ip, ua, e.Accion, e.Entidad, detalle)
	if _, err := tx.Exec(ctx, `INSERT INTO auditoria (ocurrido_en, usuario_id, accion, entidad, entidad_id, ip, user_agent, detalle, hash_anterior, hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, ahora, uid, e.Accion, e.Entidad, eid, ip, ua, detalle, anterior, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type FiltroAuditoria struct {
	Accion, Entidad, EntidadID, UsuarioID string
	Desde, Hasta                          *time.Time
	Page, Limit                           int
}

func (r *Postgres) ListarAuditoria(ctx context.Context, f FiltroAuditoria) (models.Pagina[models.EventoAuditoria], error) {
	page, limit, off := paginar(f.Page, f.Limit)
	conds := []string{"TRUE"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.Accion != "" {
		add("a.accion = $%d", f.Accion)
	}
	if f.Entidad != "" {
		add("a.entidad = $%d", f.Entidad)
	}
	if f.EntidadID != "" {
		add("a.entidad_id = $%d", f.EntidadID)
	}
	if f.UsuarioID != "" {
		add("a.usuario_id = $%d", f.UsuarioID)
	}
	if f.Desde != nil {
		add("a.ocurrido_en >= $%d", *f.Desde)
	}
	if f.Hasta != nil {
		add("a.ocurrido_en <= $%d", *f.Hasta)
	}
	where := strings.Join(conds, " AND ")
	res := models.Pagina[models.EventoAuditoria]{Data: []models.EventoAuditoria{}, Page: page, Limit: limit}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM auditoria a WHERE `+where, args...).Scan(&res.Total); err != nil {
		return res, err
	}
	rows, err := r.pool.Query(ctx, `SELECT a.id, a.ocurrido_en, a.usuario_id, u.nombre, u.email, a.accion, a.entidad, a.entidad_id,
		a.ip, a.user_agent, a.detalle FROM auditoria a LEFT JOIN usuarios u ON u.id = a.usuario_id
		WHERE `+where+fmt.Sprintf(` ORDER BY a.id DESC LIMIT %d OFFSET %d`, limit, off), args...)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var e models.EventoAuditoria
		if err := rows.Scan(&e.ID, &e.OcurridoEn, &e.UsuarioID, &e.Usuario, &e.Email, &e.Accion, &e.Entidad, &e.EntidadID, &e.IP, &e.UserAgent, &e.Detalle); err != nil {
			return res, err
		}
		res.Data = append(res.Data, e)
	}
	return res, rows.Err()
}

type ResultadoVerificacion struct {
	Valida      bool   `json:"valida"`
	Verificados int    `json:"verificados"`
	RotaEnID    *int64 `json:"rotaEnId"`
}

// VerificarAuditoria recorre la cadena y devuelve la primera fila alterada, si existe.
func (r *Postgres) VerificarAuditoria(ctx context.Context) (ResultadoVerificacion, error) {
	res := ResultadoVerificacion{Valida: true}
	anterior := HashGenesis
	var ultimo int64
	for {
		rows, err := r.pool.Query(ctx, `SELECT id, ocurrido_en, usuario_id::text, accion, entidad, entidad_id, ip, user_agent, detalle, hash_anterior, hash
			FROM auditoria WHERE id > $1 ORDER BY id LIMIT 1000`, ultimo)
		if err != nil {
			return res, err
		}
		n := 0
		for rows.Next() {
			n++
			var id int64
			var oc time.Time
			var uid, eid, ip, ua *string
			var accion, entidad, prev, hash string
			var det map[string]any
			if err := rows.Scan(&id, &oc, &uid, &accion, &entidad, &eid, &ip, &ua, &det, &prev, &hash); err != nil {
				rows.Close()
				return res, err
			}
			if prev != anterior || HashAuditoria(prev, oc, uid, eid, ip, ua, accion, entidad, det) != hash {
				rows.Close()
				res.Valida = false
				res.RotaEnID = &id
				return res, nil
			}
			anterior, ultimo = hash, id
			res.Verificados++
		}
		rows.Close()
		if n == 0 {
			return res, nil
		}
	}
}

func (r *Postgres) ContarEventosSeguridad(ctx context.Context, desde time.Time) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT accion, count(*) FROM auditoria WHERE ocurrido_en > $1
		AND accion IN ('LOGIN_FALLIDO','CUENTA_BLOQUEADA','MFA_FALLIDO','SESION_EXPIRADA') GROUP BY accion`, desde)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var a string
		var n int
		if err := rows.Scan(&a, &n); err != nil {
			return nil, err
		}
		out[a] = n
	}
	return out, rows.Err()
}

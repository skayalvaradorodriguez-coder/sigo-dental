// Package pruebas: pruebas de integración de la API completa (Fiber app.Test) contra una
// base PostgreSQL REAL de pruebas (las reglas críticas viven en la BD: EXCLUDE, triggers).
// Uso: docker compose up -d && go test ./pruebas/ -v
// Si la base de pruebas no responde, estas pruebas se omiten (las unitarias siguen corriendo).
package pruebas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"sigo-backend/almacenamiento"
	"sigo-backend/facturacion"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"sigo-backend/cache"
	"sigo-backend/config"
	"sigo-backend/controllers"
	"sigo-backend/database"
	"sigo-backend/notificaciones"
	"sigo-backend/repository"
	"sigo-backend/routes"
	"sigo-backend/seguridad"
	"sigo-backend/semilla"
)

var (
	app        *fiber.App
	registro   []routes.Registro
	pool       *pgxpool.Pool
	ctl        *controllers.Controllers
	almacen    = &almacenamiento.Memoria{}
	facturador = &facturacion.Simulado{}
	simulada   *notificaciones.Simulada
	saltar     string
)

const pass = semilla.PasswordDemo

func TestMain(m *testing.M) {
	os.Setenv("APP_ENV", "test")
	cfg, err := config.Cargar("../.env.test")
	if err != nil {
		fmt.Println("configuración de pruebas inválida:", err)
		os.Exit(1)
	}
	if !strings.Contains(cfg.DatabaseURL, "_test") {
		fmt.Println("Por seguridad, las pruebas sólo corren sobre una base *_test")
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err = config.ConectarDB(ctx, cfg.DatabaseURL)
	if err != nil {
		saltar = "base de pruebas no disponible: " + err.Error()
		os.Exit(m.Run())
	}
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		fmt.Println("no se pudo limpiar la base de pruebas:", err)
		os.Exit(1)
	}
	if _, err := database.Migrar(ctx, pool); err != nil {
		fmt.Println("migración:", err)
		os.Exit(1)
	}
	if err := semilla.Cargar(ctx, pool, false, cfg.ZonaHoraria); err != nil {
		fmt.Println("semilla:", err)
		os.Exit(1)
	}
	simulada = &notificaciones.Simulada{}
	desp := &notificaciones.Despachador{Proveedor: simulada, Clinica: cfg.NombreClinica, Zona: cfg.ZonaHoraria}
	ctl = controllers.NewControllers(repository.NuevoPostgres(pool), cache.NuevaRedisCache(config.ConectarRedis(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)), cfg, desp)
	ctl.Archivos = almacen
	ctl.Facturacion = facturador
	app, registro = routes.NuevaApp(ctl)
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

func requiereBD(t *testing.T) {
	t.Helper()
	if saltar != "" {
		t.Skip(saltar)
	}
}

type resp struct {
	Status int
	Body   map[string]any
	Lista  []any
	Header map[string]string
}

func llamar(t *testing.T, metodo, ruta, token string, cuerpo any) resp {
	t.Helper()
	var r io.Reader
	if cuerpo != nil {
		if s, ok := cuerpo.(string); ok {
			r = strings.NewReader(s)
		} else {
			b, _ := json.Marshal(cuerpo)
			r = bytes.NewReader(b)
		}
	}
	req := httptest.NewRequest(metodo, ruta, r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "go-test")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", metodo, ruta, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	out := resp{Status: res.StatusCode, Header: map[string]string{}}
	for k, v := range res.Header {
		out.Header[k] = strings.Join(v, ",")
	}
	if len(b) > 0 && b[0] == '[' {
		_ = json.Unmarshal(b, &out.Lista)
	} else {
		_ = json.Unmarshal(b, &out.Body)
	}
	return out
}

func espera(t *testing.T, r resp, status int, desc string) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("%s: se esperaba %d y llegó %d %v", desc, status, r.Status, r.Body)
	}
}

// codigoVigente devuelve el TOTP actual del usuario y reinicia el anti-replay (para poder
// iniciar sesión varias veces dentro del mismo intervalo de 30 s en las pruebas).
func codigoVigente(t *testing.T, email string) string {
	t.Helper()
	var sec *string
	if err := pool.QueryRow(context.Background(), `UPDATE usuarios SET mfa_ultimo_paso=NULL WHERE email=$1 RETURNING mfa_secreto`, email).Scan(&sec); err != nil || sec == nil {
		t.Fatalf("sin secreto MFA para %s", email)
	}
	return seguridad.CodigoTOTP(*sec, time.Now())
}

func reiniciarEstado(email string) {
	_, _ = pool.Exec(context.Background(), `UPDATE usuarios SET intentos_fallidos=0, bloqueado_hasta=NULL, mfa_ultimo_paso=NULL WHERE email=$1`, email)
}

// ingresar: contraseña + MFA. Devuelve el token de la sesión completa.
func ingresar(t *testing.T, email string) string {
	t.Helper()
	reiniciarEstado(email)
	r := llamar(t, "POST", "/api/login", "", map[string]string{"email": email, "password": pass})
	espera(t, r, 200, "login "+email)
	v := llamar(t, "POST", "/api/mfa/verificar", r.Body["token"].(string), map[string]string{"codigo": codigoVigente(t, email)})
	espera(t, v, 200, "mfa "+email)
	return v.Body["token"].(string)
}

func ultimaAuditoria(t *testing.T, accion string) map[string]any {
	t.Helper()
	var det map[string]any
	var uid *string
	var eid *string
	err := pool.QueryRow(context.Background(), `SELECT detalle, usuario_id::text, entidad_id FROM auditoria WHERE accion=$1 ORDER BY id DESC LIMIT 1`, accion).Scan(&det, &uid, &eid)
	if err != nil {
		t.Fatalf("no hay auditoría %s: %v", accion, err)
	}
	if det == nil {
		det = map[string]any{}
	}
	if uid != nil {
		det["_usuario"] = *uid
	}
	if eid != nil {
		det["_entidad"] = *eid
	}
	return det
}

func idDe(t *testing.T, q string, args ...any) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), q, args...).Scan(&id); err != nil {
		t.Fatalf("consulta %q: %v", q, err)
	}
	return id
}

const (
	admin      = "admin@demo.sigo"
	recepcion  = "recepcion@demo.sigo"
	odontologo = "odontologo@demo.sigo"
	asistente  = "asistente@demo.sigo"
	auditor    = "auditor@demo.sigo"
)

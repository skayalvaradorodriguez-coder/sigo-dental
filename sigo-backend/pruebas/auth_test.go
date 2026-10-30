package pruebas

import (
	"context"
	"encoding/json"
	"sigo-backend/routes"
	"strings"
	"testing"
	"time"

	"sigo-backend/seguridad"
)

func TestHealthPublico(t *testing.T) {
	requiereBD(t)
	r := llamar(t, "GET", "/api/health", "", nil)
	espera(t, r, 200, "health")
	if r.Body["db"] != "ok" {
		t.Errorf("db: %v", r.Body)
	}
}

func TestSinTokenYTokenAdulterado(t *testing.T) {
	requiereBD(t)
	r := llamar(t, "GET", "/api/usuarios", "", nil)
	if r.Status != 401 || r.Body["code"] != "NO_AUTENTICADO" {
		t.Fatalf("sin token: %d %v", r.Status, r.Body)
	}
	tok := ingresar(t, admin)
	espera(t, llamar(t, "GET", "/api/usuarios", tok+"x", nil), 401, "token adulterado")
	otro, _ := seguridad.GenerarToken(strings.Repeat("z", 40), "u", "a@b.c", "ADMIN_TI", "jti-falso", time.Now().Add(time.Hour))
	espera(t, llamar(t, "GET", "/api/usuarios", otro, nil), 401, "token firmado con otra clave")
}

// Denegar por defecto: TODA ruta /api registrada en Fiber debe tener una política.
func TestTodasLasRutasTienenPolitica(t *testing.T) {
	requiereBD(t)
	conPolitica := map[string]bool{}
	for _, r := range registro {
		conPolitica[r.Metodo+" "+r.Ruta] = true
		if routes.Docs[r.Metodo+" "+r.Ruta].Resumen == "" {
			t.Errorf("ruta sin documentar en routes/openapi.go (Docs): %s %s", r.Metodo, r.Ruta)
		}
	}
	for _, rt := range app.GetRoutes(true) {
		if !strings.HasPrefix(rt.Path, "/api/") || rt.Method == "HEAD" || rt.Method == "USE" {
			continue
		}
		if !conPolitica[rt.Method+" "+rt.Path] {
			t.Errorf("ruta sin política declarada: %s %s", rt.Method, rt.Path)
		}
	}
	if len(registro) < 30 {
		t.Errorf("se esperaban al menos 30 rutas registradas, hay %d", len(registro))
	}
}

func TestValidaEntradaYRechazaCamposExtra(t *testing.T) {
	requiereBD(t)
	espera(t, llamar(t, "POST", "/api/login", "", map[string]string{"email": "no-es-correo", "password": "x"}), 400, "correo inválido")
	espera(t, llamar(t, "POST", "/api/login", "", map[string]string{"email": admin, "password": "x", "rol": "ADMIN_TI"}), 400, "campo extra")
	espera(t, llamar(t, "POST", "/api/login", "", "{malformado"), 400, "JSON inválido")
}

func TestMismoErrorSinEnumeracion(t *testing.T) {
	requiereBD(t)
	reiniciarEstado(recepcion)
	a := llamar(t, "POST", "/api/login", "", map[string]string{"email": "nadie@demo.sigo", "password": "Lo-que-sea-1"})
	b := llamar(t, "POST", "/api/login", "", map[string]string{"email": recepcion, "password": "Incorrecta-123"})
	if a.Status != 401 || b.Status != 401 || a.Body["message"] != b.Body["message"] || a.Body["code"] != b.Body["code"] {
		t.Fatalf("respuestas distintas: %v / %v", a.Body, b.Body)
	}
	if ultimaAuditoria(t, "LOGIN_FALLIDO")["motivo"] != "PASSWORD_INCORRECTO" {
		t.Error("el fallo debe quedar auditado con su motivo")
	}
	reiniciarEstado(recepcion)
}

func TestMFAObligatorioYRotacionDeToken(t *testing.T) {
	requiereBD(t)
	reiniciarEstado(admin)
	l := llamar(t, "POST", "/api/login", "", map[string]string{"email": admin, "password": pass})
	espera(t, l, 200, "login")
	if l.Body["siguientePaso"] != "VERIFICAR_MFA" {
		t.Fatalf("paso: %v", l.Body)
	}
	pre := l.Body["token"].(string)
	r := llamar(t, "GET", "/api/usuarios", pre, nil)
	if r.Status != 403 || r.Body["code"] != "MFA_REQUERIDO" {
		t.Fatalf("sin MFA: %d %v", r.Status, r.Body)
	}
	espera(t, llamar(t, "GET", "/api/panel", pre, nil), 403, "panel sin MFA")
	espera(t, llamar(t, "POST", "/api/mfa/verificar", pre, map[string]string{"codigo": "000000"}), 400, "código incorrecto")
	v := llamar(t, "POST", "/api/mfa/verificar", pre, map[string]string{"codigo": codigoVigente(t, admin)})
	espera(t, v, 200, "MFA correcto")
	if v.Body["siguientePaso"] != nil {
		t.Errorf("tras MFA no debe quedar paso pendiente: %v", v.Body["siguientePaso"])
	}
	espera(t, llamar(t, "GET", "/api/usuarios", v.Body["token"].(string), nil), 200, "con MFA")
	espera(t, llamar(t, "GET", "/api/me", pre, nil), 401, "el token previo al MFA quedó revocado")
}

func TestCodigoTOTPNoSeReutiliza(t *testing.T) {
	requiereBD(t)
	reiniciarEstado(auditor)
	codigo := codigoVigente(t, auditor)
	l1 := llamar(t, "POST", "/api/login", "", map[string]string{"email": auditor, "password": pass})
	espera(t, llamar(t, "POST", "/api/mfa/verificar", l1.Body["token"].(string), map[string]string{"codigo": codigo}), 200, "primer uso")
	l2 := llamar(t, "POST", "/api/login", "", map[string]string{"email": auditor, "password": pass})
	r := llamar(t, "POST", "/api/mfa/verificar", l2.Body["token"].(string), map[string]string{"codigo": codigo})
	if r.Status != 400 || r.Body["code"] != "CODIGO_MFA_INVALIDO" {
		t.Fatalf("reutilización: %d %v", r.Status, r.Body)
	}
}

func TestBloqueoTrasCincoIntentos(t *testing.T) {
	requiereBD(t)
	reiniciarEstado(asistente)
	for i := 0; i < 4; i++ {
		espera(t, llamar(t, "POST", "/api/login", "", map[string]string{"email": asistente, "password": "Incorrecta-123"}), 401, "intento fallido")
	}
	r := llamar(t, "POST", "/api/login", "", map[string]string{"email": asistente, "password": "Incorrecta-123"})
	if r.Status != 423 || r.Body["code"] != "CUENTA_BLOQUEADA" {
		t.Fatalf("quinto intento: %d %v", r.Status, r.Body)
	}
	espera(t, llamar(t, "POST", "/api/login", "", map[string]string{"email": asistente, "password": pass}), 423, "bloqueada aun con la clave correcta")
	ultimaAuditoria(t, "CUENTA_BLOQUEADA")
	reiniciarEstado(asistente)
}

func TestCierrePorInactividad(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, recepcion)
	espera(t, llamar(t, "GET", "/api/panel", tok, nil), 200, "activa")
	uid := idDe(t, `SELECT id FROM usuarios WHERE email=$1`, recepcion)
	_, _ = pool.Exec(context.Background(), `UPDATE sesiones SET ultima_actividad = now() - interval '16 minutes' WHERE usuario_id=$1`, uid)
	r := llamar(t, "GET", "/api/panel", tok, nil)
	if r.Status != 401 || r.Body["code"] != "SESION_EXPIRADA" {
		t.Fatalf("16 min: %d %v", r.Status, r.Body)
	}
	if ultimaAuditoria(t, "SESION_EXPIRADA")["_usuario"] != uid {
		t.Error("la expiración debe quedar auditada")
	}
	espera(t, llamar(t, "GET", "/api/me", tok, nil), 401, "la sesión quedó revocada")

	tok2 := ingresar(t, recepcion)
	_, _ = pool.Exec(context.Background(), `UPDATE sesiones SET ultima_actividad = now() - interval '14 minutes' WHERE usuario_id=$1 AND revocada_en IS NULL`, uid)
	espera(t, llamar(t, "POST", "/api/heartbeat", tok2, nil), 204, "14 min: heartbeat")
	espera(t, llamar(t, "GET", "/api/panel", tok2, nil), 200, "14 min: sigue activa")
}

func TestLogoutRevoca(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, odontologo)
	espera(t, llamar(t, "POST", "/api/logout", tok, nil), 204, "logout")
	espera(t, llamar(t, "GET", "/api/me", tok, nil), 401, "token tras logout")
	ultimaAuditoria(t, "LOGOUT")
}

func TestPrimerIngreso(t *testing.T) {
	requiereBD(t)
	email := "nuevo@demo.sigo"
	nueva := "Primer-Ingreso-2026"
	l := llamar(t, "POST", "/api/login", "", map[string]string{"email": email, "password": pass})
	espera(t, l, 200, "login")
	if l.Body["siguientePaso"] != "CAMBIAR_PASSWORD" {
		t.Fatalf("paso: %v", l.Body["siguientePaso"])
	}
	tok := l.Body["token"].(string)
	if r := llamar(t, "GET", "/api/panel", tok, nil); r.Body["code"] != "CAMBIO_PASSWORD_REQUERIDO" {
		t.Fatalf("panel: %v", r.Body)
	}
	espera(t, llamar(t, "POST", "/api/mfa/configurar", tok, nil), 403, "MFA antes de cambiar la clave")
	if r := llamar(t, "POST", "/api/password", tok, map[string]string{"actual": pass, "nueva": "corta"}); r.Body["code"] != "PASSWORD_DEBIL" {
		t.Fatalf("clave débil: %v", r.Body)
	}
	espera(t, llamar(t, "POST", "/api/password", tok, map[string]string{"actual": "mala", "nueva": nueva}), 400, "clave actual incorrecta")
	espera(t, llamar(t, "POST", "/api/password", tok, map[string]string{"actual": pass, "nueva": nueva}), 204, "cambio")
	if r := llamar(t, "GET", "/api/me", tok, nil); r.Body["siguientePaso"] != "CONFIGURAR_MFA" {
		t.Fatalf("tras cambio: %v", r.Body["siguientePaso"])
	}
	if r := llamar(t, "GET", "/api/panel", tok, nil); r.Body["code"] != "MFA_CONFIGURACION_REQUERIDA" {
		t.Fatalf("panel sin MFA: %v", r.Body)
	}
	cfg := llamar(t, "POST", "/api/mfa/configurar", tok, nil)
	espera(t, cfg, 200, "configurar")
	if !strings.HasPrefix(cfg.Body["otpauthUrl"].(string), "otpauth://totp/") {
		t.Errorf("otpauth: %v", cfg.Body)
	}
	espera(t, llamar(t, "POST", "/api/mfa/activar", tok, map[string]string{"codigo": "000000"}), 400, "código incorrecto")
	act := llamar(t, "POST", "/api/mfa/activar", tok, map[string]string{"codigo": seguridad.CodigoTOTP(cfg.Body["secreto"].(string), time.Now())})
	espera(t, act, 200, "activar")
	tok = act.Body["token"].(string)
	espera(t, llamar(t, "GET", "/api/panel", tok, nil), 200, "panel con sesión completa")
	espera(t, llamar(t, "POST", "/api/mfa/configurar", tok, nil), 409, "MFA ya activo")
}

func TestAjustes(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, admin)
	espera(t, llamar(t, "PUT", "/api/ajustes/seguridad.minutosInactividad", tok, map[string]int{"valor": 30}), 400, "fuera de rango")
	espera(t, llamar(t, "PUT", "/api/ajustes/no.existe", tok, map[string]int{"valor": 1}), 400, "desconocido")
	espera(t, llamar(t, "PUT", "/api/ajustes/seguridad.minutosInactividad", tok, map[string]int{"valor": 10}), 200, "válido")
	if r := llamar(t, "GET", "/api/me", tok, nil); r.Body["sesion"].(map[string]any)["minutosInactividad"].(float64) != 10 {
		t.Errorf("no se aplicó: %v", r.Body["sesion"])
	}
	if d := ultimaAuditoria(t, "MODIFICACION"); d["anterior"].(float64) != 15 || d["valor"].(float64) != 10 {
		t.Errorf("auditoría: %v", d)
	}
	espera(t, llamar(t, "PUT", "/api/ajustes/seguridad.minutosInactividad", tok, map[string]int{"valor": 15}), 200, "restaurar")
	r := llamar(t, "GET", "/api/ajustes", tok, nil)
	for _, a := range r.Lista {
		m := a.(map[string]any)
		if m["clave"] == "clinica.horasBloqueoNotas" && m["valor"].(float64) != 24 {
			t.Errorf("bloqueo de notas por defecto debe ser 24 h: %v", m)
		}
	}
}

func TestGestionUsuarios(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, admin)
	c := llamar(t, "POST", "/api/usuarios", tok, map[string]string{"email": "Lucia.Paredes@Demo.Sigo", "nombre": "Lucía Paredes (ficticio)", "rol": "ASISTENTE"})
	espera(t, c, 201, "crear")
	u := c.Body["usuario"].(map[string]any)
	if u["email"] != "lucia.paredes@demo.sigo" || len(c.Body["passwordTemporal"].(string)) < 10 {
		t.Errorf("creación: %v", c.Body)
	}
	if _, ok := u["passwordHash"]; ok {
		t.Error("nunca se devuelve el hash")
	}
	espera(t, llamar(t, "POST", "/api/usuarios", tok, map[string]string{"email": "lucia.paredes@demo.sigo", "nombre": "Duplicado", "rol": "ASISTENTE"}), 409, "duplicado")
	espera(t, llamar(t, "POST", "/api/usuarios", tok, map[string]string{"email": "sinreg@demo.sigo", "nombre": "Sin Registro", "rol": "ODONTOLOGO"}), 400, "odontólogo sin registro")
	espera(t, llamar(t, "POST", "/api/usuarios", tok, map[string]string{"email": "x@demo.sigo", "nombre": "Rol malo", "rol": "SUPERUSER"}), 400, "rol inválido")
	yo := idDe(t, `SELECT id FROM usuarios WHERE email=$1`, admin)
	espera(t, llamar(t, "PATCH", "/api/usuarios/"+yo, tok, map[string]bool{"activo": false}), 400, "no puede desactivarse a sí mismo")

	// Cambio de rol: auditado y cierra sesiones.
	id := u["id"].(string)
	p := llamar(t, "PATCH", "/api/usuarios/"+id, tok, map[string]string{"rol": "RECEPCIONISTA"})
	espera(t, p, 200, "cambio de rol")
	if d := ultimaAuditoria(t, "MODIFICACION"); d["cambios"].(map[string]any)["rol"] == nil {
		t.Errorf("cambio de rol no auditado: %v", d)
	}
	espera(t, llamar(t, "PATCH", "/api/usuarios/00000000-0000-0000-0000-000000000000", tok, map[string]string{"nombre": "Nadie"}), 404, "inexistente")
}

func TestDesactivarCierraSesiones(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, admin)
	c := llamar(t, "POST", "/api/usuarios", tok, map[string]string{"email": "temporal@demo.sigo", "nombre": "Usuario Temporal (ficticio)", "rol": "AUDITOR"})
	id := c.Body["usuario"].(map[string]any)["id"].(string)
	sec := seguridad.GenerarSecretoTOTP()
	_, _ = pool.Exec(context.Background(), `UPDATE usuarios SET mfa_activo=true, mfa_secreto=$2, debe_cambiar_password=false WHERE id=$1`, id, sec)
	l := llamar(t, "POST", "/api/login", "", map[string]string{"email": "temporal@demo.sigo", "password": c.Body["passwordTemporal"].(string)})
	v := llamar(t, "POST", "/api/mfa/verificar", l.Body["token"].(string), map[string]string{"codigo": seguridad.CodigoTOTP(sec, time.Now())})
	victima := v.Body["token"].(string)
	espera(t, llamar(t, "GET", "/api/panel", victima, nil), 200, "víctima activa")
	espera(t, llamar(t, "PATCH", "/api/usuarios/"+id, tok, map[string]bool{"activo": false}), 200, "desactivar")
	espera(t, llamar(t, "GET", "/api/panel", victima, nil), 401, "sesión cerrada al desactivar")
}

func TestRestablecimientos(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, admin)
	victima := ingresar(t, odontologo)
	id := idDe(t, `SELECT id FROM usuarios WHERE email=$1`, odontologo)
	r := llamar(t, "POST", "/api/usuarios/"+id+"/restablecer-password", tok, nil)
	espera(t, r, 200, "restablecer clave")
	espera(t, llamar(t, "GET", "/api/panel", victima, nil), 401, "sesiones cerradas")
	temp := r.Body["passwordTemporal"].(string)
	reiniciarEstado(odontologo)
	l := llamar(t, "POST", "/api/login", "", map[string]string{"email": odontologo, "password": temp})
	v := llamar(t, "POST", "/api/mfa/verificar", l.Body["token"].(string), map[string]string{"codigo": codigoVigente(t, odontologo)})
	if v.Body["siguientePaso"] != "CAMBIAR_PASSWORD" {
		t.Fatalf("debe exigir cambio: %v", v.Body["siguientePaso"])
	}
	espera(t, llamar(t, "POST", "/api/password", v.Body["token"].(string), map[string]string{"actual": temp, "nueva": pass}), 204, "vuelve a la clave demo")

	idA := idDe(t, `SELECT id FROM usuarios WHERE email=$1`, asistente)
	espera(t, llamar(t, "POST", "/api/usuarios/"+idA+"/restablecer-mfa", tok, nil), 204, "restablecer MFA")
	reiniciarEstado(asistente)
	l2 := llamar(t, "POST", "/api/login", "", map[string]string{"email": asistente, "password": pass})
	if l2.Body["siguientePaso"] != "CONFIGURAR_MFA" {
		t.Fatalf("debe reenrolar: %v", l2.Body["siguientePaso"])
	}
	cfg := llamar(t, "POST", "/api/mfa/configurar", l2.Body["token"].(string), nil)
	espera(t, llamar(t, "POST", "/api/mfa/activar", l2.Body["token"].(string), map[string]string{"codigo": seguridad.CodigoTOTP(cfg.Body["secreto"].(string), time.Now())}), 200, "reenrolado")
}

func TestAuditoriaInmutable(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, admin)
	r := llamar(t, "GET", "/api/auditoria?accion=LOGIN_EXITOSO&limit=1", tok, nil)
	espera(t, r, 200, "listar")
	ev := r.Body["data"].([]any)[0].(map[string]any)
	if _, err := time.Parse(time.RFC3339Nano, ev["ocurridoEn"].(string)); err != nil || ev["ip"] == nil {
		t.Errorf("fecha ISO-8601 e IP obligatorias: %v", ev)
	}
	ctx := context.Background()
	for _, q := range []string{`UPDATE auditoria SET accion='X'`, `DELETE FROM auditoria`, `TRUNCATE auditoria`} {
		if _, err := pool.Exec(ctx, q); err == nil || !strings.Contains(err.Error(), "inmutable") {
			t.Errorf("%s debió fallar: %v", q, err)
		}
	}
	v := llamar(t, "POST", "/api/auditoria/verificar", tok, nil)
	if v.Body["valida"] != true {
		t.Fatalf("cadena inicial: %v", v.Body)
	}
	// Alguien con privilegios de dueño desactiva el trigger y altera una fila: la cadena lo detecta.
	var id int64
	var ip *string
	_ = pool.QueryRow(ctx, `SELECT id, ip FROM auditoria ORDER BY id LIMIT 1`).Scan(&id, &ip)
	_, _ = pool.Exec(ctx, `ALTER TABLE auditoria DISABLE TRIGGER auditoria_sin_update`)
	_, _ = pool.Exec(ctx, `UPDATE auditoria SET ip='6.6.6.6' WHERE id=$1`, id)
	rota := llamar(t, "POST", "/api/auditoria/verificar", tok, nil)
	_, _ = pool.Exec(ctx, `UPDATE auditoria SET ip=$2 WHERE id=$1`, id, ip)
	_, _ = pool.Exec(ctx, `ALTER TABLE auditoria ENABLE TRIGGER auditoria_sin_update`)
	if rota.Body["valida"] != false || int64(rota.Body["rotaEnId"].(float64)) != id {
		t.Fatalf("debió detectar la alteración en %d: %v", id, rota.Body)
	}
	if v := llamar(t, "POST", "/api/auditoria/verificar", tok, nil); v.Body["valida"] != true {
		t.Fatalf("tras restaurar: %v", v.Body)
	}
}

// El OpenAPI se genera del registro real: cada ruta aparece con su política de acceso.
func TestOpenAPIGenerado(t *testing.T) {
	requiereBD(t)
	b, err := routes.OpenAPI(append([]routes.Registro(nil), registro...))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			Politica string `json:"x-politica"`
			Security []any  `json:"security"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Paths["/api/usuarios"]["post"].Politica != "permisos:usuarios.gestionar" {
		t.Errorf("política de POST /api/usuarios: %+v", doc.Paths["/api/usuarios"]["post"])
	}
	if doc.Paths["/api/health"]["get"].Security != nil || doc.Paths["/api/usuarios/{id}"]["patch"].Security == nil {
		t.Error("sólo las rutas públicas van sin seguridad Bearer")
	}
}

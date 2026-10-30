package pruebas

import (
	"net/url"
	"testing"
	"time"
)

// Catálogos que usa la agenda (RF-09): procedimientos, convenios y odontólogos activos.
func TestCatalogosDeLectura(t *testing.T) {
	requiereBD(t)
	rec := ingresar(t, recepcion)
	if l := llamar(t, "GET", "/api/catalogos/procedimientos", rec, nil); l.Status != 200 || len(l.Lista) < 6 {
		t.Errorf("procedimientos: %d %d", l.Status, len(l.Lista))
	}
	if l := llamar(t, "GET", "/api/catalogos/convenios", rec, nil); l.Status != 200 || len(l.Lista) < 2 {
		t.Errorf("convenios: %d %d", l.Status, len(l.Lista))
	}
	o := llamar(t, "GET", "/api/catalogos/odontologos", rec, nil)
	espera(t, o, 200, "odontólogos")
	for _, x := range o.Lista {
		if x.(map[string]any)["registroProfesional"] == nil {
			t.Errorf("todo odontólogo listado tiene registro profesional: %v", x)
		}
	}
	// Con ?todos=true sólo el administrador ve también los inactivos.
	adm := ingresar(t, admin)
	espera(t, llamar(t, "GET", "/api/catalogos/procedimientos?todos=true", adm, nil), 200, "admin: todos")
}

// Un administrador no puede desactivarse ni quitarse el rol a sí mismo (así nunca se queda la
// clínica sin administrador por error).
func TestAdministradorNoSeDesactivaASiMismo(t *testing.T) {
	requiereBD(t)
	adm := ingresar(t, admin)
	id := idDe(t, `SELECT id FROM usuarios WHERE email=$1`, admin)
	for _, cambio := range []map[string]any{{"activo": false}, {"rol": "AUDITOR"}} {
		if r := llamar(t, "PATCH", "/api/usuarios/"+id, adm, cambio); r.Status != 400 || r.Body["code"] != "VALIDACION" {
			t.Errorf("%v: %d %v", cambio, r.Status, r.Body)
		}
	}
}

// RNF-06: la bitácora se filtra por acción, fechas y usuario.
func TestFiltrosDeAuditoria(t *testing.T) {
	requiereBD(t)
	adm := ingresar(t, admin)
	r := llamar(t, "GET", "/api/auditoria?accion=LOGIN_EXITOSO&limit=5", adm, nil)
	espera(t, r, 200, "por acción")
	for _, e := range r.Body["data"].([]any) {
		if e.(map[string]any)["accion"] != "LOGIN_EXITOSO" {
			t.Errorf("filtro por acción: %v", e)
		}
	}
	desde := url.QueryEscape(time.Now().Add(time.Hour).Format(time.RFC3339))
	if r := llamar(t, "GET", "/api/auditoria?desde="+desde, adm, nil); r.Body["total"].(float64) != 0 {
		t.Errorf("nada después de una hora en el futuro: %v", r.Body["total"])
	}
	uid := idDe(t, `SELECT id FROM usuarios WHERE email=$1`, admin)
	if r := llamar(t, "GET", "/api/auditoria?usuarioId="+uid+"&limit=50", adm, nil); r.Body["total"].(float64) == 0 {
		t.Error("filtro por usuario")
	}
	espera(t, llamar(t, "GET", "/api/auditoria?usuarioId=no-es-uuid", adm, nil), 400, "usuarioId inválido")
	espera(t, llamar(t, "GET", "/api/auditoria?desde=ayer", adm, nil), 400, "fecha inválida")
}

// Una nota puede vincularse a una cita sólo si la cita es del mismo paciente.
func TestNotaVinculadaACita(t *testing.T) {
	requiereBD(t)
	odo := ingresar(t, odontologo)
	ana := idAna(t)
	propia := idDe(t, `SELECT id FROM citas WHERE paciente_id=$1 LIMIT 1`, ana)
	ajena := idDe(t, `SELECT id FROM citas WHERE paciente_id<>$1 LIMIT 1`, ana)
	n := notaCompleta()
	n["citaId"] = ajena
	if r := llamar(t, "POST", "/api/pacientes/"+ana+"/notas", odo, n); r.Body["code"] != "CITA_INVALIDA" {
		t.Errorf("cita de otro paciente: %v", r.Body)
	}
	n["citaId"] = propia
	r := llamar(t, "POST", "/api/pacientes/"+ana+"/notas", odo, n)
	espera(t, r, 201, "cita propia")
	if r.Body["citaId"] != propia {
		t.Errorf("vínculo con la cita: %v", r.Body["citaId"])
	}
	espera(t, llamar(t, "PUT", "/api/notas/"+r.Body["id"].(string), odo, n), 400, "la cita de una nota no se cambia")
}

// Rutas inexistentes: 404 en JSON con el mismo formato de error.
func TestRutaInexistente(t *testing.T) {
	requiereBD(t)
	r := llamar(t, "GET", "/api/no-existe", ingresar(t, admin), nil)
	if r.Status != 404 || r.Body["code"] != "NO_ENCONTRADO" {
		t.Errorf("404: %d %v", r.Status, r.Body)
	}
	if r := llamar(t, "GET", "/api/pacientes/no-es-uuid", ingresar(t, recepcion), nil); r.Status != 404 {
		t.Errorf("id inválido: %d", r.Status)
	}
}

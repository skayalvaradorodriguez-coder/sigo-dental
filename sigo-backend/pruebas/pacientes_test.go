package pruebas

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var secuencia atomic.Int64

func adulto(extra map[string]any) map[string]any {
	d := map[string]any{
		"tipoDocumento": "CEDULA", "numeroDocumento": fmt.Sprintf("T%d%d", time.Now().UnixNano()%1e7, secuencia.Add(1)),
		"nombres": "Prueba", "apellidos": "Paciente Adulto", "fechaNacimiento": "1990-05-14", "sexo": "FEMENINO",
		"telefono": "+000 0000 1111", "emergenciaNombre": "Contacto Prueba", "emergenciaTelefono": "+000 0000 2222",
		"canalPreferido": "WHATSAPP", "autorizaMensajes": true,
	}
	for k, v := range extra {
		d[k] = v
	}
	return d
}

var tutor = map[string]any{"nombre": "Tutor Prueba", "tipoDocumento": "CEDULA", "numeroDocumento": "TUT00001", "parentesco": "Madre", "telefono": "+000 0000 3333"}

func anamnesis(extra map[string]any) map[string]any {
	d := map[string]any{"enfermedadesSistemicas": []string{}, "alergias": []any{}, "coagulopatia": false, "anticoagulado": false}
	for k, v := range extra {
		d[k] = v
	}
	return d
}

func TestRegistroPacienteAdulto(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, recepcion)
	r := llamar(t, "POST", "/api/pacientes", tok, adulto(nil))
	espera(t, r, 201, "crear")
	if r.Body["esMenor"] != false || r.Body["anamnesisPendiente"] != true || r.Body["tutor"] != nil {
		t.Errorf("ficha: %v", r.Body)
	}
	ultimaAuditoria(t, "CREACION")
}

func TestMenorExigeTutor(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, recepcion)
	nac := fmt.Sprintf("%d-01-15", time.Now().Year()-8)
	r := llamar(t, "POST", "/api/pacientes", tok, adulto(map[string]any{"fechaNacimiento": nac}))
	if r.Status != 400 || r.Body["code"] != "TUTOR_REQUERIDO" {
		t.Fatalf("sin tutor: %d %v", r.Status, r.Body)
	}
	ok := llamar(t, "POST", "/api/pacientes", tok, adulto(map[string]any{"fechaNacimiento": nac, "tutor": tutor}))
	espera(t, ok, 201, "con tutor")
	if ok.Body["esMenor"] != true || ok.Body["tutor"].(map[string]any)["parentesco"] != "Madre" {
		t.Errorf("menor: %v", ok.Body)
	}
	// No se puede quitar el tutor mientras siga siendo menor.
	sin := adulto(map[string]any{"fechaNacimiento": nac, "quitarTutor": true, "numeroDocumento": ok.Body["numeroDocumento"]})
	espera(t, llamar(t, "PUT", "/api/pacientes/"+ok.Body["id"].(string), tok, sin), 400, "quitar tutor a un menor")
}

func TestDocumentoUnicoYValidaciones(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, recepcion)
	d := adulto(nil)
	espera(t, llamar(t, "POST", "/api/pacientes", tok, d), 201, "primero")
	d["nombres"] = "Otro"
	espera(t, llamar(t, "POST", "/api/pacientes", tok, d), 409, "documento duplicado")
	espera(t, llamar(t, "POST", "/api/pacientes", tok, adulto(map[string]any{"fechaNacimiento": "2999-01-01"})), 400, "fecha futura")
	espera(t, llamar(t, "POST", "/api/pacientes", tok, adulto(map[string]any{"telefono": "abc"})), 400, "teléfono inválido")
}

func TestModificacionAuditaValores(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, recepcion)
	p := llamar(t, "POST", "/api/pacientes", tok, adulto(nil)).Body
	cambio := adulto(map[string]any{"numeroDocumento": p["numeroDocumento"], "telefono": "+000 0000 9999"})
	espera(t, llamar(t, "PUT", "/api/pacientes/"+p["id"].(string), tok, cambio), 200, "actualizar")
	tel := ultimaAuditoria(t, "MODIFICACION")["cambios"].(map[string]any)["telefono"].(map[string]any)
	if tel["de"] != "+000 0000 1111" || tel["a"] != "+000 0000 9999" {
		t.Errorf("auditoría del cambio: %v", tel)
	}
}

func TestLecturasAuditadasYPrivilegios(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, recepcion)
	l := llamar(t, "GET", "/api/pacientes?q=Torres", tok, nil)
	espera(t, l, 200, "buscar")
	if ultimaAuditoria(t, "LECTURA")["q"] != "Torres" {
		t.Error("la búsqueda debe auditarse")
	}
	id := l.Body["data"].([]any)[0].(map[string]any)["id"].(string)
	espera(t, llamar(t, "GET", "/api/pacientes/"+id, tok, nil), 200, "ficha")
	if ultimaAuditoria(t, "LECTURA")["_entidad"] != id {
		t.Error("la consulta de la ficha debe auditarse")
	}
	for _, e := range []string{auditor, admin} {
		espera(t, llamar(t, "GET", "/api/pacientes", ingresar(t, e), nil), 403, e+" no ve pacientes")
	}
}

func TestAlertaPenicilinaCritica(t *testing.T) {
	requiereBD(t)
	tok := ingresar(t, recepcion)
	l := llamar(t, "GET", "/api/pacientes?q=DEMO000101", tok, nil)
	ana := l.Body["data"].([]any)[0].(map[string]any)
	al := ana["alertas"].([]any)[0].(map[string]any)
	if al["severidad"] != "CRITICA" || !strings.Contains(al["etiqueta"].(string), "Penicilina") {
		t.Fatalf("alerta: %v", al)
	}
	espera(t, llamar(t, "GET", "/api/pacientes/"+ana["id"].(string)+"/anamnesis", tok, nil), 403, "recepción no ve la anamnesis")
	espera(t, llamar(t, "POST", "/api/pacientes/"+ana["id"].(string)+"/anamnesis", tok, anamnesis(nil)), 403, "recepción no escribe anamnesis")
}

func TestAnamnesisVersionada(t *testing.T) {
	requiereBD(t)
	rec := ingresar(t, recepcion)
	p := llamar(t, "POST", "/api/pacientes", rec, adulto(nil)).Body
	id := p["id"].(string)
	asis := ingresar(t, asistente)
	v1 := llamar(t, "POST", "/api/pacientes/"+id+"/anamnesis", asis, anamnesis(map[string]any{"anticoagulado": true}))
	espera(t, v1, 201, "v1")
	if v1.Body["version"].(float64) != 1 || v1.Body["alertas"].([]any)[0].(map[string]any)["codigo"] != "ANTICOAGULADO" {
		t.Fatalf("v1: %v", v1.Body)
	}
	if f := llamar(t, "GET", "/api/pacientes/"+id, rec, nil).Body; f["anamnesisPendiente"] != false {
		t.Errorf("ya no está pendiente: %v", f)
	}
	odo := ingresar(t, odontologo)
	espera(t, llamar(t, "POST", "/api/pacientes/"+id+"/anamnesis", odo, anamnesis(map[string]any{"enfermedadesSistemicas": []string{"Asma"}})), 201, "v2")
	h := llamar(t, "GET", "/api/pacientes/"+id+"/anamnesis", odo, nil)
	if len(h.Lista) != 2 || h.Lista[0].(map[string]any)["version"].(float64) != 2 {
		t.Fatalf("historial: %v", h.Lista)
	}
	al := llamar(t, "GET", "/api/pacientes/"+id, rec, nil).Body["alertas"].([]any)
	if len(al) != 1 || al[0].(map[string]any)["etiqueta"] != "Asma" {
		t.Errorf("las alertas usan la versión vigente: %v", al)
	}
	espera(t, llamar(t, "POST", "/api/pacientes/"+id+"/anamnesis", asis, map[string]any{"enfermedadesSistemicas": []string{}}), 400, "apartados obligatorios")
	espera(t, llamar(t, "POST", "/api/pacientes/"+id+"/anamnesis", asis, anamnesis(map[string]any{"alergias": []any{map[string]string{"sustancia": "X", "tipo": "VENENO"}}})), 400, "tipo de alergia")
}

func TestReglasEnBaseDeDatos(t *testing.T) {
	requiereBD(t)
	ctx := context.Background()
	for q, msg := range map[string]string{
		`UPDATE anamnesis SET anticoagulado=false`: "solo insercion",
		`DELETE FROM anamnesis`:                    "solo insercion",
		`DELETE FROM pacientes`:                    "no admite borrado",
	} {
		if _, err := pool.Exec(ctx, q); err == nil || !strings.Contains(err.Error(), msg) {
			t.Errorf("%s debió fallar con %q: %v", q, msg, err)
		}
	}
}

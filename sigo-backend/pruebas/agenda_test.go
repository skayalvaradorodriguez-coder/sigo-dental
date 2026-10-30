package pruebas

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var diaPrueba atomic.Int64

func init() { diaPrueba.Store(20) }

// horario de prueba alejado de los datos demo: dentro de N días, a la hora UTC indicada.
func horario(dias int64, hora, min int) string {
	d := time.Now().UTC().AddDate(0, 0, int(dias))
	return time.Date(d.Year(), d.Month(), d.Day(), hora, min, 0, 0, time.UTC).Format(time.RFC3339)
}

type ids struct{ sillonA, sillonB, odontologo, paciente, sinAutorizacion, recepcion string }

func cargarIDs(t *testing.T) ids {
	return ids{
		sillonA:         idDe(t, `SELECT id FROM sillones WHERE nombre='Sillón 1A'`),
		sillonB:         idDe(t, `SELECT id FROM sillones WHERE nombre='Sillón 1B'`),
		odontologo:      idDe(t, `SELECT id FROM usuarios WHERE email=$1`, odontologo),
		paciente:        idDe(t, `SELECT id FROM pacientes WHERE numero_documento='DEMO000101'`),
		sinAutorizacion: idDe(t, `SELECT id FROM pacientes WHERE numero_documento='DEMO000105'`),
		recepcion:       idDe(t, `SELECT id FROM usuarios WHERE email=$1`, recepcion),
	}
}

func cita(i ids, extra map[string]any) map[string]any {
	d := map[string]any{"pacienteId": i.paciente, "odontologoId": i.odontologo, "sillonId": i.sillonA, "inicio": horario(diaPrueba.Load(), 14, 0), "duracionMin": 30}
	for k, v := range extra {
		d[k] = v
	}
	return d
}

func TestAgendarConRecordatorios(t *testing.T) {
	requiereBD(t)
	i := cargarIDs(t)
	diaPrueba.Add(1)
	tok := ingresar(t, recepcion)
	r := llamar(t, "POST", "/api/citas", tok, cita(i, nil))
	espera(t, r, 201, "agendar")
	if r.Body["estado"] != "PROGRAMADO" || r.Body["paciente"] != "Ana Lucía Torres Vera" || len(r.Body["alertasCriticas"].([]any)) != 1 {
		t.Errorf("cita: %v", r.Body)
	}
	ns := r.Body["notificaciones"].([]any)
	if len(ns) != 3 || len(r.Body["eventos"].([]any)) != 1 {
		t.Fatalf("mensajes/eventos: %v", r.Body)
	}
	for _, n := range ns {
		if n.(map[string]any)["estado"] != "PENDIENTE" {
			t.Errorf("mensaje no pendiente: %v", n)
		}
	}
}

func TestSuperposicionPorSillon(t *testing.T) {
	requiereBD(t)
	i := cargarIDs(t)
	dia := diaPrueba.Add(1)
	tok := ingresar(t, recepcion)
	jorge := idDe(t, `SELECT id FROM usuarios WHERE email='odontologo2@demo.sigo'`)
	espera(t, llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"inicio": horario(dia, 14, 0), "duracionMin": 60})), 201, "primera")
	r := llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"odontologoId": jorge, "inicio": horario(dia, 14, 30)}))
	if r.Status != 409 || r.Body["code"] != "SILLON_OCUPADO" {
		t.Fatalf("superposición con otro odontólogo: %d %v", r.Status, r.Body)
	}
	if r.Body["extra"].(map[string]any)["conflicto"].(map[string]any)["paciente"] != "Ana Lucía Torres Vera" {
		t.Errorf("debe informar la cita en conflicto: %v", r.Body)
	}
	espera(t, llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"inicio": horario(dia, 15, 0)})), 201, "consecutiva")
	espera(t, llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"sillonId": i.sillonB, "inicio": horario(dia, 14, 0)})), 201, "otro sillón")
}

func TestCanceladaLiberaSillon(t *testing.T) {
	requiereBD(t)
	i := cargarIDs(t)
	diaPrueba.Add(1)
	tok := ingresar(t, recepcion)
	a := llamar(t, "POST", "/api/citas", tok, cita(i, nil)).Body
	espera(t, llamar(t, "POST", "/api/citas", tok, cita(i, nil)), 409, "ocupado")
	espera(t, llamar(t, "POST", "/api/citas/"+a["id"].(string)+"/estado", tok, map[string]string{"estado": "CANCELADO", "nota": "Paciente reprogramará"}), 200, "cancelar")
	espera(t, llamar(t, "POST", "/api/citas", tok, cita(i, nil)), 201, "sillón liberado")
}

func TestVeinteSolicitudesSimultaneas(t *testing.T) {
	requiereBD(t)
	i := cargarIDs(t)
	dia := diaPrueba.Add(1)
	tok := ingresar(t, recepcion)
	inicio := horario(dia, 16, 0)
	var wg sync.WaitGroup
	var ok, ocupado atomic.Int32
	for n := 0; n < 20; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"sillonId": i.sillonB, "inicio": inicio}))
			switch r.Status {
			case 201:
				ok.Add(1)
			case 409:
				ocupado.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || ocupado.Load() != 19 {
		t.Fatalf("se esperaban 1 éxito y 19 rechazos; hubo %d y %d", ok.Load(), ocupado.Load())
	}
	var n int
	ini, _ := time.Parse(time.RFC3339, inicio)
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM citas WHERE sillon_id=$1 AND inicio=$2`, i.sillonB, ini).Scan(&n)
	if n != 1 {
		t.Fatalf("en la BD hay %d citas en ese horario", n)
	}
}

func TestRestriccionEnLaBaseDeDatos(t *testing.T) {
	requiereBD(t)
	i := cargarIDs(t)
	dia := diaPrueba.Add(1)
	ctx := context.Background()
	ini, _ := time.Parse(time.RFC3339, horario(dia, 18, 0))
	ins := `INSERT INTO citas (paciente_id, odontologo_id, sillon_id, inicio, fin, creado_por) VALUES ($1,$2,$3,$4,$5,$6)`
	if _, err := pool.Exec(ctx, ins, i.paciente, i.odontologo, i.sillonA, ini, ini.Add(30*time.Minute), i.recepcion); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, ins, i.paciente, i.odontologo, i.sillonA, ini.Add(10*time.Minute), ini.Add(40*time.Minute), i.recepcion)
	if err == nil || !strings.Contains(err.Error(), "23P01") {
		t.Fatalf("un INSERT directo superpuesto debe violar la restricción EXCLUDE: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM citas WHERE inicio=$1`, ini); err == nil || !strings.Contains(err.Error(), "no admite borrado") {
		t.Fatalf("las citas no se borran: %v", err)
	}
}

func TestValidacionesYPermisosAgenda(t *testing.T) {
	requiereBD(t)
	i := cargarIDs(t)
	diaPrueba.Add(1)
	tok := ingresar(t, recepcion)
	espera(t, llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"inicio": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)})), 400, "pasado")
	espera(t, llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"duracionMin": 33})), 400, "duración")
	espera(t, llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"odontologoId": i.recepcion})), 400, "no es odontólogo")
	rango := "desde=" + url.QueryEscape(horario(1, 0, 0)) + "&hasta=" + url.QueryEscape(horario(2, 0, 0))
	asis := ingresar(t, asistente)
	espera(t, llamar(t, "GET", "/api/citas?"+rango, asis, nil), 200, "asistente ve la agenda")
	espera(t, llamar(t, "POST", "/api/citas", asis, cita(i, map[string]any{"inicio": horario(200, 9, 0)})), 403, "asistente no agenda")
	espera(t, llamar(t, "GET", "/api/citas?"+rango, ingresar(t, auditor), nil), 403, "auditor no ve la agenda")
	espera(t, llamar(t, "GET", "/api/citas?desde="+url.QueryEscape(horario(1, 0, 0))+"&hasta="+url.QueryEscape(horario(60, 0, 0)), tok, nil), 400, "rango > 31 días")
}

func TestFiltrosAgenda(t *testing.T) {
	requiereBD(t)
	i := cargarIDs(t)
	dia := diaPrueba.Add(1)
	tok := ingresar(t, recepcion)
	espera(t, llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"inicio": horario(dia, 13, 0)})), 201, "A")
	espera(t, llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"sillonId": i.sillonB, "inicio": horario(dia, 13, 0)})), 201, "B")
	rango := "desde=" + url.QueryEscape(horario(dia, 0, 0)) + "&hasta=" + url.QueryEscape(horario(dia, 23, 59))
	if n := len(llamar(t, "GET", "/api/citas?"+rango, tok, nil).Lista); n != 2 {
		t.Errorf("todas: %d", n)
	}
	if n := len(llamar(t, "GET", "/api/citas?"+rango+"&sillonId="+i.sillonB, tok, nil).Lista); n != 1 {
		t.Errorf("por sillón: %d", n)
	}
	if n := len(llamar(t, "GET", "/api/citas?"+rango+"&odontologoId="+i.recepcion, tok, nil).Lista); n != 0 {
		t.Errorf("por odontólogo: %d", n)
	}
}

func TestFlujoDeEstados(t *testing.T) {
	requiereBD(t)
	i := cargarIDs(t)
	tok := ingresar(t, recepcion)
	ahora := time.Now().UTC().Truncate(5 * time.Minute)
	a := llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"sillonId": i.sillonB, "inicio": ahora.Format(time.RFC3339), "duracionMin": 10}))
	espera(t, a, 201, "cita de ahora")
	id := a.Body["id"].(string)
	for _, e := range []string{"CONFIRMADO", "EN_SALA_ESPERA", "EN_ATENCION", "ATENDIDO"} {
		espera(t, llamar(t, "POST", "/api/citas/"+id+"/estado", tok, map[string]string{"estado": e}), 200, e)
	}
	d := llamar(t, "GET", "/api/citas/"+id, tok, nil).Body
	if len(d["eventos"].([]any)) != 5 {
		t.Errorf("historial: %v", d["eventos"])
	}
	if r := llamar(t, "POST", "/api/citas/"+id+"/estado", tok, map[string]string{"estado": "PROGRAMADO"}); r.Body["code"] != "TRANSICION_INVALIDA" {
		t.Errorf("transición inválida: %v", r.Body)
	}
	diaPrueba.Add(1)
	b := llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"inicio": horario(diaPrueba.Load(), 19, 0)})).Body
	espera(t, llamar(t, "POST", "/api/citas/"+b["id"].(string)+"/estado", tok, map[string]string{"estado": "CANCELADO"}), 400, "cancelar sin motivo")
	espera(t, llamar(t, "POST", "/api/citas/"+b["id"].(string)+"/estado", tok, map[string]string{"estado": "NO_ASISTIO"}), 400, "no asistió antes de la hora")
}

func TestReprogramar(t *testing.T) {
	requiereBD(t)
	i := cargarIDs(t)
	dia := diaPrueba.Add(1)
	tok := ingresar(t, recepcion)
	a := llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"inicio": horario(dia, 9, 0)})).Body
	espera(t, llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"inicio": horario(dia, 11, 0)})), 201, "otra")
	espera(t, llamar(t, "PATCH", "/api/citas/"+a["id"].(string)+"/reprogramar", tok, map[string]any{"inicio": horario(dia, 11, 15), "duracionMin": 30}), 409, "choca")
	m := llamar(t, "PATCH", "/api/citas/"+a["id"].(string)+"/reprogramar", tok, map[string]any{"inicio": horario(dia, 12, 0), "duracionMin": 30})
	espera(t, m, 200, "reprogramar")
	can, pen := 0, 0
	for _, n := range m.Body["notificaciones"].([]any) {
		switch n.(map[string]any)["estado"] {
		case "CANCELADO":
			can++
		case "PENDIENTE":
			pen++
		}
	}
	if can != 3 || pen != 3 {
		t.Errorf("recordatorios rehechos: cancelados %d, pendientes %d", can, pen)
	}
}

func TestRecordatoriosYProveedorSimulado(t *testing.T) {
	requiereBD(t)
	i := cargarIDs(t)
	tok := ingresar(t, recepcion)
	ini := time.Now().Add(30 * time.Hour).UTC().Truncate(time.Hour)
	a := llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"sillonId": i.sillonB, "inicio": ini.Format(time.RFC3339)})).Body
	por := map[string]map[string]any{}
	for _, n := range a["notificaciones"].([]any) {
		por[n.(map[string]any)["tipo"].(string)] = n.(map[string]any)
	}
	if por["RECORDATORIO_48H"]["estado"] != "OMITIDO" || por["RECORDATORIO_24H"]["estado"] != "PENDIENTE" {
		t.Errorf("cita en 30 h: %v", por)
	}

	diaPrueba.Add(1)
	s := llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"pacienteId": i.sinAutorizacion, "inicio": horario(diaPrueba.Load(), 21, 0)})).Body
	for _, n := range s["notificaciones"].([]any) {
		if n.(map[string]any)["estado"] != "OMITIDO" {
			t.Errorf("sin autorización: %v", n)
		}
	}

	diaPrueba.Add(1)
	mantener := llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"inicio": horario(diaPrueba.Load(), 7, 0)})).Body
	anular := llamar(t, "POST", "/api/citas", tok, cita(i, map[string]any{"inicio": horario(diaPrueba.Load(), 8, 0)})).Body
	espera(t, llamar(t, "POST", "/api/citas/"+anular["id"].(string)+"/estado", tok, map[string]string{"estado": "CANCELADO", "nota": "Prueba"}), 200, "cancelar")
	p := llamar(t, "POST", "/api/notificaciones/procesar", tok, nil)
	espera(t, p, 200, "procesar")
	if p.Body["enviados"].(float64) < 1 {
		t.Errorf("procesar: %v", p.Body)
	}
	for _, n := range llamar(t, "GET", "/api/citas/"+mantener["id"].(string), tok, nil).Body["notificaciones"].([]any) {
		m := n.(map[string]any)
		if m["tipo"] == "CONFIRMACION" {
			if m["estado"] != "ENVIADO" || m["simulado"] != true || !strings.HasPrefix(m["idProveedor"].(string), "SIM-") {
				t.Errorf("confirmación: %v", m)
			}
			if strings.Contains(strings.ToLower(m["mensaje"].(string)), "penicilina") || !strings.Contains(m["mensaje"].(string), "Ana") {
				t.Errorf("el mensaje no debe tener datos clínicos: %v", m["mensaje"])
			}
		}
	}
	for _, n := range llamar(t, "GET", "/api/citas/"+anular["id"].(string), tok, nil).Body["notificaciones"].([]any) {
		if n.(map[string]any)["estado"] != "CANCELADO" {
			t.Errorf("los mensajes de una cita cancelada no se envían: %v", n)
		}
	}
	b := llamar(t, "GET", "/api/notificaciones", tok, nil)
	if b.Body["proveedor"].(map[string]any)["simulado"] != true {
		t.Errorf("la bandeja debe indicar proveedor simulado: %v", b.Body["proveedor"])
	}
}

func TestCatalogos(t *testing.T) {
	requiereBD(t)
	adm := ingresar(t, admin)
	est := llamar(t, "GET", "/api/catalogos/estructura?todos=true", adm, nil)
	consultorio := est.Lista[0].(map[string]any)["consultorios"].([]any)[0].(map[string]any)["id"].(string)
	espera(t, llamar(t, "POST", "/api/catalogos/sillones", adm, map[string]any{"consultorioId": consultorio, "nombre": "Sillón Prueba"}), 201, "crear sillón")
	espera(t, llamar(t, "POST", "/api/catalogos/sillones", adm, map[string]any{"consultorioId": consultorio, "nombre": "Sillón Prueba"}), 409, "duplicado")
	espera(t, llamar(t, "POST", "/api/catalogos/procedimientos", adm, map[string]any{"codigo": "x", "nombre": "Mal", "duracionMin": 3, "arancel": "-1"}), 400, "procedimiento inválido")
	rec := ingresar(t, recepcion)
	s := llamar(t, "GET", "/api/catalogos/sillones", rec, nil)
	encontrado := false
	for _, x := range s.Lista {
		encontrado = encontrado || x.(map[string]any)["nombre"] == "Sillón Prueba"
	}
	if !encontrado {
		t.Error("la caché debe invalidarse al crear un sillón")
	}
	espera(t, llamar(t, "POST", "/api/catalogos/sillones", rec, map[string]any{"consultorioId": consultorio, "nombre": "Otro"}), 403, "recepción no crea")
	// Desde la Etapa 6 el Auditor lee catálogos (procedimientos y odontólogos para reglas de honorarios
	// y recetas de consumo), pero no los modifica.
	audTok := ingresar(t, auditor)
	espera(t, llamar(t, "GET", "/api/catalogos/sillones", audTok, nil), 200, "auditor lee catálogos")
	espera(t, llamar(t, "POST", "/api/catalogos/sillones", audTok, map[string]any{"consultorioId": consultorio, "nombre": "Otro"}), 403, "auditor no crea")

	// Un sillón desactivado no admite citas.
	tmp := llamar(t, "POST", "/api/catalogos/sillones", adm, map[string]any{"consultorioId": consultorio, "nombre": "Sillón Temporal"}).Body
	espera(t, llamar(t, "PATCH", "/api/catalogos/sillones/"+tmp["id"].(string), adm, map[string]any{"activo": false}), 200, "desactivar")
	i := cargarIDs(t)
	diaPrueba.Add(1)
	espera(t, llamar(t, "POST", "/api/citas", rec, cita(i, map[string]any{"sillonId": tmp["id"]})), 400, "sillón inactivo")
}

func TestPanelAgendaDelDia(t *testing.T) {
	requiereBD(t)
	w := llamar(t, "GET", "/api/panel", ingresar(t, recepcion), nil).Body["widgets"].(map[string]any)
	if w["agendaHoy"].(map[string]any)["alcance"] != "TODAS" {
		t.Errorf("recepción ve toda la agenda: %v", w)
	}
	w = llamar(t, "GET", "/api/panel", ingresar(t, odontologo), nil).Body["widgets"].(map[string]any)
	if w["agendaHoy"].(map[string]any)["alcance"] != "MIAS" || w["pacientes"].(map[string]any)["total"].(float64) < 8 {
		t.Errorf("odontólogo: %v", w)
	}
}

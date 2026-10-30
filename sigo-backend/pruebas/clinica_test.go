package pruebas

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Etapa 3 — expediente clínico: RF-03, RF-04, RF-05, RN-01 y RNF-06.

func idAna(t *testing.T) string {
	return idDe(t, `SELECT id FROM pacientes WHERE numero_documento='DEMO000101'`)
}

// marcasDe devuelve "diente/superficie/hallazgo" del estado vigente.
func marcasDe(body map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, m := range body["estado"].([]any) {
		x := m.(map[string]any)
		s, _ := x["superficie"].(string)
		out[strings.Join([]string{fmtNum(x["diente"]), s, x["hallazgo"].(string)}, "/")] = true
	}
	return out
}

func fmtNum(v any) string { return fmt.Sprint(int(v.(float64))) }

func TestOdontogramaEventosEHistorial(t *testing.T) {
	requiereBD(t)
	odo := ingresar(t, odontologo)
	ana := idAna(t)
	o := llamar(t, "GET", "/api/pacientes/"+ana+"/odontograma", odo, nil)
	espera(t, o, 200, "odontograma")
	if len(o.Body["catalogo"].([]any)) != 10 || !marcasDe(o.Body)["16/O/CARIES"] {
		t.Fatalf("catálogo/estado inicial: %v", o.Body["estado"])
	}
	if ultimaAuditoria(t, "LECTURA")["_entidad"] != ana {
		t.Error("la lectura del odontograma debe auditarse")
	}
	historial := len(o.Body["historial"].([]any))

	// "I" (incisal) se guarda como O; agregar dos veces ⇒ 409; quitar ⇒ deja de estar vigente.
	r := llamar(t, "POST", "/api/pacientes/"+ana+"/odontograma", odo, map[string]any{"diente": 21, "superficie": "I", "hallazgo": "CARIES"})
	espera(t, r, 201, "agregar caries incisal")
	if !marcasDe(r.Body)["21/O/CARIES"] {
		t.Errorf("caries 21: %v", r.Body["estado"])
	}
	if r := llamar(t, "POST", "/api/pacientes/"+ana+"/odontograma", odo, map[string]any{"diente": 21, "superficie": "O", "hallazgo": "CARIES"}); r.Body["code"] != "MARCA_YA_REGISTRADA" {
		t.Errorf("duplicado: %v", r.Body)
	}
	q := llamar(t, "POST", "/api/pacientes/"+ana+"/odontograma", odo, map[string]any{"diente": 21, "superficie": "O", "hallazgo": "CARIES", "accion": "QUITAR"})
	espera(t, q, 201, "quitar")
	if marcasDe(q.Body)["21/O/CARIES"] || len(q.Body["historial"].([]any)) != historial+2 {
		t.Errorf("quitar debe conservar el historial: %d eventos", len(q.Body["historial"].([]any)))
	}
	if r := llamar(t, "POST", "/api/pacientes/"+ana+"/odontograma", odo, map[string]any{"diente": 21, "superficie": "O", "hallazgo": "CARIES", "accion": "QUITAR"}); r.Body["code"] != "MARCA_INEXISTENTE" {
		t.Errorf("quitar inexistente: %v", r.Body)
	}
	for _, malo := range []map[string]any{
		{"diente": 19, "superficie": "O", "hallazgo": "CARIES"},
		{"diente": 54, "hallazgo": "IMPLANTE"},
		{"diente": 16, "hallazgo": "CARIES"},
		{"diente": 16, "superficie": "M", "hallazgo": "ENDODONCIA_REQUERIDA"},
		{"diente": 16, "superficie": "O", "hallazgo": "FRACTURA"},
	} {
		espera(t, llamar(t, "POST", "/api/pacientes/"+ana+"/odontograma", odo, malo), 400, "marca inválida")
	}
	// El asistente también registra hallazgos; recepción no ve el expediente.
	espera(t, llamar(t, "POST", "/api/pacientes/"+ana+"/odontograma", ingresar(t, asistente), map[string]any{"diente": 55, "superficie": "O", "hallazgo": "SELLANTE"}), 201, "asistente, temporal")
	for _, e := range []string{recepcion, auditor, admin} {
		espera(t, llamar(t, "GET", "/api/pacientes/"+ana+"/odontograma", ingresar(t, e), nil), 403, e+" no ve el odontograma")
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM odontograma_eventos`); err == nil || !strings.Contains(err.Error(), "solo insercion") {
		t.Errorf("el historial del odontograma no se borra: %v", err)
	}
}

func TestAnamnesisPendienteBloqueaRegistrosClinicos(t *testing.T) {
	requiereBD(t)
	p := llamar(t, "POST", "/api/pacientes", ingresar(t, recepcion), adulto(nil)).Body
	id := p["id"].(string)
	odo := ingresar(t, odontologo)
	for ruta, cuerpo := range map[string]any{
		"/odontograma":      map[string]any{"diente": 16, "superficie": "O", "hallazgo": "CARIES"},
		"/notas":            map[string]any{"subjetivo": "Dolor"},
		"/periodontogramas": map[string]any{"dientes": []any{map[string]any{"diente": 18, "ausente": true}}},
	} {
		if r := llamar(t, "POST", "/api/pacientes/"+id+ruta, odo, cuerpo); r.Status != 409 || r.Body["code"] != "ANAMNESIS_PENDIENTE" {
			t.Errorf("%s sin anamnesis: %d %v", ruta, r.Status, r.Body)
		}
	}
	espera(t, llamar(t, "GET", "/api/pacientes/"+id+"/odontograma", odo, nil), 200, "la lectura sí se permite")
}

func sitios(ps, mg []int, sangra bool) []any {
	out := []any{}
	for i, s := range []string{"MV", "V", "DV", "ML", "L", "DL"} {
		out = append(out, map[string]any{"sitio": s, "profundidad": ps[i], "margen": mg[i], "sangrado": sangra && i < 2})
	}
	return out
}

func TestPeriodontogramaNIC(t *testing.T) {
	requiereBD(t)
	odo := ingresar(t, odontologo)
	ana := idAna(t)
	cuerpo := map[string]any{"observaciones": "Prueba", "dientes": []any{
		map[string]any{"diente": 16, "movilidad": 1, "furca": 2, "sitios": sitios([]int{5, 3, 4, 2, 3, 2}, []int{-2, 0, 2, 0, 1, 0}, true)},
		map[string]any{"diente": 18, "ausente": true},
	}}
	r := llamar(t, "POST", "/api/pacientes/"+ana+"/periodontogramas", odo, cuerpo)
	espera(t, r, 201, "periodontograma")
	d16 := r.Body["dientes"].([]any)[0].(map[string]any)
	s := d16["sitios"].([]any)
	if s[0].(map[string]any)["nic"].(float64) != 3 || s[2].(map[string]any)["nic"].(float64) != 6 {
		t.Errorf("NIC = PS + MG (5−2=3, 4+2=6): %v", s)
	}
	res := r.Body["resumen"].(map[string]any)
	if res["sitios"].(float64) != 6 || res["bolsasDe4oMas"].(float64) != 2 || res["nicMaximo"].(float64) != 6 {
		t.Errorf("resumen: %v", res)
	}
	malos := []map[string]any{
		{"dientes": []any{map[string]any{"diente": 16, "sitios": sitios([]int{1, 1, 1, 1, 1, 1}, []int{0, 0, 0, 0, 0, 0}, false)[:5]}}},
		{"dientes": []any{map[string]any{"diente": 16, "sitios": sitios([]int{16, 1, 1, 1, 1, 1}, []int{0, 0, 0, 0, 0, 0}, false)}}},
		{"dientes": []any{map[string]any{"diente": 54, "sitios": sitios([]int{1, 1, 1, 1, 1, 1}, []int{0, 0, 0, 0, 0, 0}, false)}}},
		{"dientes": []any{map[string]any{"diente": 16, "movilidad": 4, "sitios": sitios([]int{1, 1, 1, 1, 1, 1}, []int{0, 0, 0, 0, 0, 0}, false)}}},
		{"dientes": []any{}},
	}
	for i, m := range malos {
		if r := llamar(t, "POST", "/api/pacientes/"+ana+"/periodontogramas", odo, m); r.Status != 400 {
			t.Errorf("caso %d debió ser 400: %d", i, r.Status)
		}
	}
	l := llamar(t, "GET", "/api/pacientes/"+ana+"/periodontogramas", odo, nil)
	if len(l.Lista) < 2 || l.Lista[0].(map[string]any)["id"] != r.Body["id"] {
		t.Errorf("lista (más reciente primero): %d", len(l.Lista))
	}
	if _, err := pool.Exec(context.Background(), `UPDATE perio_sitios SET profundidad = 1`); err == nil {
		t.Error("los sitios del periodontograma no se modifican")
	}
}

func notaCompleta() map[string]any {
	return map[string]any{"subjetivo": "Dolor al masticar", "objetivo": "Caries en 16", "evaluacion": "Caries dentinaria", "plan": "Restauración"}
}

func TestNotasSOEPFirmaYAdendas(t *testing.T) {
	requiereBD(t)
	ana := idAna(t)
	asis, odo := ingresar(t, asistente), ingresar(t, odontologo)

	b := llamar(t, "POST", "/api/pacientes/"+ana+"/notas", asis, map[string]any{"subjetivo": "Borrador del asistente"})
	espera(t, b, 201, "borrador")
	nid := b.Body["id"].(string)
	creado, _ := time.Parse(time.RFC3339Nano, b.Body["creadoEn"].(string))
	bloqueo, _ := time.Parse(time.RFC3339Nano, b.Body["bloqueoEn"].(string))
	if b.Body["estado"] != "BORRADOR" || b.Body["editable"] != true || bloqueo.Sub(creado).Round(time.Minute) != 24*time.Hour {
		t.Fatalf("borrador: %v (bloqueo %v)", b.Body, bloqueo.Sub(creado))
	}
	espera(t, llamar(t, "PUT", "/api/notas/"+nid, asis, notaCompleta()), 200, "el autor edita")
	if ultimaAuditoria(t, "MODIFICACION")["cambios"].(map[string]any)["subjetivo"] == nil {
		t.Error("la edición se audita con el valor anterior")
	}
	if r := llamar(t, "PUT", "/api/notas/"+nid, odo, notaCompleta()); r.Body["code"] != "NO_ES_AUTOR" {
		t.Errorf("otro usuario no edita el borrador: %v", r.Body)
	}
	espera(t, llamar(t, "POST", "/api/notas/"+nid+"/firmar", asis, map[string]string{"codigo": codigoVigente(t, asistente)}), 403, "el asistente no firma")
	if r := llamar(t, "POST", "/api/notas/"+nid+"/firmar", odo, map[string]string{"codigo": "000000"}); r.Body["code"] != "CODIGO_MFA_INVALIDO" {
		t.Errorf("firma con código inválido: %v", r.Body)
	}
	ultimaAuditoria(t, "FIRMA_FALLIDA")

	f := llamar(t, "POST", "/api/notas/"+nid+"/firmar", odo, map[string]string{"codigo": codigoVigente(t, odontologo)})
	espera(t, f, 200, "firmar")
	if f.Body["estado"] != "FIRMADA" || f.Body["integra"] != true || f.Body["registroProfesional"] != "REG-DEMO-0001" ||
		len(f.Body["hashFirma"].(string)) != 64 || f.Body["editable"] != false {
		t.Fatalf("firmada: %v", f.Body)
	}
	if r := llamar(t, "PUT", "/api/notas/"+nid, asis, notaCompleta()); r.Body["code"] != "NOTA_FIRMADA" {
		t.Errorf("una nota firmada no se edita: %v", r.Body)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE notas_soep SET plan='cambiado' WHERE id=$1`, nid); err == nil || !strings.Contains(err.Error(), "nota firmada") {
		t.Errorf("la BD también lo impide: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM notas_soep WHERE id=$1`, nid); err == nil {
		t.Error("las notas no se borran")
	}

	// Adenda: firmada con código MFA; la nota original no cambia.
	espera(t, llamar(t, "POST", "/api/notas/"+nid+"/adendas", odo, map[string]string{"texto": "Se agrega: paciente alérgico a penicilina.", "codigo": "123"}), 400, "adenda sin código válido")
	a := llamar(t, "POST", "/api/notas/"+nid+"/adendas", odo, map[string]string{"texto": "Se agrega: paciente alérgico a penicilina.", "codigo": codigoVigente(t, odontologo)})
	espera(t, a, 201, "adenda")
	if len(a.Body["adendas"].([]any)) != 1 || a.Body["plan"] != "Restauración" {
		t.Errorf("adenda: %v", a.Body)
	}

	// Un borrador no lleva adenda (se corrige directamente) y una nota incompleta no se firma.
	otra := llamar(t, "POST", "/api/pacientes/"+ana+"/notas", odo, map[string]any{"subjetivo": "Sólo S"}).Body["id"].(string)
	if r := llamar(t, "POST", "/api/notas/"+otra+"/adendas", odo, map[string]string{"texto": "texto", "codigo": codigoVigente(t, odontologo)}); r.Body["code"] != "NOTA_EDITABLE" {
		t.Errorf("adenda a borrador: %v", r.Body)
	}
	if r := llamar(t, "POST", "/api/notas/"+otra+"/firmar", odo, map[string]string{"codigo": codigoVigente(t, odontologo)}); r.Body["code"] != "NOTA_INCOMPLETA" {
		t.Errorf("firma de nota incompleta: %v", r.Body)
	}

	// Alteración directa en la BD (desactivando el trigger, como haría un DBA): el hash lo detecta.
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`ALTER TABLE notas_soep DISABLE TRIGGER notas_soep_guardia`, `UPDATE notas_soep SET plan='Exodoncia' WHERE id='` + nid + `'`,
		`ALTER TABLE notas_soep ENABLE TRIGGER notas_soep_guardia`} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, n := range llamar(t, "GET", "/api/pacientes/"+ana+"/notas", odo, nil).Lista {
		if m := n.(map[string]any); m["id"] == nid && m["integra"] != false {
			t.Errorf("la alteración debe detectarse: %v", m["integra"])
		}
	}
}

// RF-05: el bloqueo a las 24 h se aplica en la API y en la BD; el plazo es configurable.
func TestBloqueoDeNotasPorPlazo(t *testing.T) {
	requiereBD(t)
	ana := idAna(t)
	odo := ingresar(t, odontologo)
	nid := llamar(t, "POST", "/api/pacientes/"+ana+"/notas", odo, notaCompleta()).Body["id"].(string)
	ctx := context.Background()
	tx, _ := pool.Begin(ctx)
	for _, q := range []string{`ALTER TABLE notas_soep DISABLE TRIGGER notas_soep_guardia`,
		`UPDATE notas_soep SET creado_en = now() - interval '25 hours', bloqueo_en = now() - interval '1 hour' WHERE id='` + nid + `'`,
		`ALTER TABLE notas_soep ENABLE TRIGGER notas_soep_guardia`} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	_ = tx.Commit(ctx)
	if r := llamar(t, "PUT", "/api/notas/"+nid, odo, notaCompleta()); r.Body["code"] != "NOTA_BLOQUEADA" {
		t.Errorf("vencido el plazo no se edita: %v", r.Body)
	}
	if _, err := pool.Exec(ctx, `UPDATE notas_soep SET subjetivo='x' WHERE id=$1`, nid); err == nil || !strings.Contains(err.Error(), "bloqueada") {
		t.Errorf("la BD impide editar tras el plazo: %v", err)
	}
	l := llamar(t, "GET", "/api/pacientes/"+ana+"/notas", odo, nil)
	for _, n := range l.Lista {
		if m := n.(map[string]any); m["id"] == nid && m["estado"] != "BLOQUEADA_SIN_FIRMA" {
			t.Errorf("estado: %v", m["estado"])
		}
	}
	espera(t, llamar(t, "POST", "/api/notas/"+nid+"/firmar", odo, map[string]string{"codigo": codigoVigente(t, odontologo)}), 200, "la firma sí se registra (no cambia el texto)")

	// Parámetro configurable: con 2 h, las notas nuevas se bloquean a las 2 h.
	adm := ingresar(t, admin)
	espera(t, llamar(t, "PUT", "/api/ajustes/clinica.horasBloqueoNotas", adm, map[string]int{"valor": 2}), 200, "ajuste 2 h")
	defer llamar(t, "PUT", "/api/ajustes/clinica.horasBloqueoNotas", adm, map[string]int{"valor": 24})
	n := llamar(t, "POST", "/api/pacientes/"+ana+"/notas", odo, notaCompleta()).Body
	c1, _ := time.Parse(time.RFC3339Nano, n["creadoEn"].(string))
	b1, _ := time.Parse(time.RFC3339Nano, n["bloqueoEn"].(string))
	if b1.Sub(c1).Round(time.Minute) != 2*time.Hour {
		t.Errorf("bloqueo con 2 h: %v", b1.Sub(c1))
	}
}

func TestProcedimientosRealizadosInmutables(t *testing.T) {
	requiereBD(t)
	ana := idAna(t)
	odo := ingresar(t, odontologo)
	proc := idDe(t, `SELECT id FROM procedimientos WHERE codigo='RES-01'`)
	base := map[string]any{"procedimientoId": proc, "diente": 16, "superficies": []string{"O", "M", "o"}, "observaciones": "Resina compuesta"}
	espera(t, llamar(t, "POST", "/api/pacientes/"+ana+"/procedimientos", odo, base), 400, "sin código MFA")
	espera(t, llamar(t, "POST", "/api/pacientes/"+ana+"/procedimientos", ingresar(t, asistente), base), 403, "el asistente no registra procedimientos realizados")
	espera(t, llamar(t, "POST", "/api/pacientes/"+ana+"/procedimientos", odo, map[string]any{"procedimientoId": proc, "superficies": []string{"O"}, "codigo": codigoVigente(t, odontologo)}), 400, "superficies sin diente")
	base["codigo"] = codigoVigente(t, odontologo)
	r := llamar(t, "POST", "/api/pacientes/"+ana+"/procedimientos", odo, base)
	espera(t, r, 201, "registrar")
	if len(r.Body["superficies"].([]any)) != 2 || r.Body["codigo"] != "RES-01" || len(r.Body["hashFirma"].(string)) != 64 {
		t.Errorf("realizado: %v", r.Body)
	}
	rid := r.Body["id"].(string)
	ctx := context.Background()
	for _, q := range []string{`DELETE FROM procedimientos_realizados WHERE id=$1`, `UPDATE procedimientos_realizados SET observaciones='x' WHERE id=$1`} {
		if _, err := pool.Exec(ctx, q, rid); err == nil || !strings.Contains(err.Error(), "solo insercion") {
			t.Errorf("RN-01 en la BD (%s): %v", q, err)
		}
	}
	a := llamar(t, "POST", "/api/procedimientos-realizados/"+rid+"/adendas", odo, map[string]string{"texto": "Corrección: la superficie fue distal, no mesial.", "codigo": codigoVigente(t, odontologo)})
	espera(t, a, 201, "adenda")
	if len(a.Body["adendas"].([]any)) != 1 || len(a.Body["superficies"].([]any)) != 2 {
		t.Errorf("la adenda no cambia el registro original: %v", a.Body)
	}
	if ultimaAuditoria(t, "ADENDA")["hash"] == nil {
		t.Error("la adenda se audita con su hash")
	}
	l := llamar(t, "GET", "/api/pacientes/"+ana+"/procedimientos", odo, nil)
	if len(l.Lista) == 0 {
		t.Error("lista de procedimientos")
	}
	espera(t, llamar(t, "GET", "/api/pacientes/"+ana+"/procedimientos", ingresar(t, recepcion), nil), 403, "recepción no ve el expediente")
}

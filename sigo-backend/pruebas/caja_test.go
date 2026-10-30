package pruebas

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

// Etapa 5 — caja con pagos mixtos y comprobantes (RF-12) y cierre ciego (RN-02).

// realizadoDirecto registra un procedimiento realizado en la BD (la firma se prueba en clinica_test).
func realizadoDirecto(t *testing.T, pacienteID, codigo string) string {
	t.Helper()
	return idDe(t, `INSERT INTO procedimientos_realizados (paciente_id, procedimiento_id, odontologo_id, hash_firma)
		VALUES ($1, (SELECT id FROM procedimientos WHERE codigo=$2), (SELECT id FROM usuarios WHERE email=$3), 'prueba') RETURNING id`,
		pacienteID, codigo, odontologo)
}

func pago(medio, monto, ref string) map[string]string {
	return map[string]string{"medio": medio, "monto": monto, "referencia": ref}
}

// sinTeorico falla si una respuesta para el recepcionista revela el teórico o una suma de la caja.
func sinTeorico(t *testing.T, r resp, prohibidos ...string) {
	t.Helper()
	b, _ := json.Marshal(r.Body)
	if r.Lista != nil {
		b, _ = json.Marshal(r.Lista)
	}
	s := strings.ToLower(string(b))
	for _, p := range append([]string{"teorico", "diferencia", "declarado"}, prohibidos...) {
		if strings.Contains(s, strings.ToLower(p)) {
			t.Errorf("RN-02: la respuesta al recepcionista contiene %q: %s", p, s)
		}
	}
}

func TestCajaCobroMixtoYCierreCiego(t *testing.T) {
	requiereBD(t)
	ctx := context.Background()
	rec := ingresar(t, recepcion)
	jorge := pacienteDoc(t, "DEMO000106")
	val := procID(t, "VAL-01")
	conv := idDe(t, `SELECT id FROM convenios WHERE nombre LIKE 'Convenio Demo%'`)

	espera(t, llamar(t, "POST", "/api/caja/sesion", ingresar(t, odontologo), map[string]string{"fondoInicial": "10"}), 403, "el odontólogo no opera caja")
	r := llamar(t, "GET", "/api/caja/sesion", rec, nil)
	espera(t, r, 200, "mi caja")
	if r.Body["abierta"] != false {
		t.Fatalf("no debería haber caja abierta: %v", r.Body)
	}
	cobroVal := map[string]any{"pacienteId": jorge, "conceptos": []map[string]string{{"procedimientoId": val}}, "pagos": []any{pago("EFECTIVO", "25.00", "")}}
	r = llamar(t, "POST", "/api/cobros", rec, cobroVal)
	espera(t, r, 409, "sin caja abierta no se cobra")
	if r.Body["code"] != "CAJA_NO_ABIERTA" {
		t.Errorf("código: %v", r.Body["code"])
	}
	espera(t, llamar(t, "POST", "/api/caja/sesion", rec, map[string]string{"fondoInicial": "-5"}), 400, "fondo negativo")
	r = llamar(t, "POST", "/api/caja/sesion", rec, map[string]string{"fondoInicial": "50"})
	espera(t, r, 201, "abrir caja")
	sesion := r.Body["sesion"].(map[string]any)["id"].(string)
	sinTeorico(t, r)
	espera(t, llamar(t, "POST", "/api/caja/sesion", rec, map[string]string{"fondoInicial": "50"}), 409, "una sola caja abierta por recepcionista")

	// Pendientes de cobro: el realizado aparece con su arancel.
	rz := realizadoDirecto(t, jorge, "PRO-01")
	pc := llamar(t, "GET", "/api/pacientes/"+jorge+"/por-cobrar", rec, nil)
	espera(t, pc, 200, "por cobrar")
	var pend map[string]any
	for _, x := range pc.Lista {
		if m := x.(map[string]any); m["procedimientoRealizadoId"] == rz {
			pend = m
		}
	}
	if pend == nil || pend["monto"] != "35.00" || pend["origen"] != "ARANCEL" {
		t.Fatalf("pendiente: %v", pc.Lista)
	}

	// Pago mixto: 35.00 (realizado) + 25.00 (consulta) = 60.00
	conceptos := []map[string]string{{"procedimientoRealizadoId": rz}, {"procedimientoId": val}}
	espera(t, llamar(t, "POST", "/api/cobros", rec, map[string]any{"pacienteId": jorge, "conceptos": conceptos,
		"pagos": []any{pago("EFECTIVO", "40.00", ""), pago("TARJETA", "20.00", "")}}), 400, "tarjeta sin número de voucher")
	r = llamar(t, "POST", "/api/cobros", rec, map[string]any{"pacienteId": jorge, "conceptos": conceptos,
		"pagos": []any{pago("EFECTIVO", "30.00", ""), pago("TARJETA", "20.00", "V-123")}})
	espera(t, r, 400, "los pagos no suman el total")
	if r.Body["code"] != "PAGO_NO_CUADRA" || r.Body["extra"].(map[string]any)["total"] != "60.00" {
		t.Errorf("no cuadra: %v", r.Body)
	}
	espera(t, llamar(t, "POST", "/api/cobros", rec, map[string]any{"pacienteId": jorge, "conceptos": conceptos,
		"pagos": []any{pago("EFECTIVO", "50.00", ""), map[string]string{"medio": "SEGURO", "monto": "10.00"}}}), 400, "seguro sin convenio")
	seguro := map[string]string{"medio": "SEGURO", "monto": "10.00", "convenioId": conv}
	r = llamar(t, "POST", "/api/cobros", rec, map[string]any{"pacienteId": jorge, "conceptos": conceptos,
		"pagos": []any{pago("EFECTIVO", "30.00", ""), pago("TARJETA", "20.00", "V-123"), seguro}})
	espera(t, r, 201, "cobro con pago mixto")
	if r.Body["total"] != "60.00" || len(r.Body["pagos"].([]any)) != 3 || len(r.Body["conceptos"].([]any)) != 2 {
		t.Errorf("cobro: %v", r.Body)
	}
	comp := r.Body["comprobantes"].([]any)
	if len(comp) != 1 || comp[0].(map[string]any)["estado"] != "AUTORIZADO" || comp[0].(map[string]any)["simulado"] != true ||
		!strings.HasPrefix(comp[0].(map[string]any)["numero"].(string), "SIM-F-") {
		t.Errorf("factura simulada: %v", comp)
	}
	espera(t, llamar(t, "POST", "/api/cobros", rec, map[string]any{"pacienteId": jorge, "conceptos": []map[string]string{{"procedimientoRealizadoId": rz}},
		"pagos": []any{pago("EFECTIVO", "35.00", "")}}), 400, "un realizado no se cobra dos veces")
	for _, x := range llamar(t, "GET", "/api/pacientes/"+jorge+"/por-cobrar", rec, nil).Lista {
		if x.(map[string]any)["procedimientoRealizadoId"] == rz {
			t.Error("ya cobrado: no debe seguir pendiente")
		}
	}

	// Anulación con nota de crédito.
	r = llamar(t, "POST", "/api/cobros", rec, cobroVal)
	espera(t, r, 201, "cobro de consulta")
	anular := r.Body["id"].(string)
	espera(t, llamar(t, "POST", "/api/cobros/"+anular+"/anular", rec, map[string]string{"motivo": "x"}), 400, "motivo muy corto")
	r = llamar(t, "POST", "/api/cobros/"+anular+"/anular", rec, map[string]string{"motivo": "Se cobró al paciente equivocado."})
	espera(t, r, 200, "anular")
	cs := r.Body["comprobantes"].([]any)
	if r.Body["anulacion"] == nil || len(cs) != 2 || cs[1].(map[string]any)["tipo"] != "NOTA_CREDITO" || cs[1].(map[string]any)["estado"] != "AUTORIZADO" {
		t.Errorf("anulación con nota de crédito: %v", r.Body)
	}
	espera(t, llamar(t, "POST", "/api/cobros/"+anular+"/anular", rec, map[string]string{"motivo": "Otra vez el mismo."}), 409, "no se anula dos veces")

	// El servicio de facturación no responde: el cobro queda y la factura se reintenta.
	facturador.FallarSiguiente = true
	r = llamar(t, "POST", "/api/cobros", rec, map[string]any{"pacienteId": jorge, "conceptos": []map[string]string{{"procedimientoId": val}},
		"pagos": []any{pago("TRANSFERENCIA", "25.00", "TRF-778")}})
	espera(t, r, 201, "cobro aunque la facturación falle")
	reintento := r.Body["id"].(string)
	if c := r.Body["comprobantes"].([]any); len(c) != 1 || c[0].(map[string]any)["estado"] != "ERROR" {
		t.Errorf("comprobante con error: %v", c)
	}
	espera(t, llamar(t, "POST", "/api/cobros/"+reintento+"/comprobante", ingresar(t, auditor), nil), 403, "el auditor no emite comprobantes")
	r = llamar(t, "POST", "/api/cobros/"+reintento+"/comprobante", rec, nil)
	espera(t, r, 200, "reintento")
	if c := r.Body["comprobantes"].([]any); len(c) != 2 || c[1].(map[string]any)["estado"] != "AUTORIZADO" {
		t.Errorf("reintento autorizado: %v", c)
	}
	espera(t, llamar(t, "POST", "/api/cobros/"+reintento+"/comprobante", rec, nil), 409, "ya autorizado")

	// Doble cobro simultáneo del mismo realizado: exactamente uno gana.
	rz2 := realizadoDirecto(t, jorge, "PRO-01")
	var ok, mal int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x := llamar(t, "POST", "/api/cobros", rec, map[string]any{"pacienteId": jorge, "conceptos": []map[string]string{{"procedimientoRealizadoId": rz2}},
				"pagos": []any{pago("EFECTIVO", "35.00", "")}})
			mu.Lock()
			defer mu.Unlock()
			if x.Status == 201 {
				ok++
			} else if x.Status == 409 || x.Status == 400 {
				mal++
			}
		}()
	}
	wg.Wait()
	if ok != 1 || mal != 9 {
		t.Errorf("doble cobro simultáneo: %d éxitos, %d rechazos", ok, mal)
	}

	// Lo que ve el recepcionista no revela totales.
	mc := llamar(t, "GET", "/api/caja/sesion", rec, nil)
	sinTeorico(t, mc)
	if mc.Body["sesion"].(map[string]any)["cobros"].(float64) != 4 {
		t.Errorf("cantidad de cobros: %v", mc.Body)
	}
	sinTeorico(t, llamar(t, "GET", "/api/caja/cobros", rec, nil))
	espera(t, llamar(t, "GET", "/api/caja/cierres", rec, nil), 403, "recepción no ve los cierres")

	// BD: sólo inserción y cuadre al centavo.
	for _, q := range []string{`UPDATE cobros SET total=1 WHERE sesion_id=$1`, `DELETE FROM cobro_pagos WHERE cobro_id IN (SELECT id FROM cobros WHERE sesion_id=$1)`} {
		if _, err := pool.Exec(ctx, q, sesion); err == nil {
			t.Errorf("la BD debe impedir: %s", q)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO cobros (sesion_id, paciente_id, total, registrado_por) VALUES ($1,$2,99,(SELECT id FROM usuarios WHERE email=$3))`,
		sesion, jorge, recepcion); err == nil || !strings.Contains(err.Error(), "deben sumar") {
		t.Errorf("un cobro sin conceptos ni pagos no se confirma: %v", err)
	}

	// Cierre ciego. Teórico: efectivo 50 + 30 + 35 = 115.00 (la consulta anulada no cuenta),
	// tarjeta 20.00, transferencia 25.00. Se declara 113.00 en efectivo (faltan 2.00).
	espera(t, llamar(t, "POST", "/api/caja/sesion/cerrar", rec, map[string]string{"efectivo": "abc", "tarjeta": "0", "transferencia": "0"}), 400, "monto inválido")
	r = llamar(t, "POST", "/api/caja/sesion/cerrar", rec, map[string]string{"efectivo": "113.00", "tarjeta": "20", "transferencia": "25.00", "nota": "Conteo de la tarde"})
	espera(t, r, 200, "cierre ciego")
	sinTeorico(t, r, "115.00", "-2.00")
	espera(t, llamar(t, "POST", "/api/caja/sesion/cerrar", rec, map[string]string{"efectivo": "1", "tarjeta": "0", "transferencia": "0"}), 409, "no hay caja abierta")
	espera(t, llamar(t, "POST", "/api/cobros", rec, cobroVal), 409, "caja cerrada: no se cobra")
	espera(t, llamar(t, "POST", "/api/cobros/"+reintento+"/anular", rec, map[string]string{"motivo": "Después del cierre."}), 409, "caja cerrada: no se anula")
	if _, err := pool.Exec(ctx, `INSERT INTO cobros_anulaciones (cobro_id, motivo, anulado_por) VALUES ($1, 'directo en la BD', (SELECT id FROM usuarios WHERE email=$2))`,
		reintento, recepcion); err == nil {
		t.Error("la BD no admite anular en una caja cerrada")
	}

	// El Auditor concilia.
	aud := ingresar(t, auditor)
	l := llamar(t, "GET", "/api/caja/cierres?estado=CERRADA", aud, nil)
	espera(t, l, 200, "cierres por revisar")
	var cierre map[string]any
	for _, x := range l.Lista {
		if m := x.(map[string]any); m["id"] == sesion {
			cierre = m
		}
	}
	if cierre == nil || cierre["cuadra"] != false || cierre["anulados"].(float64) != 1 || cierre["seguro"] != "10.00" {
		t.Fatalf("cierre: %v", cierre)
	}
	dif := map[string]map[string]any{}
	for _, d := range cierre["diferencias"].([]any) {
		dif[d.(map[string]any)["medio"].(string)] = d.(map[string]any)
	}
	if dif["EFECTIVO"]["teorico"] != "115.00" || dif["EFECTIVO"]["diferencia"] != "-2.00" || dif["TARJETA"]["cuadra"] != true || dif["TRANSFERENCIA"]["teorico"] != "25.00" {
		t.Errorf("diferencias: %v", dif)
	}
	// Tolerancia configurable: con 3.00 el faltante de 2.00 cuadra.
	adm := ingresar(t, admin)
	espera(t, llamar(t, "PUT", "/api/ajustes/caja.toleranciaCentavos", adm, map[string]int{"valor": 300}), 200, "tolerancia 3.00")
	if d := llamar(t, "GET", "/api/caja/cierres/"+sesion, aud, nil); d.Body["cierre"].(map[string]any)["cuadra"] != true {
		t.Errorf("con tolerancia debe cuadrar: %v", d.Body["cierre"])
	}
	espera(t, llamar(t, "PUT", "/api/ajustes/caja.toleranciaCentavos", adm, map[string]int{"valor": 0}), 200, "tolerancia 0")
	espera(t, llamar(t, "POST", "/api/caja/cierres/"+sesion+"/revisar", aud, map[string]string{}), 400, "descuadre sin nota")
	r = llamar(t, "POST", "/api/caja/cierres/"+sesion+"/revisar", aud, map[string]string{"nota": "Faltante de 2.00 repuesto por la cajera."})
	espera(t, r, 200, "revisión")
	if r.Body["cierre"].(map[string]any)["estado"] != "REVISADA" || len(r.Body["cobros"].([]any)) != 4 {
		t.Errorf("revisada: %v", r.Body["cierre"])
	}
	espera(t, llamar(t, "POST", "/api/caja/cierres/"+sesion+"/revisar", aud, map[string]string{"nota": "otra vez"}), 409, "no se revisa dos veces")
	if _, err := pool.Exec(ctx, `UPDATE caja_sesiones SET teorico_efectivo=113 WHERE id=$1`, sesion); err == nil {
		t.Error("una caja revisada no cambia")
	}
	cxc := llamar(t, "GET", "/api/caja/cuentas-por-cobrar", aud, nil)
	espera(t, cxc, 200, "cuentas por cobrar a aseguradoras")
	if len(cxc.Lista) == 0 {
		t.Error("el pago con seguro genera una cuenta por cobrar")
	}
	h := llamar(t, "GET", "/api/pacientes/"+jorge+"/cobros", aud, nil)
	if len(h.Lista) < 4 {
		t.Errorf("historial del paciente: %d", len(h.Lista))
	}
	if p := llamar(t, "GET", "/api/panel", aud, nil); p.Body["widgets"].(map[string]any)["cierres"] == nil {
		t.Error("el panel del auditor muestra los cierres por revisar")
	}
	if p := llamar(t, "GET", "/api/panel", rec, nil); p.Body["widgets"].(map[string]any)["caja"].(map[string]any)["abierta"] != false {
		t.Error("el panel de recepción muestra el estado de su caja")
	}
}

func TestCobroDesdePresupuestoAprobado(t *testing.T) {
	requiereBD(t)
	// Un realizado hecho desde una tarea se cobra con el subtotal del ítem aprobado, no con el arancel.
	pac := pacienteDoc(t, "DEMO000104") // sin anamnesis: el realizado se inserta directo
	item := idDe(t, `WITH p AS (INSERT INTO presupuestos (paciente_id, odontologo_id, titulo) VALUES ($1,(SELECT id FROM usuarios WHERE email=$2),'Prueba caja') RETURNING id)
		INSERT INTO presupuesto_items (presupuesto_id, orden, fase, fase_nombre, sesion, procedimiento_id, honorario, costo_laboratorio, descuento)
		SELECT p.id, 1, 1, 'Fase 1', 1, (SELECT id FROM procedimientos WHERE codigo='END-01'), 150.00, 12.50, 2.50 FROM p RETURNING id`, pac, odontologo)
	tarea := idDe(t, `INSERT INTO tareas_tratamiento (presupuesto_item_id, paciente_id, procedimiento_id, fase, sesion)
		VALUES ($1,$2,(SELECT id FROM procedimientos WHERE codigo='END-01'),1,1) RETURNING id`, item, pac)
	rz := idDe(t, `INSERT INTO procedimientos_realizados (paciente_id, procedimiento_id, odontologo_id, hash_firma, tarea_id)
		VALUES ($1,(SELECT id FROM procedimientos WHERE codigo='END-01'),(SELECT id FROM usuarios WHERE email=$2),'prueba',$3) RETURNING id`, pac, odontologo, tarea)
	rec := ingresar(t, recepcion)
	for _, x := range llamar(t, "GET", "/api/pacientes/"+pac+"/por-cobrar", rec, nil).Lista {
		if m := x.(map[string]any); m["procedimientoRealizadoId"] == rz {
			if m["monto"] != "160.00" || m["origen"] != "PRESUPUESTO" {
				t.Errorf("150 + 12.50 − 2.50 = 160.00 del presupuesto: %v", m)
			}
			return
		}
	}
	t.Error("el realizado desde la tarea debe estar pendiente de cobro")
}

package pruebas

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Etapa 6 — inventario (RF-14) y honorarios (RF-13).

func insumoID(t *testing.T, codigo string) string {
	return idDe(t, `SELECT id FROM insumos WHERE codigo=$1`, codigo)
}

func ubicacionID(t *testing.T, nombre string) string {
	return idDe(t, `SELECT id FROM inventario_ubicaciones WHERE nombre=$1`, nombre)
}

func saldo(t *testing.T, insumo, ubicacion string) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(), `SELECT COALESCE(sum(cantidad),0)::numeric(12,3)::text FROM movimientos_stock WHERE insumo_id=$1 AND ubicacion_id=$2`,
		insumo, ubicacion).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func registrarProc(t *testing.T, tok, pac, codigo string, extra map[string]any) resp {
	t.Helper()
	body := map[string]any{"procedimientoId": procID(t, codigo), "codigo": codigoVigente(t, odontologo)}
	for k, v := range extra {
		body[k] = v
	}
	return llamar(t, "POST", "/api/pacientes/"+pac+"/procedimientos", tok, body)
}

func TestDescargoAutomaticoDeInsumos(t *testing.T) {
	requiereBD(t)
	ctx := context.Background()
	odo := ingresar(t, odontologo)
	pac := pacienteDoc(t, "DEMO000107")
	bodega, c1 := ubicacionID(t, "Bodega central"), ubicacionID(t, "Consultorio 1")
	gua, res := insumoID(t, "GUA-01"), insumoID(t, "RES-A2")
	antesG, antesR, antesC1 := saldo(t, gua, bodega), saldo(t, res, bodega), saldo(t, gua, c1)

	// Sin cita ni ubicación: se descarga de la bodega (por defecto) según la receta de RES-01.
	r := registrarProc(t, odo, pac, "RES-01", map[string]any{"diente": 36, "superficies": []string{"O"}})
	espera(t, r, 201, "procedimiento con descargo")
	rz := r.Body["id"].(string)
	if saldo(t, gua, bodega) != restar(antesG, "1.000") || saldo(t, res, bodega) != restar(antesR, "0.300") {
		t.Errorf("descargo en bodega: guantes %s→%s, resina %s→%s", antesG, saldo(t, gua, bodega), antesR, saldo(t, res, bodega))
	}
	k := llamar(t, "GET", "/api/inventario/movimientos?procedimientoRealizadoId="+rz, odo, nil)
	espera(t, k, 200, "kardex del procedimiento")
	if len(k.Lista) != 6 || k.Lista[0].(map[string]any)["tipo"] != "SALIDA_PROCEDIMIENTO" {
		t.Errorf("6 insumos descargados: %v", k.Lista)
	}
	// Con ubicación indicada: se descarga del consultorio.
	espera(t, registrarProc(t, odo, pac, "PRO-01", map[string]any{"ubicacionId": c1}), 201, "descargo en el consultorio")
	if saldo(t, gua, c1) != restar(antesC1, "1.000") {
		t.Errorf("guantes del consultorio: %s → %s", antesC1, saldo(t, gua, c1))
	}

	// Atomicidad: si el descargo falla, tampoco queda el procedimiento.
	var antes int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM procedimientos_realizados WHERE paciente_id=$1`, pac).Scan(&antes)
	if _, err := pool.Exec(ctx, `CREATE OR REPLACE FUNCTION prueba_falla() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'falla simulada'; END; $$ LANGUAGE plpgsql;
		CREATE TRIGGER prueba_falla BEFORE INSERT ON movimientos_stock FOR EACH ROW WHEN (NEW.tipo = 'SALIDA_PROCEDIMIENTO') EXECUTE FUNCTION prueba_falla();`); err != nil {
		t.Fatal(err)
	}
	r = registrarProc(t, odo, pac, "PRO-01", nil)
	_, _ = pool.Exec(ctx, `DROP TRIGGER prueba_falla ON movimientos_stock; DROP FUNCTION prueba_falla();`)
	espera(t, r, 500, "falla del descargo")
	var despues int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM procedimientos_realizados WHERE paciente_id=$1`, pac).Scan(&despues)
	if despues != antes {
		t.Error("si el descargo falla, el procedimiento no debe quedar registrado")
	}

	// Stock insuficiente: no bloquea el registro clínico; el saldo queda negativo y se alerta.
	sut := insumoID(t, "SUT-01")
	for i := 0; i < 3; i++ {
		espera(t, registrarProc(t, odo, pac, "EXO-01", nil), 201, "exodoncia")
	}
	st := llamar(t, "GET", "/api/inventario/stock?ubicacionId="+bodega, odo, nil)
	espera(t, st, 200, "stock de bodega")
	var sutura map[string]any
	for _, x := range st.Lista {
		if m := x.(map[string]any); m["insumoId"] == sut {
			sutura = m
		}
	}
	if sutura == nil || sutura["negativo"] != true || sutura["bajo"] != true || sutura["cantidad"] != "-1.000" {
		t.Errorf("sutura: %v", sutura)
	}
}

// restar: a − b con 3 decimales (para comparar saldos como texto).
func restar(a, b string) string {
	var s string
	_ = pool.QueryRow(context.Background(), `SELECT ($1::numeric - $2::numeric)::numeric(12,3)::text`, a, b).Scan(&s)
	return s
}

func TestMovimientosYConfiguracion(t *testing.T) {
	requiereBD(t)
	ctx := context.Background()
	asis, aud, odo := ingresar(t, asistente), ingresar(t, auditor), ingresar(t, odontologo)
	bodega, c2 := ubicacionID(t, "Bodega central"), ubicacionID(t, "Consultorio 2")
	gas := insumoID(t, "GAS-01")
	mov := "/api/inventario/movimientos"

	espera(t, llamar(t, "POST", mov, odo, map[string]any{"tipo": "ENTRADA", "insumoId": gas, "ubicacionId": bodega, "cantidad": "10"}), 403, "el odontólogo no registra movimientos")
	espera(t, llamar(t, "POST", mov, asis, map[string]any{"tipo": "SALIDA_PROCEDIMIENTO", "insumoId": gas, "ubicacionId": bodega, "cantidad": "1"}), 400, "salidas por procedimiento son automáticas")
	espera(t, llamar(t, "POST", mov, asis, map[string]any{"tipo": "ENTRADA", "insumoId": gas, "ubicacionId": bodega, "cantidad": "-5"}), 400, "entrada negativa")
	antes := saldo(t, gas, bodega)
	espera(t, llamar(t, "POST", mov, asis, map[string]any{"tipo": "ENTRADA", "insumoId": gas, "ubicacionId": bodega, "cantidad": "100", "costoUnitario": "0.0625"}), 201, "entrada con costo nuevo")
	var costo string
	_ = pool.QueryRow(ctx, `SELECT costo_unitario::text FROM insumos WHERE id=$1`, gas).Scan(&costo)
	if costo != "0.0625" || saldo(t, gas, bodega) != restar(antes, "-100") {
		t.Errorf("entrada: costo %s, saldo %s", costo, saldo(t, gas, bodega))
	}
	espera(t, llamar(t, "POST", mov, asis, map[string]any{"tipo": "AJUSTE", "insumoId": gas, "ubicacionId": bodega, "cantidad": "-3"}), 400, "ajuste sin motivo")
	espera(t, llamar(t, "POST", mov, asis, map[string]any{"tipo": "AJUSTE", "insumoId": gas, "ubicacionId": bodega, "cantidad": "-3", "motivo": "Conteo físico: 3 rotas"}), 201, "ajuste")
	espera(t, llamar(t, "POST", mov, asis, map[string]any{"tipo": "TRANSFERENCIA", "insumoId": gas, "origenId": bodega, "ubicacionId": bodega, "cantidad": "5"}), 400, "transferencia al mismo lugar")
	antesB, antesC := saldo(t, gas, bodega), saldo(t, gas, c2)
	espera(t, llamar(t, "POST", mov, asis, map[string]any{"tipo": "TRANSFERENCIA", "insumoId": gas, "origenId": bodega, "ubicacionId": c2, "cantidad": "20"}), 201, "transferencia")
	if saldo(t, gas, bodega) != restar(antesB, "20") || saldo(t, gas, c2) != restar(antesC, "-20") {
		t.Error("la transferencia mueve el stock entre ubicaciones")
	}
	if _, err := pool.Exec(ctx, `UPDATE movimientos_stock SET cantidad=1 WHERE insumo_id=$1`, gas); err == nil {
		t.Error("el kardex es de sólo inserción")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO movimientos_stock (insumo_id, ubicacion_id, tipo, cantidad, costo_unitario, transferencia_id, registrado_por)
		VALUES ($1,$2,'TRANSFERENCIA',-5,0,gen_random_uuid(),(SELECT id FROM usuarios WHERE email=$3))`, gas, bodega, asistente); err == nil ||
		!strings.Contains(err.Error(), "transferencia") {
		t.Errorf("una transferencia con una sola pata no se confirma: %v", err)
	}

	// Insumos y recetas: sólo el Auditor (inventario.configurar).
	nuevo := map[string]any{"codigo": "FLU-01", "nombre": "Barniz de flúor", "unidad": "ml", "costoUnitario": "0.95", "stockMinimo": "10"}
	espera(t, llamar(t, "POST", "/api/inventario/insumos", asis, nuevo), 403, "el asistente no crea insumos")
	espera(t, llamar(t, "POST", "/api/inventario/insumos", aud, map[string]any{"codigo": "x", "nombre": "", "unidad": "", "costoUnitario": "1.23456"}), 400, "validación")
	r := llamar(t, "POST", "/api/inventario/insumos", aud, nuevo)
	espera(t, r, 201, "crear insumo")
	flu := r.Body["id"].(string)
	espera(t, llamar(t, "POST", "/api/inventario/insumos", aud, nuevo), 409, "código repetido")
	espera(t, llamar(t, "PATCH", "/api/inventario/insumos/"+flu, aud, map[string]any{"nombre": "Barniz de flúor 5 %", "unidad": "ml", "costoUnitario": "1.10", "activo": true}), 200, "modificar insumo")

	sel := procID(t, "SEL-01")
	ruta := "/api/inventario/recetas/" + sel
	espera(t, llamar(t, "PUT", ruta, asis, map[string]any{"lineas": []any{}}), 403, "el asistente no cambia recetas")
	espera(t, llamar(t, "PUT", ruta, aud, map[string]any{"lineas": []map[string]string{{"insumoId": flu, "cantidad": "1"}, {"insumoId": flu, "cantidad": "2"}}}), 400, "insumo repetido")
	r = llamar(t, "PUT", ruta, aud, map[string]any{"lineas": []map[string]string{{"insumoId": insumoID(t, "GUA-01"), "cantidad": "1"}, {"insumoId": flu, "cantidad": "0.5"}}})
	espera(t, r, 200, "guardar receta")
	for _, x := range r.Lista {
		if m := x.(map[string]any); m["procedimientoId"] == sel {
			if len(m["lineas"].([]any)) != 2 || m["costoEstandar"] != "0.73" { // 1 × 0.18 + 0.5 × 1.10 = 0.73
				t.Errorf("receta: %v", m)
			}
		}
	}
	if ultimaAuditoria(t, "MODIFICACION")["antes"] == nil {
		t.Error("el cambio de receta se audita con la versión anterior")
	}
}

func TestHonorariosTresReglasYLiquidacion(t *testing.T) {
	requiereBD(t)
	ctx := context.Background()
	pac := pacienteDoc(t, "DEMO000106")
	odo, aud, rec := ingresar(t, odontologo), ingresar(t, auditor), ingresar(t, recepcion)
	valeria := idDe(t, `SELECT id FROM usuarios WHERE email=$1`, odontologo)
	jorge := idDe(t, `SELECT id FROM usuarios WHERE email=$1`, odontologo2)

	// PRO-01 por la API (descarga 1 par de guantes y 5 g de pasta: 0.18 + 0.40 = 0.58 de materiales).
	r := registrarProc(t, odo, pac, "PRO-01", nil)
	espera(t, r, 201, "profilaxis")
	pro := r.Body["id"].(string)
	// END-01 desde un presupuesto: honorario 200.00 + laboratorio 30.00, con 3 limas descargadas (3.30).
	item := idDe(t, `WITH p AS (INSERT INTO presupuestos (paciente_id, odontologo_id, titulo) VALUES ($1,$2,'Honorarios') RETURNING id)
		INSERT INTO presupuesto_items (presupuesto_id, orden, fase, fase_nombre, sesion, procedimiento_id, honorario, costo_laboratorio)
		SELECT p.id, 1, 1, 'Fase 1', 1, (SELECT id FROM procedimientos WHERE codigo='END-01'), 200.00, 30.00 FROM p RETURNING id`, pac, valeria)
	tarea := idDe(t, `INSERT INTO tareas_tratamiento (presupuesto_item_id, paciente_id, procedimiento_id, fase, sesion)
		VALUES ($1,$2,(SELECT id FROM procedimientos WHERE codigo='END-01'),1,1) RETURNING id`, item, pac)
	end := idDe(t, `INSERT INTO procedimientos_realizados (paciente_id, procedimiento_id, odontologo_id, hash_firma, tarea_id)
		VALUES ($1,(SELECT id FROM procedimientos WHERE codigo='END-01'),$2,'prueba',$3) RETURNING id`, pac, valeria, tarea)
	if _, err := pool.Exec(ctx, `INSERT INTO movimientos_stock (insumo_id, ubicacion_id, tipo, cantidad, costo_unitario, procedimiento_realizado_id, registrado_por)
		VALUES ((SELECT id FROM insumos WHERE codigo='LIM-01'), (SELECT id FROM inventario_ubicaciones WHERE por_defecto), 'SALIDA_PROCEDIMIENTO', -3, 1.10, $1, $2)`, end, valeria); err != nil {
		t.Fatal(err)
	}
	// Jorge: profilaxis con regla de monto fijo (12.00).
	pj := idDe(t, `INSERT INTO procedimientos_realizados (paciente_id, procedimiento_id, odontologo_id, hash_firma)
		VALUES ($1,(SELECT id FROM procedimientos WHERE codigo='PRO-01'),$2,'prueba') RETURNING id`, pac, jorge)

	// Se cobran (los honorarios se calculan sobre lo cobrado).
	espera(t, llamar(t, "POST", "/api/caja/sesion", rec, map[string]string{"fondoInicial": "0"}), 201, "abrir caja")
	espera(t, llamar(t, "POST", "/api/cobros", rec, map[string]any{"pacienteId": pac,
		"conceptos": []map[string]string{{"procedimientoRealizadoId": pro}, {"procedimientoRealizadoId": end}, {"procedimientoRealizadoId": pj}},
		"pagos":     []any{pago("EFECTIVO", "300.00", "")}}), 201, "cobro 35 + 230 + 35")
	espera(t, llamar(t, "POST", "/api/caja/sesion/cerrar", rec, map[string]string{"efectivo": "300", "tarjeta": "0", "transferencia": "0"}), 200, "cerrar caja")

	hoy := hoyLocal()
	calc := "/api/honorarios/calculo?desde=" + hoy + "&hasta=" + hoy
	espera(t, llamar(t, "GET", calc+"&odontologoId="+jorge, odo, nil), 403, "un odontólogo no ve los honorarios de otro")
	c := llamar(t, "GET", calc, odo, nil)
	espera(t, c, 200, "mis honorarios")
	lin := map[string]map[string]any{}
	for _, x := range c.Body["lineas"].([]any) {
		m := x.(map[string]any)
		lin[m["procedimientoRealizadoId"].(string)] = m
	}
	if l := lin[pro]; l == nil || l["honorario"] != "14.00" || l["materiales"] != "0.58" || !strings.Contains(l["regla"].(string), "40 % de lo cobrado") {
		t.Errorf("profilaxis: 40 %% de 35.00 = 14.00: %v", l)
	}
	if l := lin[end]; l == nil || l["monto"] != "230.00" || l["laboratorio"] != "30.00" || l["materiales"] != "3.30" || l["base"] != "196.70" || l["honorario"] != "98.35" {
		t.Errorf("endodoncia: 50 %% de (230 − 30 − 3.30) = 98.35: %v", l)
	}
	if lin[pj] != nil {
		t.Error("lo de Jorge no aparece en la liquidación de Valeria")
	}
	cj := llamar(t, "GET", calc+"&odontologoId="+jorge, aud, nil)
	espera(t, cj, 200, "el auditor ve todos")
	if ls := cj.Body["lineas"].([]any); len(ls) != 1 || ls[0].(map[string]any)["honorario"] != "12.00" {
		t.Errorf("Jorge: monto fijo 12.00: %v", ls)
	}

	// Reglas: sólo el Auditor; valores válidos; sólo inserción.
	regla := map[string]any{"tipo": "PORCENTAJE", "valor": "150", "vigenteDesde": hoy}
	espera(t, llamar(t, "POST", "/api/honorarios/reglas", odo, regla), 403, "el odontólogo no crea reglas")
	espera(t, llamar(t, "POST", "/api/honorarios/reglas", aud, regla), 400, "porcentaje > 100")
	regla["valor"], regla["vigenteDesde"] = "45", "2099-01-01"
	espera(t, llamar(t, "POST", "/api/honorarios/reglas", aud, regla), 201, "regla futura")
	if c2 := llamar(t, "GET", calc, odo, nil); c2.Body["total"] != c.Body["total"] {
		t.Error("una regla futura no cambia el cálculo de hoy")
	}
	if _, err := pool.Exec(ctx, `UPDATE reglas_honorario SET valor=1`); err == nil {
		t.Error("las reglas son de sólo inserción")
	}

	// Liquidación: sólo el Auditor; un cobro se liquida una vez.
	liq := map[string]string{"odontologoId": valeria, "desde": hoy, "hasta": hoy}
	espera(t, llamar(t, "POST", "/api/honorarios/liquidaciones", odo, liq), 403, "el odontólogo no se liquida a sí mismo")
	l := llamar(t, "POST", "/api/honorarios/liquidaciones", aud, liq)
	espera(t, l, 201, "liquidación")
	if l.Body["total"] != c.Body["total"] || len(l.Body["lineas"].([]any)) != len(c.Body["lineas"].([]any)) {
		t.Errorf("la liquidación copia la vista previa: %v vs %v", l.Body["total"], c.Body["total"])
	}
	espera(t, llamar(t, "POST", "/api/honorarios/liquidaciones", aud, liq), 409, "nada pendiente")
	if c3 := llamar(t, "GET", calc, odo, nil); len(c3.Body["lineas"].([]any)) != 0 {
		t.Error("lo liquidado ya no está pendiente")
	}
	id := l.Body["id"].(string)
	espera(t, llamar(t, "GET", "/api/honorarios/liquidaciones/"+id, odo, nil), 200, "la odontóloga ve su liquidación")
	espera(t, llamar(t, "GET", "/api/honorarios/liquidaciones/"+id, ingresar(t, odontologo2), nil), 404, "otro odontólogo no la ve")
	if ls := llamar(t, "GET", "/api/honorarios/liquidaciones", odo, nil); len(ls.Lista) == 0 {
		t.Error("lista de mis liquidaciones")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM liquidacion_lineas WHERE liquidacion_id=$1`, id); err == nil {
		t.Error("una liquidación no se modifica")
	}
}

func hoyLocal() string { return time.Now().In(ctl.Cfg.ZonaHoraria).Format("2006-01-02") }

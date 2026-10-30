package pruebas

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// Etapa 4 — presupuestos (RF-07, RF-08), consentimientos (RF-06), recetas y archivos.

const odontologo2 = "odontologo2@demo.sigo"

func pacienteDoc(t *testing.T, doc string) string {
	return idDe(t, `SELECT id FROM pacientes WHERE numero_documento=$1`, doc)
}

func procID(t *testing.T, codigo string) string {
	return idDe(t, `SELECT id FROM procedimientos WHERE codigo=$1`, codigo)
}

func itemsDemo(t *testing.T) []map[string]any {
	return []map[string]any{
		{"fase": 1, "faseNombre": "Higiene", "sesion": 1, "procedimientoId": procID(t, "PRO-01"), "honorario": "35.00"},
		// 10.00 sobre 45.00 = 22 % > tope del 10 % ⇒ requiere autorización de otra persona.
		{"fase": 2, "faseNombre": "Operatoria", "sesion": 2, "procedimientoId": procID(t, "RES-01"), "diente": 16, "superficies": []string{"o", "M"}, "honorario": "45.00", "descuento": "10.00"},
		{"fase": 3, "faseNombre": "Endodoncia", "sesion": 3, "procedimientoId": procID(t, "END-01"), "diente": 25, "honorario": "180.00", "costoLaboratorio": "20.00"},
	}
}

func TestPresupuestoFasesDescuentoYAprobacionParcial(t *testing.T) {
	requiereBD(t)
	pac := pacienteDoc(t, "DEMO000106")
	odo := ingresar(t, odontologo)
	rec := ingresar(t, recepcion)
	base := "/api/pacientes/" + pac + "/presupuestos"

	espera(t, llamar(t, "POST", base, rec, map[string]any{"titulo": "Plan", "items": itemsDemo(t)}), 403, "recepción no elabora presupuestos")
	malo := itemsDemo(t)
	malo[0]["descuento"] = "40.00" // descuento > honorario
	malo[1]["diente"] = 19
	r := llamar(t, "POST", base, odo, map[string]any{"titulo": "Plan", "items": malo})
	espera(t, r, 400, "validación")
	campos := r.Body["extra"].(map[string]any)["campos"].(map[string]any)
	if campos["items[0].descuento"] == nil || campos["items[1].diente"] == nil {
		t.Errorf("errores por campo: %v", campos)
	}

	r = llamar(t, "POST", base, odo, map[string]any{"titulo": "Plan integral", "items": itemsDemo(t)})
	espera(t, r, 201, "crear")
	id := r.Body["id"].(string)
	tot := r.Body["totales"].(map[string]any)
	if r.Body["estado"] != "BORRADOR" || tot["honorarios"] != "260.00" || tot["laboratorio"] != "20.00" || tot["descuentos"] != "10.00" || tot["total"] != "270.00" {
		t.Errorf("totales: %v", tot)
	}
	fases := tot["fases"].([]any)
	if len(fases) != 3 || fases[1].(map[string]any)["total"] != "35.00" || fases[2].(map[string]any)["total"] != "200.00" {
		t.Errorf("totales por fase: %v", fases)
	}
	if r.Body["descuentosPendientes"].(float64) != 1 {
		t.Errorf("un descuento pendiente de autorización: %v", r.Body["descuentosPendientes"])
	}

	pres := "/api/presupuestos/" + id
	r = llamar(t, "POST", pres+"/presentar", odo, nil)
	espera(t, r, 409, "presentar con descuento sin autorizar")
	if r.Body["code"] != "DESCUENTO_SIN_AUTORIZAR" {
		t.Errorf("código: %v", r.Body["code"])
	}
	r = llamar(t, "POST", pres+"/autorizar-descuentos", odo, map[string]string{"codigo": codigoVigente(t, odontologo)})
	espera(t, r, 403, "cuatro ojos: quien elaboró no autoriza")
	espera(t, llamar(t, "POST", pres+"/autorizar-descuentos", rec, map[string]string{"codigo": codigoVigente(t, recepcion)}), 403, "recepción no autoriza descuentos")
	odo2 := ingresar(t, odontologo2)
	espera(t, llamar(t, "POST", pres+"/autorizar-descuentos", odo2, map[string]string{"codigo": "000000"}), 400, "código MFA incorrecto")
	r = llamar(t, "POST", pres+"/autorizar-descuentos", odo2, map[string]string{"codigo": codigoVigente(t, odontologo2)})
	espera(t, r, 200, "otro odontólogo autoriza")
	if r.Body["descuentosPendientes"].(float64) != 0 || ultimaAuditoria(t, "DESCUENTO_AUTORIZADO")["_entidad"] != id {
		t.Errorf("autorización: %v", r.Body["descuentosPendientes"])
	}

	r = llamar(t, "POST", pres+"/presentar", odo, nil)
	espera(t, r, 200, "presentar")
	if r.Body["estado"] != "PRESENTADO" || len(r.Body["hashPresentado"].(string)) != 64 || r.Body["integro"] != true {
		t.Errorf("presentado: %v %v %v", r.Body["estado"], r.Body["hashPresentado"], r.Body["integro"])
	}
	r = llamar(t, "PUT", pres, odo, map[string]any{"titulo": "Otro", "items": itemsDemo(t)})
	espera(t, r, 409, "presentado ya no se edita")
	if r.Body["code"] != "PRESUPUESTO_NO_EDITABLE" {
		t.Errorf("código: %v", r.Body["code"])
	}
	ctx := context.Background()
	for _, q := range []string{`UPDATE presupuesto_items SET honorario=1 WHERE presupuesto_id=$1`, `DELETE FROM presupuesto_items WHERE presupuesto_id=$1`,
		`UPDATE presupuestos SET titulo='x' WHERE id=$1`} {
		if _, err := pool.Exec(ctx, q, id); err == nil {
			t.Errorf("la BD debe impedir: %s", q)
		}
	}

	items := r.Body["items"]
	if items == nil {
		items = llamar(t, "GET", pres, odo, nil).Body["items"]
	}
	its := items.([]any)
	itemID := func(i int) string { return its[i].(map[string]any)["id"].(string) }
	espera(t, llamar(t, "POST", pres+"/decision", rec, map[string]any{"decisiones": []map[string]any{{"itemId": itemID(0), "aprobado": true}}}), 400, "decisión incompleta")
	r = llamar(t, "POST", pres+"/decision", rec, map[string]any{"nota": "El paciente pospone la resina.", "decisiones": []map[string]any{
		{"itemId": itemID(0), "aprobado": true}, {"itemId": itemID(1), "aprobado": false}, {"itemId": itemID(2), "aprobado": true}}})
	espera(t, r, 200, "aprobación parcial")
	if r.Body["estado"] != "APROBADO_PARCIAL" || r.Body["totales"].(map[string]any)["aprobado"] != "235.00" {
		t.Errorf("parcial: %v %v", r.Body["estado"], r.Body["totales"])
	}
	espera(t, llamar(t, "POST", pres+"/decision", rec, map[string]any{"decisiones": []map[string]any{}}), 409, "no se decide dos veces")

	// Tareas agendables: sólo las aprobadas.
	tr := llamar(t, "GET", "/api/pacientes/"+pac+"/tareas", rec, nil)
	espera(t, tr, 200, "tareas")
	var endo map[string]any
	deEste := 0
	for _, x := range tr.Lista {
		m := x.(map[string]any)
		if m["presupuestoId"] == id {
			deEste++
		}
		if m["presupuestoId"] == id && m["codigo"] == "END-01" {
			endo = m
		}
		if m["presupuestoId"] == id && m["codigo"] == "RES-01" {
			t.Error("el ítem rechazado no genera tarea")
		}
	}
	if deEste != 2 {
		t.Errorf("aprobar 2 de 3 ítems crea exactamente 2 tareas: %d", deEste)
	}
	if endo == nil || endo["estado"] != "PENDIENTE" {
		t.Fatalf("tarea de endodoncia: %v", tr.Lista)
	}
	tareaID := endo["id"].(string)

	// La cita se agenda desde la tarea: hereda el procedimiento.
	i := cargarIDs(t)
	dia := diaPrueba.Add(1)
	cita := map[string]any{"pacienteId": pac, "odontologoId": i.odontologo, "sillonId": i.sillonA, "inicio": horario(dia, 13, 0), "duracionMin": 90, "tareaId": tareaID}
	otra := pacienteDoc(t, "DEMO000101")
	cita["pacienteId"] = otra
	espera(t, llamar(t, "POST", "/api/citas", rec, cita), 400, "la tarea es de otro paciente")
	cita["pacienteId"] = pac
	c := llamar(t, "POST", "/api/citas", rec, cita)
	espera(t, c, 201, "cita desde la tarea")
	if c.Body["procedimientoId"] != procID(t, "END-01") {
		t.Errorf("la cita hereda el procedimiento: %v", c.Body["procedimientoId"])
	}
	cita["inicio"] = horario(dia, 16, 0)
	espera(t, llamar(t, "POST", "/api/citas", rec, cita), 400, "la tarea ya está agendada")
	if estadoTarea(t, rec, pac, tareaID) != "AGENDADA" {
		t.Error("la tarea pasa a AGENDADA")
	}

	// Al registrarla como realizada, la tarea queda REALIZADA (diente y procedimiento heredados).
	rz := llamar(t, "POST", "/api/pacientes/"+pac+"/procedimientos", odo, map[string]any{"tareaId": tareaID, "observaciones": "Conductometría y obturación.", "codigo": codigoVigente(t, odontologo)})
	espera(t, rz, 201, "realizado desde la tarea")
	if rz.Body["codigo"] != "END-01" || rz.Body["diente"].(float64) != 25 {
		t.Errorf("hereda de la tarea: %v", rz.Body)
	}
	if estadoTarea(t, rec, pac, tareaID) != "REALIZADA" {
		t.Error("la tarea pasa a REALIZADA")
	}
	r = llamar(t, "POST", "/api/pacientes/"+pac+"/procedimientos", odo, map[string]any{"tareaId": tareaID, "codigo": codigoVigente(t, odontologo)})
	espera(t, r, 400, "no se realiza dos veces")

	// Bandeja del módulo y lectura por el auditor (sin permiso).
	b := llamar(t, "GET", "/api/presupuestos?estado=APROBADO_PARCIAL", rec, nil)
	espera(t, b, 200, "bandeja")
	if len(b.Lista) == 0 {
		t.Error("la bandeja filtra por estado")
	}
	espera(t, llamar(t, "GET", "/api/presupuestos?estado=XX", rec, nil), 400, "estado inválido")
	espera(t, llamar(t, "GET", pres, ingresar(t, auditor), nil), 403, "el auditor no ve presupuestos")
}

func estadoTarea(t *testing.T, tok, pac, id string) string {
	t.Helper()
	for _, x := range llamar(t, "GET", "/api/pacientes/"+pac+"/tareas", tok, nil).Lista {
		if m := x.(map[string]any); m["id"] == id {
			return m["estado"].(string)
		}
	}
	return ""
}

func TestPresupuestoRechazadoYBorradorEditable(t *testing.T) {
	requiereBD(t)
	pac := pacienteDoc(t, "DEMO000107")
	odo := ingresar(t, odontologo)
	base := "/api/pacientes/" + pac + "/presupuestos"
	espera(t, llamar(t, "POST", base, odo, map[string]any{"titulo": "Vacío", "items": []any{}}), 201, "borrador vacío")
	r := llamar(t, "GET", base, odo, nil)
	id := r.Lista[0].(map[string]any)["id"].(string)
	espera(t, llamar(t, "POST", "/api/presupuestos/"+id+"/presentar", odo, nil), 400, "no se presenta vacío")
	items := itemsDemo(t)[:1]
	u := llamar(t, "PUT", "/api/presupuestos/"+id, odo, map[string]any{"titulo": "Sólo profilaxis", "items": items})
	espera(t, u, 200, "editar borrador")
	if u.Body["titulo"] != "Sólo profilaxis" || len(u.Body["items"].([]any)) != 1 {
		t.Errorf("borrador editado: %v", u.Body)
	}
	espera(t, llamar(t, "POST", "/api/presupuestos/"+id+"/autorizar-descuentos", ingresar(t, odontologo2), map[string]string{"codigo": codigoVigente(t, odontologo2)}), 409, "sin descuentos pendientes")
	espera(t, llamar(t, "POST", "/api/presupuestos/"+id+"/presentar", odo, nil), 200, "presentar")
	it := llamar(t, "GET", "/api/presupuestos/"+id, odo, nil).Body["items"].([]any)[0].(map[string]any)["id"]
	r = llamar(t, "POST", "/api/presupuestos/"+id+"/decision", odo, map[string]any{"decisiones": []map[string]any{{"itemId": it, "aprobado": false}}})
	espera(t, r, 200, "rechazo")
	if r.Body["estado"] != "RECHAZADO" || r.Body["totales"].(map[string]any)["aprobado"] != "0.00" {
		t.Errorf("rechazado: %v", r.Body["estado"])
	}
	for _, x := range llamar(t, "GET", "/api/pacientes/"+pac+"/tareas", odo, nil).Lista {
		if x.(map[string]any)["presupuestoId"] == id {
			t.Error("un presupuesto rechazado no genera tareas")
		}
	}
}

// ---------------------------------------------------------------- Consentimientos

// firmaPNG: un trazo "a mano" con ruido para que no quede vacía.
func firmaPNG(t *testing.T, ancho, alto int, trazo bool) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, ancho, alto))
	if trazo {
		for x := 10; x < ancho-10; x++ {
			y := alto/2 + (x*7%23 - 11)
			for d := -1; d <= 1; d++ {
				img.Set(x, y+d, color.RGBA{0, 0, 80, 255})
			}
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
}

func plantilla(t *testing.T, tok, codigo string) string {
	t.Helper()
	for _, x := range llamar(t, "GET", "/api/consentimientos/plantillas", tok, nil).Lista {
		if m := x.(map[string]any); m["codigo"] == codigo {
			return m["id"].(string)
		}
	}
	t.Fatalf("sin plantilla %s", codigo)
	return ""
}

// vista: pide la vista previa y devuelve el cuerpo para firmar con su hashDocumento.
func vista(t *testing.T, tok, pac string, cuerpo map[string]any) map[string]any {
	t.Helper()
	r := llamar(t, "POST", "/api/pacientes/"+pac+"/consentimientos/vista-previa", tok, map[string]any{"plantillaId": cuerpo["plantillaId"], "procedimientoId": cuerpo["procedimientoId"]})
	espera(t, r, 200, "vista previa")
	cuerpo["hashDocumento"] = r.Body["hashDocumento"]
	return cuerpo
}

func TestConsentimientoFirmaManuscrita(t *testing.T) {
	requiereBD(t)
	ana := pacienteDoc(t, "DEMO000101")
	odo := ingresar(t, odontologo)
	ruta := "/api/pacientes/" + ana + "/consentimientos"
	exo := plantilla(t, odo, "EXO-01")
	espera(t, llamar(t, "POST", ruta, ingresar(t, recepcion), map[string]any{"plantillaId": exo, "metodo": "MANUSCRITA", "firmaImagen": firmaPNG(t, 300, 100, true)}), 403, "recepción no gestiona consentimientos")
	espera(t, llamar(t, "POST", ruta, odo, vista(t, odo, ana, map[string]any{"plantillaId": exo, "metodo": "MANUSCRITA", "firmaImagen": firmaPNG(t, 4, 4, false)})), 400, "firma vacía")
	espera(t, llamar(t, "POST", ruta, odo, map[string]any{"plantillaId": exo, "metodo": "MANUSCRITA", "firmaImagen": "data:image/png;base64,SG9sYQ=="}), 400, "no es PNG")
	espera(t, llamar(t, "POST", ruta, odo, map[string]any{"plantillaId": exo, "metodo": "BIOMETRICA"}), 400, "método inválido")

	sinVista := map[string]any{"plantillaId": exo, "metodo": "MANUSCRITA", "firmaImagen": firmaPNG(t, 400, 150, true)}
	espera(t, llamar(t, "POST", ruta, odo, sinVista), 400, "sin vista previa no se firma")
	sinVista["hashDocumento"] = strings.Repeat("a", 64)
	r := llamar(t, "POST", ruta, odo, sinVista)
	espera(t, r, 409, "el documento mostrado no coincide")
	if r.Body["code"] != "DOCUMENTO_CAMBIO" {
		t.Errorf("código: %v", r.Body["code"])
	}
	pv := llamar(t, "POST", ruta+"/vista-previa", odo, map[string]any{"plantillaId": exo})
	if !strings.Contains(pv.Body["texto"].(string), "Exodoncia simple") || pv.Body["firmanteRelacion"] != "PACIENTE" {
		t.Errorf("vista previa: %v", pv.Body)
	}
	r = llamar(t, "POST", ruta, odo, vista(t, odo, ana, map[string]any{"plantillaId": exo, "metodo": "MANUSCRITA", "firmaImagen": firmaPNG(t, 400, 150, true)}))
	espera(t, r, 201, "firma manuscrita")
	if r.Body["hashDocumento"] != pv.Body["hashDocumento"] {
		t.Error("se firma exactamente el texto de la vista previa")
	}
	txt := r.Body["texto"].(string)
	if r.Body["estado"] != "FIRMADO" || r.Body["integro"] != true || r.Body["firmanteRelacion"] != "PACIENTE" ||
		!strings.Contains(txt, "Ana Lucía Torres Vera") || !strings.Contains(txt, "Exodoncia simple") || strings.Contains(txt, "{{") {
		t.Errorf("consentimiento: %v %v %v\n%s", r.Body["estado"], r.Body["integro"], r.Body["firmanteRelacion"], txt)
	}
	if !strings.HasPrefix(r.Body["firmaImagen"].(string), "data:image/png;base64,") || len(r.Body["hashFirma"].(string)) != 64 {
		t.Error("devuelve la imagen y el hash")
	}
	id := r.Body["id"].(string)
	ctx := context.Background()
	for _, q := range []string{`UPDATE consentimientos SET texto='otro' WHERE id=$1`, `DELETE FROM consentimientos WHERE id=$1`} {
		if _, err := pool.Exec(ctx, q, id); err == nil {
			t.Errorf("la BD debe impedir: %s", q)
		}
	}
	l := llamar(t, "GET", ruta, ingresar(t, asistente), nil)
	espera(t, l, 200, "el asistente consulta (clinica.leer)")
	if len(l.Lista) == 0 || l.Lista[0].(map[string]any)["firmaImagen"] != nil {
		t.Error("el listado no incluye la imagen (sólo el detalle)")
	}
	d := llamar(t, "GET", "/api/consentimientos/"+id, odo, nil)
	espera(t, d, 200, "detalle")
	if d.Body["integro"] != true || d.Body["firmaImagen"] == nil {
		t.Errorf("detalle: %v", d.Body["integro"])
	}
}

func TestConsentimientoOTPYTutor(t *testing.T) {
	requiereBD(t)
	sofia := pacienteDoc(t, "DEMO000103") // menor de edad: firma la tutora
	odo := ingresar(t, odontologo)
	gen := plantilla(t, odo, "GEN-01")
	ruta := "/api/pacientes/" + sofia + "/consentimientos"
	espera(t, llamar(t, "POST", ruta, odo, map[string]any{"plantillaId": gen, "metodo": "OTP", "firmaImagen": firmaPNG(t, 300, 100, true)}), 400, "OTP sin imagen")
	r := llamar(t, "POST", ruta, odo, vista(t, odo, sofia, map[string]any{"plantillaId": gen, "metodo": "OTP", "procedimientoId": procID(t, "SEL-01")}))
	espera(t, r, 201, "OTP enviado")
	cod, _ := r.Body["otpSimuladoCodigo"].(string)
	if r.Body["estado"] != "PENDIENTE_OTP" || r.Body["firmanteRelacion"] != "TUTOR" || r.Body["firmanteNombre"] != "María Paz (ficticia)" ||
		!regexp.MustCompile(`^\d{6}$`).MatchString(cod) || r.Body["otpSimulado"] != true {
		t.Fatalf("pendiente OTP: %v", r.Body)
	}
	if dest, _ := r.Body["otpDestinatario"].(string); !strings.HasSuffix(dest, "0901") || strings.Contains(dest, "0000 0901") {
		t.Errorf("destinatario enmascarado: %q", dest)
	}
	if !strings.Contains(simulada.Enviados[len(simulada.Enviados)-1].Texto, cod) {
		t.Error("el código se envía por el proveedor (simulado)")
	}
	id := r.Body["id"].(string)
	if d := llamar(t, "GET", "/api/consentimientos/"+id, odo, nil); d.Body["otpSimuladoCodigo"] != nil {
		t.Error("el código no se vuelve a mostrar")
	}
	otro := "000000"
	if cod == otro {
		otro = "111111"
	}
	v := llamar(t, "POST", "/api/consentimientos/"+id+"/verificar-otp", odo, map[string]string{"codigo": otro})
	espera(t, v, 400, "código incorrecto")
	if v.Body["code"] != "OTP_INVALIDO" || !strings.Contains(v.Body["message"].(string), "4") {
		t.Errorf("intentos restantes: %v", v.Body)
	}
	v = llamar(t, "POST", "/api/consentimientos/"+id+"/verificar-otp", odo, map[string]string{"codigo": cod})
	espera(t, v, 200, "firma OTP")
	if v.Body["estado"] != "FIRMADO" || v.Body["integro"] != true || !strings.Contains(v.Body["texto"].(string), "Sellantes") {
		t.Errorf("firmado: %v", v.Body)
	}
	espera(t, llamar(t, "POST", "/api/consentimientos/"+id+"/verificar-otp", odo, map[string]string{"codigo": cod}), 409, "ya firmado")

	// Bloqueo a los 5 intentos, aunque después llegue el código correcto.
	r = llamar(t, "POST", ruta, odo, vista(t, odo, sofia, map[string]any{"plantillaId": gen, "metodo": "OTP"}))
	id2, cod2 := r.Body["id"].(string), r.Body["otpSimuladoCodigo"].(string)
	malo := "999999"
	if cod2 == malo {
		malo = "888888"
	}
	for range 5 {
		llamar(t, "POST", "/api/consentimientos/"+id2+"/verificar-otp", odo, map[string]string{"codigo": malo})
	}
	v = llamar(t, "POST", "/api/consentimientos/"+id2+"/verificar-otp", odo, map[string]string{"codigo": cod2})
	espera(t, v, 409, "bloqueado")
	if v.Body["code"] != "OTP_BLOQUEADO" {
		t.Errorf("código: %v", v.Body["code"])
	}
	p := llamar(t, "GET", "/api/consentimientos?estado=PENDIENTE_OTP", odo, nil)
	if len(p.Lista) == 0 {
		t.Error("bandeja de pendientes")
	}
}

func TestPlantillasVersionadas(t *testing.T) {
	requiereBD(t)
	adm := ingresar(t, admin)
	odo := ingresar(t, odontologo)
	texto := "Yo, {{firmante}}, autorizo la atención de {{paciente}} en {{clinica}}. Texto de ejemplo versión dos, sin validez legal."
	espera(t, llamar(t, "POST", "/api/consentimientos/plantillas", odo, map[string]any{"codigo": "GEN-01", "titulo": "x", "texto": texto}), 403, "el odontólogo no edita plantillas")
	espera(t, llamar(t, "POST", "/api/consentimientos/plantillas", adm, map[string]any{"codigo": "g 1", "titulo": "x", "texto": "corto"}), 400, "validación")
	antes := plantilla(t, adm, "GEN-01")
	r := llamar(t, "POST", "/api/consentimientos/plantillas", adm, map[string]any{"codigo": "gen-01", "titulo": "Consentimiento general v2", "texto": texto})
	espera(t, r, 201, "nueva versión")
	if r.Body["version"].(float64) < 2 || r.Body["codigo"] != "GEN-01" {
		t.Errorf("versión: %v", r.Body)
	}
	if plantilla(t, odo, "GEN-01") == antes {
		t.Error("la lista muestra la última versión")
	}
	if _, err := pool.Exec(context.Background(), `UPDATE consentimiento_plantillas SET texto='x' WHERE id=$1`, antes); err == nil {
		t.Error("las versiones anteriores no cambian")
	}
}

// ---------------------------------------------------------------- Recetas

func receta(meds ...string) []map[string]string {
	out := []map[string]string{}
	for _, m := range meds {
		out = append(out, map[string]string{"medicamento": m, "presentacion": "Tabletas 500 mg", "dosis": "1 tableta", "via": "Oral", "frecuencia": "Cada 8 horas", "duracion": "5 días"})
	}
	return out
}

func TestRecetasConAlertasYFirma(t *testing.T) {
	requiereBD(t)
	ana := pacienteDoc(t, "DEMO000101") // alergia a penicilina
	odo := ingresar(t, odontologo)
	ruta := "/api/pacientes/" + ana + "/recetas"
	espera(t, llamar(t, "POST", ruta, ingresar(t, asistente), map[string]any{"items": receta("Paracetamol")}), 403, "el asistente no receta")
	espera(t, llamar(t, "POST", ruta, odo, map[string]any{"items": []any{}}), 400, "sin medicamentos")

	r := llamar(t, "POST", ruta, odo, map[string]any{"items": receta("Amoxicilina + ácido clavulánico", "Paracetamol"), "codigo": codigoVigente(t, odontologo)})
	espera(t, r, 409, "alerta de alergia")
	al := r.Body["extra"].(map[string]any)["alertas"].([]any)
	if r.Body["code"] != "ALERTA_ALERGIA" || len(al) != 1 || !strings.Contains(strings.ToLower(al[0].(map[string]any)["medicamento"].(string)), "amoxicilina") {
		t.Fatalf("alertas: %v", r.Body)
	}
	espera(t, llamar(t, "POST", ruta, odo, map[string]any{"items": receta("Amoxicilina"), "confirmarAlertas": true, "codigo": "000000"}), 400, "firma con código incorrecto")
	r = llamar(t, "POST", ruta, odo, map[string]any{"items": receta("Amoxicilina"), "confirmarAlertas": true, "indicaciones": "Control en 7 días.", "codigo": codigoVigente(t, odontologo)})
	espera(t, r, 201, "receta con alerta confirmada")
	if r.Body["integra"] != true || len(r.Body["alertasConfirmadas"].([]any)) != 1 || len(r.Body["hashFirma"].(string)) != 64 {
		t.Errorf("receta: %v", r.Body)
	}
	id := r.Body["id"].(string)
	if _, err := pool.Exec(context.Background(), `UPDATE receta_items SET dosis='2 tabletas' WHERE receta_id=$1`, id); err == nil {
		t.Error("la receta emitida no cambia")
	}
	espera(t, llamar(t, "POST", ruta, odo, map[string]any{"items": receta("Paracetamol"), "codigo": codigoVigente(t, odontologo)}), 201, "sin alertas no pide confirmación")

	carlos := pacienteDoc(t, "DEMO000102") // anticoagulado
	r = llamar(t, "POST", "/api/pacientes/"+carlos+"/recetas", odo, map[string]any{"items": receta("Ibuprofeno"), "codigo": codigoVigente(t, odontologo)})
	espera(t, r, 409, "AINE en paciente anticoagulado")
	luis := pacienteDoc(t, "DEMO000104")
	espera(t, llamar(t, "POST", "/api/pacientes/"+luis+"/recetas", odo, map[string]any{"items": receta("Paracetamol"), "codigo": codigoVigente(t, odontologo)}), 409, "anamnesis pendiente")

	l := llamar(t, "GET", ruta, ingresar(t, asistente), nil)
	espera(t, l, 200, "listar")
	if len(l.Lista) < 2 || l.Lista[0].(map[string]any)["integra"] != true {
		t.Errorf("recetas: %v", l.Lista)
	}
}

// ---------------------------------------------------------------- Archivos

func subir(t *testing.T, ruta, tok, nombre string, datos []byte, categoria string) resp {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	_ = w.WriteField("categoria", categoria)
	_ = w.WriteField("descripcion", "Prueba (ficticia)")
	if datos != nil {
		f, _ := w.CreateFormFile("archivo", nombre)
		_, _ = f.Write(datos)
	}
	_ = w.Close()
	req := httptest.NewRequest("POST", ruta, &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out := resp{Status: res.StatusCode}
	body, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(body, &out.Body)
	return out
}

func TestArchivosConIntegridad(t *testing.T) {
	requiereBD(t)
	ana := pacienteDoc(t, "DEMO000101")
	asis := ingresar(t, asistente)
	ruta := "/api/pacientes/" + ana + "/archivos"
	pngB, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(firmaPNG(t, 200, 80, true), "data:image/png;base64,"))

	espera(t, subir(t, ruta, ingresar(t, recepcion), "rx.png", pngB, "RADIOGRAFIA"), 403, "recepción no sube al expediente")
	espera(t, subir(t, ruta, asis, "rx.png", nil, "RADIOGRAFIA"), 400, "sin archivo")
	espera(t, subir(t, ruta, asis, "rx.png", pngB, "VIDEO"), 400, "categoría inválida")
	r := subir(t, ruta, asis, "falso.jpg", []byte("MZ esto no es una imagen"), "FOTOGRAFIA")
	espera(t, r, 400, "contenido no permitido aunque la extensión sea .jpg")

	r = subir(t, ruta, asis, `C:\fotos\rx "panorámica".png`, pngB, "RADIOGRAFIA")
	espera(t, r, 201, "subir")
	if r.Body["tipoMime"] != "image/png" || len(r.Body["sha256"].(string)) != 64 || r.Body["nombre"] != `rx "panorámica".png` {
		t.Errorf("archivo: %v", r.Body)
	}
	id := r.Body["id"].(string)
	l := llamar(t, "GET", ruta, asis, nil)
	if len(l.Lista) == 0 {
		t.Error("listar")
	}

	req := httptest.NewRequest("GET", "/api/archivos/"+id+"/contenido", nil)
	req.Header.Set("Authorization", "Bearer "+asis)
	res, _ := app.Test(req, -1)
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !bytes.Equal(b, pngB) || res.Header.Get("Content-Type") != "image/png" || res.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("descarga: %d %s %s", res.StatusCode, res.Header.Get("Content-Type"), res.Header.Get("Cache-Control"))
	}
	if ultimaAuditoria(t, "LECTURA")["paciente"] != ana {
		t.Error("la descarga se audita")
	}
	espera(t, llamar(t, "GET", "/api/archivos/"+id+"/contenido", ingresar(t, recepcion), nil), 403, "recepción no descarga")

	almacen.Alterar(ana+"/"+id, append([]byte{}, append(pngB, 0)...))
	a := llamar(t, "GET", "/api/archivos/"+id+"/contenido", asis, nil)
	espera(t, a, 409, "archivo alterado fuera del sistema")
	if a.Body["code"] != "ARCHIVO_ALTERADO" {
		t.Errorf("código: %v", a.Body["code"])
	}
	if _, err := pool.Exec(context.Background(), `UPDATE archivos_paciente SET sha256='x' WHERE id=$1`, id); err == nil {
		t.Error("el registro del archivo no cambia")
	}
}

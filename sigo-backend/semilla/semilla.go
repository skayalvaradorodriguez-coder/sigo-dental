// Package semilla carga datos de demostración — TODOS FICTICIOS.
//   - Usuarios: crea los que falten (uno por rol) con la contraseña demo y MFA activo
//     (secreto aleatorio; código vigente con `go run . totp <correo>`). Los existentes NO se
//     modifican, salvo con `go run . semilla --reset-usuarios`.
//   - Clínica (Etapa 2): sólo si aún no hay sedes.
//   - Expediente (Etapa 3): odontograma, periodontograma y una nota en borrador de Ana Torres,
//     sólo si aún no hay registros de odontograma.
//   - Planes (Etapa 4): plantillas de consentimiento (si no hay ninguna) y un presupuesto en
//     borrador para Ana Torres (si no hay presupuestos). Textos de EJEMPLO, sin validez legal:
//     cada clínica debe usar los que apruebe su asesoría jurídica.
//   - Caja (Etapa 5): una caja de ayer, ya cerrada, con un faltante de 1.00 en efectivo para que
//     el Auditor la concilie (sólo si aún no hay cajas). Comprobantes del proveedor SIMULADO.
//   - Inventario y honorarios (Etapa 6): ubicaciones, 12 insumos con stock inicial, recetas de
//     consumo y tres reglas de honorarios de ejemplo (sólo si aún no hay ubicaciones). Costos ficticios.
package semilla

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sigo-backend/domain"
	"sigo-backend/facturacion"
	"sigo-backend/notificaciones"
	"sigo-backend/seguridad"
)

// PasswordDemo: contraseña pública de los usuarios FICTICIOS de demostración (la semilla no corre en
// producción y obliga a cambiarla sólo al usuario de primer ingreso).
const PasswordDemo = "Demo-Sigo-2026" // #nosec G101 -- credencial de demostración documentada, no un secreto

type UsuarioDemo struct {
	Email, Nombre, Rol, Registro string
	PrimerIngreso                bool
}

var Usuarios = []UsuarioDemo{
	{"admin@demo.sigo", "Andrés Molina (ficticio)", "ADMIN_TI", "", false},
	{"recepcion@demo.sigo", "Carla Benítez (ficticio)", "RECEPCIONISTA", "", false},
	{"odontologo@demo.sigo", "Dra. Valeria Ríos (ficticio)", "ODONTOLOGO", "REG-DEMO-0001", false},
	{"odontologo2@demo.sigo", "Dr. Jorge Andrade (ficticio)", "ODONTOLOGO", "REG-DEMO-0002", false},
	{"asistente@demo.sigo", "Mateo Cárdenas (ficticio)", "ASISTENTE", "", false},
	{"auditor@demo.sigo", "Patricia Lozano (ficticio)", "AUDITOR", "", false},
	// Para demostrar el primer ingreso: cambio de contraseña + enrolamiento MFA con QR.
	{"nuevo@demo.sigo", "Usuario Nuevo (ficticio)", "RECEPCIONISTA", "", true},
}

func Cargar(ctx context.Context, pool *pgxpool.Pool, resetUsuarios bool, zona *time.Location) error {
	hash, err := seguridad.HashPassword(PasswordDemo)
	if err != nil {
		return err
	}
	for _, u := range Usuarios {
		var reg, secreto *string
		if u.Registro != "" {
			reg = &u.Registro
		}
		if !u.PrimerIngreso {
			s := seguridad.GenerarSecretoTOTP()
			secreto = &s
		}
		var existe bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM usuarios WHERE email=$1)`, u.Email).Scan(&existe); err != nil {
			return err
		}
		switch {
		case !existe:
			_, err = pool.Exec(ctx, `INSERT INTO usuarios (email, nombre, rol, registro_profesional, password_hash, debe_cambiar_password, mfa_activo, mfa_secreto)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, u.Email, u.Nombre, u.Rol, reg, hash, u.PrimerIngreso, !u.PrimerIngreso, secreto)
		case resetUsuarios:
			_, err = pool.Exec(ctx, `UPDATE usuarios SET nombre=$2, rol=$3, registro_profesional=$4, password_hash=$5, debe_cambiar_password=$6,
				mfa_activo=$7, mfa_secreto=$8, mfa_ultimo_paso=NULL, intentos_fallidos=0, bloqueado_hasta=NULL, activo=TRUE, actualizado_en=now()
				WHERE email=$1`, u.Email, u.Nombre, u.Rol, reg, hash, u.PrimerIngreso, !u.PrimerIngreso, secreto)
		}
		if err != nil {
			return err
		}
	}
	if err := cargarClinica(ctx, pool, zona); err != nil {
		return err
	}
	if err := cargarExpediente(ctx, pool); err != nil {
		return err
	}
	if err := cargarPlanes(ctx, pool); err != nil {
		return err
	}
	if err := cargarCaja(ctx, pool); err != nil {
		return err
	}
	return cargarInventario(ctx, pool)
}

func idUsuario(ctx context.Context, pool *pgxpool.Pool, email string) (id string) {
	_ = pool.QueryRow(ctx, `SELECT id FROM usuarios WHERE email=$1`, email).Scan(&id)
	return
}

func cargarClinica(ctx context.Context, pool *pgxpool.Pool, zona *time.Location) error {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sedes`).Scan(&n); err != nil || n > 0 {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	recep := idUsuario(ctx, pool, "recepcion@demo.sigo")
	valeria := idUsuario(ctx, pool, "odontologo@demo.sigo")
	jorge := idUsuario(ctx, pool, "odontologo2@demo.sigo")
	asistente := idUsuario(ctx, pool, "asistente@demo.sigo")

	var sede, c1, c2, s1a, s1b, s2a string
	if err := tx.QueryRow(ctx, `INSERT INTO sedes (nombre, direccion) VALUES ('Sede Central (ficticia)', 'Av. Ejemplo 123') RETURNING id`).Scan(&sede); err != nil {
		return err
	}
	_ = tx.QueryRow(ctx, `INSERT INTO consultorios (sede_id, nombre) VALUES ($1,'Consultorio 1') RETURNING id`, sede).Scan(&c1)
	_ = tx.QueryRow(ctx, `INSERT INTO consultorios (sede_id, nombre) VALUES ($1,'Consultorio 2') RETURNING id`, sede).Scan(&c2)
	_ = tx.QueryRow(ctx, `INSERT INTO sillones (consultorio_id, nombre) VALUES ($1,'Sillón 1A') RETURNING id`, c1).Scan(&s1a)
	_ = tx.QueryRow(ctx, `INSERT INTO sillones (consultorio_id, nombre) VALUES ($1,'Sillón 1B') RETURNING id`, c1).Scan(&s1b)
	_ = tx.QueryRow(ctx, `INSERT INTO sillones (consultorio_id, nombre) VALUES ($1,'Sillón 2A') RETURNING id`, c2).Scan(&s2a)

	procs := map[string]string{}
	for _, p := range []struct {
		cod, nom string
		min      int
		ar       string
	}{{"VAL-01", "Consulta de valoración", 30, "25.00"}, {"PRO-01", "Profilaxis", 45, "35.00"}, {"RES-01", "Restauración con resina", 60, "45.00"},
		{"END-01", "Endodoncia unirradicular", 90, "180.00"}, {"EXO-01", "Exodoncia simple", 45, "50.00"}, {"SEL-01", "Sellantes", 30, "20.00"}} {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO procedimientos (codigo, nombre, duracion_min, arancel) VALUES ($1,$2,$3,$4) RETURNING id`, p.cod, p.nom, p.min, p.ar).Scan(&id); err != nil {
			return err
		}
		procs[p.cod] = id
	}
	var convenio string
	_ = tx.QueryRow(ctx, `INSERT INTO convenios (nombre, aseguradora) VALUES ('Convenio Demo Salud (ficticio)', 'Aseguradora Ejemplo') RETURNING id`).Scan(&convenio)
	_, _ = tx.Exec(ctx, `INSERT INTO convenios (nombre, aseguradora) VALUES ('Seguro Dental Ejemplo (ficticio)', 'Seguros Demo')`)

	type pac struct {
		doc, nom, ape, nac, sexo, tel, email, canal string
		autoriza                                    bool
		convenio                                    bool
	}
	pacientes := []pac{
		{"DEMO000101", "Ana Lucía", "Torres Vera", "1988-03-12", "FEMENINO", "+000 0000 0101", "ana.torres@example.com", "WHATSAPP", true, false},
		{"DEMO000102", "Carlos Andrés", "Mendoza Ruiz", "1956-07-30", "MASCULINO", "+000 0000 0102", "", "SMS", true, true},
		{"DEMO000103", "Sofía Valentina", "Jaramillo Paz", "2016-09-05", "FEMENINO", "+000 0000 0103", "", "WHATSAPP", true, false},
		{"DEMO000104", "Luis Fernando", "Quiroz Mora", "1979-11-21", "MASCULINO", "+000 0000 0104", "", "WHATSAPP", true, false},
		{"DEMO000105", "Gabriela Estefanía", "Ortiz León", "1995-01-08", "FEMENINO", "+000 0000 0105", "", "WHATSAPP", false, false},
		{"DEMO000106", "Jorge Enrique", "Salazar Vega", "1969-04-17", "MASCULINO", "+000 0000 0106", "", "WHATSAPP", true, true},
		{"DEMO000107", "Daniela Alejandra", "Romero Castro", "2001-06-25", "FEMENINO", "", "daniela.romero@example.com", "EMAIL", true, false},
		{"DEMO000108", "Mateo Sebastián", "Cevallos Ibarra", "2010-02-14", "MASCULINO", "+000 0000 0108", "", "WHATSAPP", true, false},
	}
	ids := map[string]string{}
	for _, p := range pacientes {
		var tel, email, conv *string
		if p.tel != "" {
			tel = &p.tel
		}
		if p.email != "" {
			email = &p.email
		}
		if p.convenio {
			conv = &convenio
		}
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO pacientes (tipo_documento, numero_documento, nombres, apellidos, fecha_nacimiento, sexo, telefono, email,
			emergencia_nombre, emergencia_telefono, emergencia_parentesco, canal_preferido, autoriza_mensajes, convenio_id, creado_por)
			VALUES ('CEDULA',$1,$2,$3,$4,$5,$6,$7,'Contacto de Emergencia (ficticio)','+000 0000 0000','Familiar',$8,$9,$10,$11) RETURNING id`,
			p.doc, p.nom, p.ape, p.nac, p.sexo, tel, email, p.canal, p.autoriza, conv, recep).Scan(&id); err != nil {
			return err
		}
		ids[p.doc] = id
	}
	_, _ = tx.Exec(ctx, `INSERT INTO tutores (paciente_id, nombre, tipo_documento, numero_documento, parentesco, telefono) VALUES
		($1,'María Paz (ficticia)','CEDULA','DEMO000901','Madre','+000 0000 0901'), ($2,'Ricardo Cevallos (ficticio)','CEDULA','DEMO000902','Padre','+000 0000 0902')`,
		ids["DEMO000103"], ids["DEMO000108"])

	// Luis (DEMO000104) queda SIN anamnesis para mostrar el aviso "Anamnesis pendiente".
	type anam struct {
		doc            string
		enf            []string
		al             []domain.Alergia
		antico, emb    bool
		med, quir, por string
	}
	for _, a := range []anam{
		{"DEMO000101", []string{"Hipertensión arterial controlada"}, []domain.Alergia{{Sustancia: "Penicilina", Tipo: "MEDICAMENTO", Reaccion: "Urticaria generalizada"}}, false, false, "Losartán 50 mg/día", "", valeria},
		{"DEMO000102", []string{"Diabetes mellitus tipo 2", "Fibrilación auricular"}, nil, true, false, "Warfarina, metformina", "Colecistectomía (2015)", valeria},
		{"DEMO000103", nil, nil, false, false, "", "", asistente},
		{"DEMO000105", nil, []domain.Alergia{{Sustancia: "Látex", Tipo: "MATERIAL", Reaccion: "Dermatitis de contacto"}}, false, true, "", "", valeria},
		{"DEMO000106", nil, nil, false, false, "", "", valeria},
		{"DEMO000107", nil, []domain.Alergia{{Sustancia: "Maní", Tipo: "ALIMENTO"}}, false, false, "", "", valeria},
		{"DEMO000108", []string{"Asma leve"}, nil, false, false, "", "", asistente},
	} {
		if a.enf == nil {
			a.enf = []string{}
		}
		if a.al == nil {
			a.al = []domain.Alergia{}
		}
		enf, _ := json.Marshal(a.enf)
		al, _ := json.Marshal(a.al)
		var emb *bool
		if a.emb {
			emb = &a.emb
		}
		nul := func(s string) *string {
			if s == "" {
				return nil
			}
			return &s
		}
		if _, err := tx.Exec(ctx, `INSERT INTO anamnesis (paciente_id, version, enfermedades_sistemicas, alergias, antecedentes_quirurgicos, coagulopatia,
			anticoagulado, medicacion_actual, embarazo, registrado_por) VALUES ($1,1,$2,$3,$4,false,$5,$6,$7,$8)`,
			ids[a.doc], enf, al, nul(a.quir), a.antico, nul(a.med), emb, a.por); err != nil {
			return err
		}
	}

	// Citas relativas a "hoy" en la zona horaria de la clínica.
	ahora := time.Now()
	hoy := ahora.In(zona)
	en := func(dias, hora, min int) time.Time {
		return time.Date(hoy.Year(), hoy.Month(), hoy.Day()+dias, hora, min, 0, 0, zona)
	}
	citas := []struct {
		doc, odo, sillon, proc string
		inicio                 time.Time
		min                    int
		motivo                 string
	}{
		{"DEMO000101", valeria, s1a, "PRO-01", en(0, 8, 0), 45, "Limpieza semestral"},
		{"DEMO000102", jorge, s1b, "EXO-01", en(0, 9, 0), 45, "Valorar extracción 4.6 (paciente anticoagulado)"},
		{"DEMO000103", valeria, s1a, "SEL-01", en(0, 10, 30), 30, "Sellantes en molares permanentes"},
		{"DEMO000105", jorge, s2a, "VAL-01", en(0, 15, 0), 30, "Sensibilidad dental"},
		{"DEMO000106", valeria, s1a, "RES-01", en(0, 16, 0), 60, "Restauración 3.6"},
		{"DEMO000107", valeria, s1a, "VAL-01", en(1, 9, 0), 30, "Primera consulta"},
		{"DEMO000108", jorge, s2a, "PRO-01", en(1, 11, 0), 45, "Control"},
		{"DEMO000104", jorge, s1b, "END-01", en(3, 10, 0), 90, "Endodoncia 2.1"},
	}
	for _, c := range citas {
		fin := c.inicio.Add(time.Duration(c.min) * time.Minute)
		estado := domain.Programado
		if fin.Before(ahora) {
			estado = domain.Atendido
		} else if c.doc == "DEMO000105" {
			estado = domain.Confirmado
		}
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO citas (paciente_id, odontologo_id, sillon_id, procedimiento_id, inicio, fin, estado, motivo, creado_por)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, ids[c.doc], c.odo, c.sillon, procs[c.proc], c.inicio, fin, estado, c.motivo, recep).Scan(&id); err != nil {
			return err
		}
		_, _ = tx.Exec(ctx, `INSERT INTO citas_eventos (cita_id, estado_nuevo, usuario_id) VALUES ($1,'PROGRAMADO',$2)`, id, recep)
		if estado != domain.Programado {
			quien := recep
			if estado == domain.Atendido {
				quien = c.odo
			}
			_, _ = tx.Exec(ctx, `INSERT INTO citas_eventos (cita_id, estado_anterior, estado_nuevo, usuario_id) VALUES ($1,'PROGRAMADO',$2,$3)`, id, estado, quien)
		}
		if estado == domain.Atendido {
			continue
		}
		var canal string
		var tel, email *string
		var autoriza bool
		_ = tx.QueryRow(ctx, `SELECT canal_preferido, telefono, email, autoriza_mensajes FROM pacientes WHERE id=$1`, ids[c.doc]).Scan(&canal, &tel, &email, &autoriza)
		for _, m := range notificaciones.Planificar(c.inicio, notificaciones.Contacto{Canal: canal, Telefono: tel, Email: email, Autoriza: autoriza}, ahora) {
			if _, err := tx.Exec(ctx, `INSERT INTO notificaciones (cita_id, tipo, canal, destinatario, programada_para, estado, motivo) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				id, m.Tipo, m.Canal, m.Destinatario, m.ProgramadaPara, m.Estado, m.Motivo); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

// cargarExpediente (Etapa 3): datos clínicos de ejemplo para la paciente DEMO000101 (ficticia).
// No se crean notas firmadas ni procedimientos realizados: esos se firman con el código MFA real.
func cargarExpediente(ctx context.Context, pool *pgxpool.Pool) error {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM odontograma_eventos`).Scan(&n); err != nil || n > 0 {
		return err
	}
	var paciente string
	if err := pool.QueryRow(ctx, `SELECT id FROM pacientes WHERE numero_documento='DEMO000101'`).Scan(&paciente); err != nil {
		return nil // sin la clínica demo no hay expediente demo
	}
	valeria := idUsuario(ctx, pool, "odontologo@demo.sigo")
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	marcas := []struct {
		diente      int
		sup, hallaz string
	}{
		{16, "O", "CARIES"}, {16, "M", "CARIES"}, {26, "O", "RESTAURACION"}, {36, "O", "RESTAURACION"}, {36, "D", "RESTAURACION"},
		{46, "", "ENDODONCIA_REALIZADA"}, {46, "", "PROTESIS_FIJA"}, {18, "", "AUSENTE"}, {38, "", "EXODONCIA_INDICADA"},
		{11, "O", "CARIES"}, {47, "O", "SELLANTE"}, {25, "", "ENDODONCIA_REQUERIDA"},
	}
	for _, m := range marcas {
		var sup *string
		if m.sup != "" {
			sup = &m.sup
		}
		if _, err := tx.Exec(ctx, `INSERT INTO odontograma_eventos (paciente_id, diente, superficie, hallazgo, accion, registrado_por, registrado_en)
			VALUES ($1,$2,$3,$4,'AGREGAR',$5, now() - interval '30 days')`, paciente, m.diente, sup, m.hallaz, valeria); err != nil {
			return err
		}
	}
	// Periodontograma de ejemplo (29 dientes presentes); valores ficticios.
	var perio string
	if err := tx.QueryRow(ctx, `INSERT INTO periodontogramas (paciente_id, observaciones, registrado_por, registrado_en)
		VALUES ($1, 'Control periodontal de ejemplo (datos ficticios).', $2, now() - interval '30 days') RETURNING id`, paciente, valeria).Scan(&perio); err != nil {
		return err
	}
	for i, d := range []int{17, 16, 15, 14, 13, 12, 11, 21, 22, 23, 24, 25, 26, 27, 28, 47, 46, 45, 44, 43, 42, 41, 31, 32, 33, 34, 35, 36, 37} {
		mov := 0
		if d == 46 {
			mov = 1
		}
		if _, err := tx.Exec(ctx, `INSERT INTO perio_dientes (periodontograma_id, diente, ausente, movilidad, furca) VALUES ($1,$2,false,$3,0)`, perio, d, mov); err != nil {
			return err
		}
		for j, sitio := range []string{"MV", "V", "DV", "ML", "L", "DL"} {
			ps := 2 + (i+j)%3 // 2–4 mm
			mg := 0
			if d == 46 && (sitio == "MV" || sitio == "V") {
				ps, mg = 5, 1
			}
			if _, err := tx.Exec(ctx, `INSERT INTO perio_sitios (periodontograma_id, diente, sitio, profundidad, margen, sangrado, supuracion)
				VALUES ($1,$2,$3,$4,$5,$6,false)`, perio, d, sitio, ps, mg, ps >= 4); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO notas_soep (paciente_id, autor_id, subjetivo, objetivo, evaluacion, plan, bloqueo_en)
		VALUES ($1,$2,'Refiere sensibilidad al frío en el lado superior derecho.','Caries oclusal y mesial en 16; resto sin cambios.',
		'Caries dentinaria en 16.','Restauración de 16 en la próxima cita. Recordar alergia a penicilina.', now() + interval '24 hours')`, paciente, valeria); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const pieConsentimiento = `

Declaro que he leído (o me han leído) este documento, que pude hacer preguntas y que me fueron respondidas. Sé que puedo retirar este consentimiento antes del procedimiento.

Firmante: {{firmante}} ({{relacion}}) — {{fecha}}.`

var Plantillas = []struct{ Codigo, Titulo, Proc, Texto string }{
	{"GEN-01", "Consentimiento general de atención odontológica", "",
		`Yo, {{firmante}} ({{relacion}}), respecto de la atención de {{paciente}} ({{documento}}), autorizo a {{odontologo}} (registro {{registro}}) y al equipo de {{clinica}} a realizar la valoración, los exámenes y los tratamientos odontológicos de rutina que se me expliquen, incluido {{procedimiento}}.

Me informaron que todo tratamiento puede tener molestias pasajeras (sensibilidad, inflamación, sangrado leve) y que es importante informar mis enfermedades, alergias y medicamentos. Me comprometo a seguir las indicaciones y a asistir a los controles.` + pieConsentimiento},
	{"EXO-01", "Consentimiento para exodoncia (extracción dental)", "EXO-01",
		`Yo, {{firmante}} ({{relacion}}), respecto de la atención de {{paciente}} ({{documento}}), autorizo a {{odontologo}} (registro {{registro}}) a realizar {{procedimiento}}.

Me explicaron el motivo de la extracción, las alternativas y los riesgos: dolor e inflamación, sangrado, infección, alveolitis, lesión de dientes vecinos, comunicación con el seno maxilar (dientes superiores) y alteración temporal o permanente de la sensibilidad del labio o la lengua (dientes inferiores). Recibiré indicaciones escritas para después del procedimiento.` + pieConsentimiento},
	{"END-01", "Consentimiento para endodoncia (tratamiento de conducto)", "END-01",
		`Yo, {{firmante}} ({{relacion}}), respecto de la atención de {{paciente}} ({{documento}}), autorizo a {{odontologo}} (registro {{registro}}) a realizar {{procedimiento}}.

Me explicaron que el tratamiento busca conservar el diente, que puede requerir varias sesiones y que el pronóstico no es seguro en todos los casos. Riesgos: dolor posoperatorio, fractura del instrumento o del diente, perforación, necesidad de retratamiento o de extracción. El diente tratado puede requerir una restauración o corona posterior.` + pieConsentimiento},
}

// cargarPlanes (Etapa 4): plantillas de consentimiento y un presupuesto demo en BORRADOR.
func cargarPlanes(ctx context.Context, pool *pgxpool.Pool) error {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM consentimiento_plantillas`).Scan(&n); err != nil {
		return err
	}
	admin := idUsuario(ctx, pool, "admin@demo.sigo")
	if n == 0 {
		for _, p := range Plantillas {
			var proc *string
			if p.Proc != "" {
				var id string
				if err := pool.QueryRow(ctx, `SELECT id FROM procedimientos WHERE codigo=$1`, p.Proc).Scan(&id); err == nil {
					proc = &id
				}
			}
			if _, err := pool.Exec(ctx, `INSERT INTO consentimiento_plantillas (codigo, version, titulo, texto, procedimiento_id, creado_por)
				VALUES ($1, 1, $2, $3, $4, NULLIF($5,'')::uuid)`, p.Codigo, p.Titulo, p.Texto, proc, admin); err != nil {
				return err
			}
		}
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM presupuestos`).Scan(&n); err != nil || n > 0 {
		return err
	}
	var paciente string
	if err := pool.QueryRow(ctx, `SELECT id FROM pacientes WHERE numero_documento='DEMO000101'`).Scan(&paciente); err != nil {
		return nil
	}
	valeria := idUsuario(ctx, pool, "odontologo@demo.sigo")
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var pres string
	if err := tx.QueryRow(ctx, `INSERT INTO presupuestos (paciente_id, odontologo_id, titulo, observaciones)
		VALUES ($1,$2,'Plan de tratamiento integral (ejemplo)','Datos ficticios para la demostración.') RETURNING id`, paciente, valeria).Scan(&pres); err != nil {
		return err
	}
	items := []struct {
		fase      int
		faseNom   string
		sesion    int
		proc      string
		diente    *int
		sups      []string
		hon, desc string
	}{
		{1, "Higiene", 1, "PRO-01", nil, []string{}, "35.00", "0"},
		{2, "Operatoria", 2, "RES-01", ptr(16), []string{"O", "M"}, "45.00", "0"},
		{2, "Operatoria", 2, "RES-01", ptr(11), []string{"O"}, "45.00", "5.00"},
		{3, "Endodoncia", 3, "END-01", ptr(25), []string{}, "180.00", "0"},
		{4, "Cirugía", 4, "EXO-01", ptr(38), []string{}, "50.00", "0"},
	}
	for i, it := range items {
		if _, err := tx.Exec(ctx, `INSERT INTO presupuesto_items (presupuesto_id, orden, fase, fase_nombre, sesion, procedimiento_id, diente, superficies, honorario, descuento)
			VALUES ($1,$2,$3,$4,$5,(SELECT id FROM procedimientos WHERE codigo=$6),$7,$8,$9,$10)`,
			pres, i+1, it.fase, it.faseNom, it.sesion, it.proc, it.diente, it.sups, it.hon, it.desc); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func ptr[T any](v T) *T { return &v }

// cargarCaja (Etapa 5): caja de ayer de recepción, cerrada a ciegas con un faltante de 1.00.
func cargarCaja(ctx context.Context, pool *pgxpool.Pool) error {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM caja_sesiones`).Scan(&n); err != nil || n > 0 {
		return err
	}
	recep := idUsuario(ctx, pool, "recepcion@demo.sigo")
	if recep == "" {
		return nil
	}
	var daniela, carlos string
	if pool.QueryRow(ctx, `SELECT id FROM pacientes WHERE numero_documento='DEMO000107'`).Scan(&daniela) != nil ||
		pool.QueryRow(ctx, `SELECT id FROM pacientes WHERE numero_documento='DEMO000102'`).Scan(&carlos) != nil {
		return nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var sesion string
	if err := tx.QueryRow(ctx, `INSERT INTO caja_sesiones (cajero_id, fondo_inicial, abierta_en) VALUES ($1, 50.00, now() - interval '1 day 2 hours') RETURNING id`, recep).Scan(&sesion); err != nil {
		return err
	}
	type pago struct{ medio, monto, ref string }
	cobros := []struct {
		paciente, codigo string
		pagos            []pago
	}{
		{daniela, "VAL-01", []pago{{"EFECTIVO", "25.00", ""}}},
		{carlos, "PRO-01", []pago{{"EFECTIVO", "20.00", ""}, {"TARJETA", "15.00", "VOUCHER-DEMO-0001"}}},
	}
	prov := &facturacion.Simulado{}
	for i, cb := range cobros {
		var procID, nombre, arancel string
		if err := tx.QueryRow(ctx, `SELECT id, nombre, arancel::text FROM procedimientos WHERE codigo=$1`, cb.codigo).Scan(&procID, &nombre, &arancel); err != nil {
			return err
		}
		var id string
		var numero int64
		if err := tx.QueryRow(ctx, `INSERT INTO cobros (sesion_id, paciente_id, total, registrado_por, registrado_en) VALUES ($1,$2,$3,$4, now() - interval '1 day' + $5 * interval '30 minutes') RETURNING id, numero`,
			sesion, cb.paciente, arancel, recep, i).Scan(&id, &numero); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO cobro_conceptos (cobro_id, procedimiento_id, descripcion, monto) VALUES ($1,$2,$3,$4)`, id, procID, cb.codigo+" · "+nombre, arancel); err != nil {
			return err
		}
		for _, p := range cb.pagos {
			var ref *string
			if p.ref != "" {
				ref = &p.ref
			}
			if _, err := tx.Exec(ctx, `INSERT INTO cobro_pagos (cobro_id, medio, monto, referencia) VALUES ($1,$2,$3,$4)`, id, p.medio, p.monto, ref); err != nil {
				return err
			}
		}
		var sec int64
		if err := tx.QueryRow(ctx, `SELECT nextval('comprobantes_factura')`).Scan(&sec); err != nil {
			return err
		}
		doc := facturacion.Documento{Tipo: "FACTURA", Secuencial: sec, Emisor: "Clínica Dental Demo", Receptor: facturacion.Receptor{Documento: "Datos ficticios", Nombre: "Paciente de demostración"},
			Lineas: []facturacion.Linea{{Descripcion: cb.codigo + " · " + nombre, Monto: arancel}}, Total: arancel, Fecha: time.Now().UTC().Format(time.RFC3339), ReciboInterno: numero}
		res, _ := prov.Emitir(ctx, doc)
		raw, _ := json.Marshal(doc)
		if _, err := tx.Exec(ctx, `INSERT INTO comprobantes (cobro_id, tipo, estado, secuencial, numero, autorizacion, proveedor, simulado, mensaje, documento, hash_documento, emitido_por)
			VALUES ($1,'FACTURA',$2,$3,$4,$5,'simulado',true,$6,$7,$8,$9)`, id, res.Estado, sec, res.Numero, res.Autorizacion, res.Mensaje, raw, seguridad.SHA256Hex(string(raw)), recep); err != nil {
			return err
		}
	}
	// Cierre ciego: declara 94.00 en efectivo (el teórico es 50 + 25 + 20 = 95.00).
	if _, err := tx.Exec(ctx, `UPDATE caja_sesiones SET estado='CERRADA', cerrada_en=now() - interval '20 hours',
		declarado_efectivo=94.00, declarado_tarjeta=15.00, declarado_transferencia=0, teorico_efectivo=95.00, teorico_tarjeta=15.00, teorico_transferencia=0
		WHERE id=$1`, sesion); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// cargarInventario (Etapa 6): ubicaciones, insumos, stock inicial, recetas de consumo y reglas de honorarios.
func cargarInventario(ctx context.Context, pool *pgxpool.Pool) error {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM inventario_ubicaciones`).Scan(&n); err != nil || n > 0 {
		return err
	}
	asis, aud := idUsuario(ctx, pool, "asistente@demo.sigo"), idUsuario(ctx, pool, "auditor@demo.sigo")
	if asis == "" || aud == "" {
		return nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	ubic := map[string]string{}
	var bodega string
	if err := tx.QueryRow(ctx, `INSERT INTO inventario_ubicaciones (nombre, por_defecto) VALUES ('Bodega central', true) RETURNING id`).Scan(&bodega); err != nil {
		return err
	}
	ubic["BOD"] = bodega
	for _, c := range []string{"Consultorio 1", "Consultorio 2"} {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO inventario_ubicaciones (nombre, consultorio_id) SELECT $1::text, id FROM consultorios WHERE nombre = $1::text RETURNING id`, c).Scan(&id); err != nil {
			return err
		}
		ubic[c] = id
	}
	insumos := []struct {
		cod, nom, uni, costo, min string
		bodega, c1, c2            string
	}{
		{"GUA-01", "Guantes de nitrilo (par)", "par", "0.18", "50", "300", "40", "40"},
		{"ANE-01", "Anestesia lidocaína 2 % (carpule)", "carpule", "0.85", "20", "100", "15", "15"},
		{"AGU-01", "Aguja dental corta", "unidad", "0.12", "30", "150", "20", "20"},
		{"RES-A2", "Resina compuesta A2", "g", "1.60", "10", "40", "6", "4"},
		{"ADH-01", "Adhesivo universal", "ml", "4.20", "5", "20", "3", "3"},
		{"GRA-01", "Ácido grabador 37 %", "ml", "0.90", "5", "30", "5", "5"},
		{"PAS-01", "Pasta profiláctica", "g", "0.08", "50", "400", "60", "60"},
		{"SEL-01", "Sellante de fosas y fisuras", "ml", "3.50", "3", "8", "2", "2"},
		{"LIM-01", "Lima endodóntica tipo K", "unidad", "1.10", "10", "30", "6", "6"},
		{"GAS-01", "Gasa estéril", "unidad", "0.05", "100", "500", "60", "60"},
		{"SUT-01", "Sutura 3-0", "unidad", "2.40", "5", "2", "1", "1"}, // bajo el mínimo: aparece la alerta
		{"HIP-01", "Hipoclorito de sodio 2,5 %", "ml", "0.01", "500", "2000", "300", "300"},
	}
	ins := map[string]string{}
	for _, x := range insumos {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO insumos (codigo, nombre, unidad, costo_unitario, stock_minimo) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			x.cod, x.nom, x.uni, x.costo, x.min).Scan(&id); err != nil {
			return err
		}
		ins[x.cod] = id
		for u, q := range map[string]string{"BOD": x.bodega, "Consultorio 1": x.c1, "Consultorio 2": x.c2} {
			if _, err := tx.Exec(ctx, `INSERT INTO movimientos_stock (insumo_id, ubicacion_id, tipo, cantidad, costo_unitario, motivo, registrado_por, registrado_en)
				VALUES ($1,$2,'ENTRADA',$3,$4,'Stock inicial (datos ficticios)',$5, now() - interval '7 days')`, id, ubic[u], q, x.costo, asis); err != nil {
				return err
			}
		}
	}
	recetas := map[string][][2]string{
		"VAL-01": {{"GUA-01", "1"}},
		"PRO-01": {{"GUA-01", "1"}, {"PAS-01", "5"}},
		"RES-01": {{"GUA-01", "1"}, {"ANE-01", "1"}, {"AGU-01", "1"}, {"RES-A2", "0.3"}, {"ADH-01", "0.1"}, {"GRA-01", "0.2"}},
		"END-01": {{"GUA-01", "2"}, {"ANE-01", "2"}, {"AGU-01", "2"}, {"LIM-01", "3"}, {"HIP-01", "20"}, {"GAS-01", "4"}},
		"EXO-01": {{"GUA-01", "1"}, {"ANE-01", "2"}, {"AGU-01", "1"}, {"GAS-01", "6"}, {"SUT-01", "1"}},
		"SEL-01": {{"GUA-01", "1"}, {"GRA-01", "0.2"}, {"SEL-01", "0.2"}},
	}
	for proc, lineas := range recetas {
		for _, l := range lineas {
			if _, err := tx.Exec(ctx, `INSERT INTO recetas_consumo (procedimiento_id, insumo_id, cantidad) SELECT id, $2, $3 FROM procedimientos WHERE codigo=$1`,
				proc, ins[l[0]], l[1]); err != nil {
				return err
			}
		}
	}
	valeria, jorge := idUsuario(ctx, pool, "odontologo@demo.sigo"), idUsuario(ctx, pool, "odontologo2@demo.sigo")
	reglas := []struct {
		odo, proc, tipo, valor string
	}{
		{"", "", "PORCENTAJE", "40"},                 // general: 40 % de lo cobrado
		{valeria, "END-01", "PORCENTAJE_NETO", "50"}, // Dra. Ríos en endodoncia: 50 % del neto
		{jorge, "PRO-01", "MONTO_FIJO", "12.00"},     // Dr. Andrade en profilaxis: 12.00 fijo
	}
	for _, r := range reglas {
		var odo, proc *string
		if r.odo != "" {
			odo = &r.odo
		}
		if r.proc != "" {
			var id string
			if err := tx.QueryRow(ctx, `SELECT id FROM procedimientos WHERE codigo=$1`, r.proc).Scan(&id); err != nil {
				return err
			}
			proc = &id
		}
		if _, err := tx.Exec(ctx, `INSERT INTO reglas_honorario (odontologo_id, procedimiento_id, tipo, valor, vigente_desde, creado_por) VALUES ($1,$2,$3,$4,'2026-01-01',$5)`,
			odo, proc, r.tipo, r.valor, aud); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

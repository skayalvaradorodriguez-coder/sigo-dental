// Package semilla carga datos de demostración — TODOS FICTICIOS.
//   - Usuarios: crea los que falten (uno por rol) con la contraseña demo y MFA activo
//     (secreto aleatorio; código vigente con `go run . totp <correo>`). Los existentes NO se
//     modifican, salvo con `go run . semilla --reset-usuarios`.
//   - Clínica (Etapa 2): sólo si aún no hay sedes.
package semilla

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sigo-backend/domain"
	"sigo-backend/notificaciones"
	"sigo-backend/seguridad"
)

const PasswordDemo = "Demo-Sigo-2026"

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
	return cargarClinica(ctx, pool, zona)
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

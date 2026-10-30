// Package semilla carga datos de demostración — TODOS FICTICIOS.
//   - Usuarios: crea los que falten (uno por rol) con la contraseña demo y MFA activo
//     (secreto aleatorio; código vigente con `go run . totp <correo>`). Los existentes NO se
//     modifican, salvo con `go run . semilla --reset-usuarios`.
package semilla

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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
	return nil
}

package models

import "time"

type Usuario struct {
	ID                  string     `json:"id"`
	Email               string     `json:"email"`
	Nombre              string     `json:"nombre"`
	Rol                 string     `json:"rol"`
	RegistroProfesional *string    `json:"registroProfesional"`
	Activo              bool       `json:"activo"`
	MFAActivo           bool       `json:"mfaActivo"`
	DebeCambiarPassword bool       `json:"debeCambiarPassword"`
	BloqueadoHasta      *time.Time `json:"bloqueadoHasta"`
	UltimoAcceso        *time.Time `json:"ultimoAcceso"`
	CreadoEn            time.Time  `json:"creadoEn"`
	// Campos internos (nunca se serializan).
	PasswordHash     string  `json:"-"`
	MFASecreto       *string `json:"-"`
	MFAUltimoPaso    *int64  `json:"-"`
	IntentosFallidos int     `json:"-"`
}

type Sesion struct {
	JTI             string
	UsuarioID       string
	MFAVerificado   bool
	CreadaEn        time.Time
	UltimaActividad time.Time
	ExpiraEn        time.Time
	RevocadaEn      *time.Time
}

type EventoAuditoria struct {
	ID         int64          `json:"id"`
	OcurridoEn time.Time      `json:"ocurridoEn"`
	UsuarioID  *string        `json:"usuarioId"`
	Usuario    *string        `json:"usuario"`
	Email      *string        `json:"email"`
	Accion     string         `json:"accion"`
	Entidad    string         `json:"entidad"`
	EntidadID  *string        `json:"entidadId"`
	IP         *string        `json:"ip"`
	UserAgent  *string        `json:"userAgent"`
	Detalle    map[string]any `json:"detalle"`
}

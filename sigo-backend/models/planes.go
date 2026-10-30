package models

import (
	"time"

	"sigo-backend/domain"
)

// ---------------------------------------------------------------- Etapa 4

type PresupuestoItem struct {
	ID                     string     `json:"id"`
	Orden                  int        `json:"orden"`
	Fase                   int        `json:"fase"`
	FaseNombre             string     `json:"faseNombre"`
	Sesion                 int        `json:"sesion"`
	ProcedimientoID        string     `json:"procedimientoId"`
	Codigo                 string     `json:"codigo"`
	Procedimiento          string     `json:"procedimiento"`
	Diente                 *int       `json:"diente"`
	Superficies            []string   `json:"superficies"`
	Honorario              string     `json:"honorario"`
	CostoLaboratorio       string     `json:"costoLaboratorio"`
	Descuento              string     `json:"descuento"`
	Subtotal               string     `json:"subtotal"`
	RequiereAutorizacion   bool       `json:"requiereAutorizacion"`
	DescuentoAutorizadoPor *string    `json:"descuentoAutorizadoPor"`
	DescuentoAutorizadoEn  *time.Time `json:"descuentoAutorizadoEn"`
	Decision               string     `json:"decision"`
}

type Presupuesto struct {
	ID                    string            `json:"id"`
	PacienteID            string            `json:"pacienteId"`
	Paciente              string            `json:"paciente"`
	OdontologoID          string            `json:"odontologoId"`
	Odontologo            string            `json:"odontologo"`
	Titulo                string            `json:"titulo"`
	Observaciones         *string           `json:"observaciones"`
	Estado                string            `json:"estado"`
	CreadoEn              time.Time         `json:"creadoEn"`
	ActualizadoEn         time.Time         `json:"actualizadoEn"`
	PresentadoEn          *time.Time        `json:"presentadoEn"`
	HashPresentado        *string           `json:"hashPresentado"`
	DecididoEn            *time.Time        `json:"decididoEn"`
	DecisionRegistradaPor *string           `json:"decisionRegistradaPor"`
	DecisionNota          *string           `json:"decisionNota"`
	Items                 []PresupuestoItem `json:"items"`
	Totales               domain.Totales    `json:"totales"`
	DescuentosPendientes  int               `json:"descuentosPendientes"`
	Integro               *bool             `json:"integro,omitempty"` // hash de lo presentado vs. lo guardado
}

type Tarea struct {
	ID              string     `json:"id"`
	PresupuestoID   string     `json:"presupuestoId"`
	Presupuesto     string     `json:"presupuesto"`
	ProcedimientoID string     `json:"procedimientoId"`
	Codigo          string     `json:"codigo"`
	Procedimiento   string     `json:"procedimiento"`
	DuracionMin     int        `json:"duracionMin"`
	Diente          *int       `json:"diente"`
	Superficies     []string   `json:"superficies"`
	Fase            int        `json:"fase"`
	Sesion          int        `json:"sesion"`
	Estado          string     `json:"estado"` // PENDIENTE | AGENDADA | REALIZADA
	CitaID          *string    `json:"citaId"`
	CitaInicio      *time.Time `json:"citaInicio"`
	RealizadoEn     *time.Time `json:"realizadoEn"`
}

type PlantillaConsentimiento struct {
	ID              string    `json:"id"`
	Codigo          string    `json:"codigo"`
	Version         int       `json:"version"`
	Titulo          string    `json:"titulo"`
	Texto           string    `json:"texto"`
	ProcedimientoID *string   `json:"procedimientoId"`
	Procedimiento   *string   `json:"procedimiento"`
	CreadoEn        time.Time `json:"creadoEn"`
}

type Consentimiento struct {
	ID                string     `json:"id"`
	PacienteID        string     `json:"pacienteId"`
	Paciente          string     `json:"paciente"`
	PlantillaID       string     `json:"plantillaId"`
	Titulo            string     `json:"titulo"`
	Version           int        `json:"version"`
	ProcedimientoID   *string    `json:"procedimientoId"`
	Procedimiento     *string    `json:"procedimiento"`
	OdontologoID      string     `json:"odontologoId"`
	Odontologo        string     `json:"odontologo"`
	Texto             string     `json:"texto"`
	HashDocumento     string     `json:"hashDocumento"`
	FirmanteRelacion  string     `json:"firmanteRelacion"`
	FirmanteNombre    string     `json:"firmanteNombre"`
	FirmanteDocumento string     `json:"firmanteDocumento"`
	Metodo            string     `json:"metodo"`
	Estado            string     `json:"estado"`
	CreadoEn          time.Time  `json:"creadoEn"`
	FirmaImagen       string     `json:"firmaImagen,omitempty"` // data URL (sólo en el detalle)
	FirmaImagenSHA256 *string    `json:"firmaImagenSha256"`
	OTPCanal          *string    `json:"otpCanal"`
	OTPDestinatario   *string    `json:"otpDestinatario"`
	OTPProveedor      *string    `json:"otpProveedor"`
	OTPSimulado       *bool      `json:"otpSimulado"`
	OTPExpira         *time.Time `json:"otpExpira"`
	OTPIntentos       int        `json:"otpIntentos"`
	FirmadoEn         *time.Time `json:"firmadoEn"`
	HashFirma         *string    `json:"hashFirma"`
	Integro           *bool      `json:"integro"`
	// OTPSimuladoCodigo: SÓLO con el proveedor simulado y sólo en la respuesta de creación, para
	// poder probar el flujo sin un SMS real. Con un proveedor real nunca se devuelve.
	OTPSimuladoCodigo string `json:"otpSimuladoCodigo,omitempty"`
	// Datos internos (no se envían).
	OTPHash        *string `json:"-"`
	FirmaImagenRaw []byte  `json:"-"`
}

type RecetaItem struct {
	Medicamento  string `json:"medicamento"`
	Presentacion string `json:"presentacion"`
	Dosis        string `json:"dosis"`
	Via          string `json:"via"`
	Frecuencia   string `json:"frecuencia"`
	Duracion     string `json:"duracion"`
	Indicaciones string `json:"indicaciones"`
}

type Receta struct {
	ID                  string                `json:"id"`
	PacienteID          string                `json:"pacienteId"`
	OdontologoID        string                `json:"odontologoId"`
	Odontologo          string                `json:"odontologo"`
	RegistroProfesional *string               `json:"registroProfesional"`
	Indicaciones        *string               `json:"indicaciones"`
	AlertasConfirmadas  []domain.AlertaReceta `json:"alertasConfirmadas"`
	EmitidaEn           time.Time             `json:"emitidaEn"`
	HashFirma           string                `json:"hashFirma"`
	Items               []RecetaItem          `json:"items"`
	Integra             *bool                 `json:"integra"`
}

type Archivo struct {
	ID          string    `json:"id"`
	PacienteID  string    `json:"pacienteId"`
	Categoria   string    `json:"categoria"`
	Nombre      string    `json:"nombre"`
	TipoMime    string    `json:"tipoMime"`
	Tamano      int       `json:"tamano"`
	SHA256      string    `json:"sha256"`
	Clave       string    `json:"-"`
	Descripcion *string   `json:"descripcion"`
	SubidoPor   string    `json:"subidoPor"`
	SubidoEn    time.Time `json:"subidoEn"`
}

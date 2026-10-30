package models

import (
	"time"

	"sigo-backend/domain"
)

type Sede struct {
	ID           string        `json:"id"`
	Nombre       string        `json:"nombre"`
	Direccion    *string       `json:"direccion"`
	Activo       bool          `json:"activo"`
	Consultorios []Consultorio `json:"consultorios,omitempty"`
}

type Consultorio struct {
	ID       string   `json:"id"`
	SedeID   string   `json:"sedeId"`
	Nombre   string   `json:"nombre"`
	Activo   bool     `json:"activo"`
	Sillones []Sillon `json:"sillones,omitempty"`
}

type Sillon struct {
	ID            string `json:"id"`
	ConsultorioID string `json:"consultorioId"`
	Nombre        string `json:"nombre"`
	Activo        bool   `json:"activo"`
	Consultorio   string `json:"consultorio,omitempty"`
	SedeID        string `json:"sedeId,omitempty"`
	Sede          string `json:"sede,omitempty"`
}

type Procedimiento struct {
	ID          string `json:"id"`
	Codigo      string `json:"codigo"`
	Nombre      string `json:"nombre"`
	DuracionMin int    `json:"duracionMin"`
	Arancel     string `json:"arancel"` // decimal como texto: sin pérdida de precisión
	Activo      bool   `json:"activo"`
}

type Convenio struct {
	ID          string  `json:"id"`
	Nombre      string  `json:"nombre"`
	Aseguradora *string `json:"aseguradora"`
	Activo      bool    `json:"activo"`
}

type Odontologo struct {
	ID                  string  `json:"id"`
	Nombre              string  `json:"nombre"`
	RegistroProfesional *string `json:"registroProfesional"`
}

type Tutor struct {
	Nombre          string  `json:"nombre"`
	TipoDocumento   string  `json:"tipoDocumento"`
	NumeroDocumento string  `json:"numeroDocumento"`
	Parentesco      string  `json:"parentesco"`
	Telefono        string  `json:"telefono"`
	Email           *string `json:"email"`
}

type Paciente struct {
	ID                   string          `json:"id"`
	TipoDocumento        string          `json:"tipoDocumento"`
	NumeroDocumento      string          `json:"numeroDocumento"`
	Nombres              string          `json:"nombres"`
	Apellidos            string          `json:"apellidos"`
	FechaNacimiento      string          `json:"fechaNacimiento"`
	Sexo                 string          `json:"sexo"`
	Telefono             *string         `json:"telefono"`
	Email                *string         `json:"email"`
	Direccion            *string         `json:"direccion"`
	EmergenciaNombre     string          `json:"emergenciaNombre"`
	EmergenciaTelefono   string          `json:"emergenciaTelefono"`
	EmergenciaParentesco *string         `json:"emergenciaParentesco"`
	CanalPreferido       string          `json:"canalPreferido"`
	AutorizaMensajes     bool            `json:"autorizaMensajes"`
	ConvenioID           *string         `json:"convenioId"`
	Convenio             *string         `json:"convenio"`
	NumeroAfiliado       *string         `json:"numeroAfiliado"`
	CreadoEn             time.Time       `json:"creadoEn"`
	Edad                 int             `json:"edad"`
	EsMenor              bool            `json:"esMenor"`
	Tutor                *Tutor          `json:"tutor"`
	AnamnesisPendiente   bool            `json:"anamnesisPendiente"`
	AnamnesisActualizada *time.Time      `json:"anamnesisActualizada"`
	Alertas              []domain.Alerta `json:"alertas"`
}

type Anamnesis struct {
	ID                      string           `json:"id"`
	Version                 int              `json:"version"`
	EnfermedadesSistemicas  []string         `json:"enfermedadesSistemicas"`
	Alergias                []domain.Alergia `json:"alergias"`
	AntecedentesQuirurgicos *string          `json:"antecedentesQuirurgicos"`
	Coagulopatia            bool             `json:"coagulopatia"`
	Anticoagulado           bool             `json:"anticoagulado"`
	MedicacionActual        *string          `json:"medicacionActual"`
	Embarazo                *bool            `json:"embarazo"`
	OtraCondicionCritica    *string          `json:"otraCondicionCritica"`
	Observaciones           *string          `json:"observaciones"`
	RegistradoPor           string           `json:"registradoPor"`
	RegistradoEn            time.Time        `json:"registradoEn"`
	Alertas                 []domain.Alerta  `json:"alertas,omitempty"`
}

func (a *Anamnesis) DatosAlerta() *domain.DatosAlerta {
	if a == nil {
		return nil
	}
	d := &domain.DatosAlerta{Alergias: a.Alergias, Anticoagulado: a.Anticoagulado, Coagulopatia: a.Coagulopatia,
		Embarazo: a.Embarazo, EnfermedadesSistemicas: a.EnfermedadesSistemicas}
	if a.OtraCondicionCritica != nil {
		d.OtraCondicionCritica = *a.OtraCondicionCritica
	}
	return d
}

type Cita struct {
	ID                 string         `json:"id"`
	Inicio             time.Time      `json:"inicio"`
	Fin                time.Time      `json:"fin"`
	Estado             string         `json:"estado"`
	Motivo             *string        `json:"motivo"`
	MotivoCancelacion  *string        `json:"motivoCancelacion"`
	PacienteID         string         `json:"pacienteId"`
	Paciente           string         `json:"paciente"`
	PacienteDocumento  string         `json:"pacienteDocumento"`
	PacienteTelefono   *string        `json:"pacienteTelefono"`
	OdontologoID       string         `json:"odontologoId"`
	Odontologo         string         `json:"odontologo"`
	SillonID           string         `json:"sillonId"`
	Sillon             string         `json:"sillon"`
	Consultorio        string         `json:"consultorio"`
	Sede               string         `json:"sede"`
	ProcedimientoID    *string        `json:"procedimientoId"`
	Procedimiento      *string        `json:"procedimiento"`
	CreadoEn           time.Time      `json:"creadoEn"`
	CreadoPor          string         `json:"creadoPor"`
	AlertasCriticas    []string       `json:"alertasCriticas"`
	AnamnesisPendiente bool           `json:"anamnesisPendiente"`
	Eventos            []EventoCita   `json:"eventos,omitempty"`
	Notificaciones     []Notificacion `json:"notificaciones,omitempty"`
}

type EventoCita struct {
	ID             int64     `json:"id"`
	EstadoAnterior *string   `json:"estadoAnterior"`
	EstadoNuevo    string    `json:"estadoNuevo"`
	Nota           *string   `json:"nota"`
	Usuario        string    `json:"usuario"`
	OcurridoEn     time.Time `json:"ocurridoEn"`
}

type Notificacion struct {
	ID             string     `json:"id"`
	CitaID         string     `json:"citaId"`
	Tipo           string     `json:"tipo"`
	Canal          string     `json:"canal"`
	Destinatario   *string    `json:"destinatario"`
	ProgramadaPara time.Time  `json:"programadaPara"`
	Estado         string     `json:"estado"`
	Motivo         *string    `json:"motivo"`
	Proveedor      *string    `json:"proveedor"`
	Simulado       *bool      `json:"simulado"`
	Mensaje        *string    `json:"mensaje"`
	IDProveedor    *string    `json:"idProveedor"`
	EnviadaEn      *time.Time `json:"enviadaEn"`
	Paciente       string     `json:"paciente,omitempty"`
	InicioCita     *time.Time `json:"inicioCita,omitempty"`
}

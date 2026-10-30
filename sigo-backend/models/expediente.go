package models

import (
	"time"

	"sigo-backend/domain"
)

// ---------------------------------------------------------------- Expediente clínico (Etapa 3)

type EventoOdontograma struct {
	ID            int64     `json:"id"`
	Diente        int       `json:"diente"`
	Superficie    *string   `json:"superficie"`
	Hallazgo      string    `json:"hallazgo"`
	Accion        string    `json:"accion"`
	Nota          *string   `json:"nota"`
	RegistradoPor string    `json:"registradoPor"`
	RegistradoEn  time.Time `json:"registradoEn"`
}

type MarcaOdontograma struct {
	Diente     int       `json:"diente"`
	Superficie *string   `json:"superficie"`
	Hallazgo   string    `json:"hallazgo"`
	Desde      time.Time `json:"desde"`
}

type Odontograma struct {
	Catalogo  []domain.Hallazgo   `json:"catalogo"`
	Estado    []MarcaOdontograma  `json:"estado"`
	Historial []EventoOdontograma `json:"historial"`
}

type PerioSitio struct {
	Sitio       string `json:"sitio"`
	Profundidad int    `json:"profundidad"`
	Margen      int    `json:"margen"`
	NIC         int    `json:"nic"`
	Sangrado    bool   `json:"sangrado"`
	Supuracion  bool   `json:"supuracion"`
}

type PerioDiente struct {
	Diente    int          `json:"diente"`
	Ausente   bool         `json:"ausente"`
	Movilidad *int         `json:"movilidad"`
	Furca     *int         `json:"furca"`
	Sitios    []PerioSitio `json:"sitios"`
}

type Periodontograma struct {
	ID            string              `json:"id"`
	Observaciones *string             `json:"observaciones"`
	RegistradoPor string              `json:"registradoPor"`
	RegistradoEn  time.Time           `json:"registradoEn"`
	Dientes       []PerioDiente       `json:"dientes"`
	Resumen       domain.ResumenPerio `json:"resumen"`
}

type Adenda struct {
	ID        string    `json:"id"`
	Autor     string    `json:"autor"`
	Texto     string    `json:"texto"`
	CreadoEn  time.Time `json:"creadoEn"`
	HashFirma string    `json:"hashFirma"`
}

type NotaSOEP struct {
	ID                  string     `json:"id"`
	PacienteID          string     `json:"pacienteId"`
	CitaID              *string    `json:"citaId"`
	AutorID             string     `json:"autorId"`
	Autor               string     `json:"autor"`
	Subjetivo           string     `json:"subjetivo"`
	Objetivo            string     `json:"objetivo"`
	Evaluacion          string     `json:"evaluacion"`
	Plan                string     `json:"plan"`
	CreadoEn            time.Time  `json:"creadoEn"`
	ActualizadoEn       time.Time  `json:"actualizadoEn"`
	BloqueoEn           time.Time  `json:"bloqueoEn"`
	FirmadaEn           *time.Time `json:"firmadaEn"`
	FirmadaPorID        *string    `json:"firmadaPorId"`
	FirmadaPor          *string    `json:"firmadaPor"`
	RegistroProfesional *string    `json:"registroProfesional"`
	HashFirma           *string    `json:"hashFirma"`
	Adendas             []Adenda   `json:"adendas"`
	// Calculados al responder:
	Estado   string `json:"estado"`   // BORRADOR | FIRMADA | BLOQUEADA_SIN_FIRMA
	Editable bool   `json:"editable"` // el usuario actual puede editarla ahora
	Integra  *bool  `json:"integra"`  // el hash coincide con el contenido (sólo firmadas)
}

type ProcedimientoRealizado struct {
	ID              string    `json:"id"`
	ProcedimientoID string    `json:"procedimientoId"`
	Codigo          string    `json:"codigo"`
	Procedimiento   string    `json:"procedimiento"`
	CitaID          *string   `json:"citaId"`
	Diente          *int      `json:"diente"`
	Superficies     []string  `json:"superficies"`
	Observaciones   *string   `json:"observaciones"`
	OdontologoID    string    `json:"odontologoId"`
	Odontologo      string    `json:"odontologo"`
	RealizadoEn     time.Time `json:"realizadoEn"`
	HashFirma       string    `json:"hashFirma"`
	Adendas         []Adenda  `json:"adendas"`
}

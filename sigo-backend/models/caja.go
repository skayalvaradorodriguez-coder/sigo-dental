package models

import (
	"time"

	"sigo-backend/domain"
)

// CajaSesion: lo que ve el recepcionista de SU caja. No tiene totales ni teórico (RN-02).
type CajaSesion struct {
	ID           string     `json:"id"`
	AbiertaEn    time.Time  `json:"abiertaEn"`
	FondoInicial string     `json:"fondoInicial"`
	Estado       string     `json:"estado"`
	Cobros       int        `json:"cobros"` // cantidad, no montos
	CerradaEn    *time.Time `json:"cerradaEn,omitempty"`
}

// CierreCaja: vista del Auditor (caja.revisar), con teórico, declarado y diferencias.
type CierreCaja struct {
	ID           string              `json:"id"`
	CajeroID     string              `json:"cajeroId"`
	Cajero       string              `json:"cajero"`
	AbiertaEn    time.Time           `json:"abiertaEn"`
	CerradaEn    *time.Time          `json:"cerradaEn"`
	Estado       string              `json:"estado"` // ABIERTA | CERRADA | REVISADA
	FondoInicial string              `json:"fondoInicial"`
	NotaCierre   *string             `json:"notaCierre"`
	Diferencias  []domain.Diferencia `json:"diferencias"`
	Cuadra       *bool               `json:"cuadra"`
	Cobros       int                 `json:"cobros"`
	Anulados     int                 `json:"anulados"`
	Seguro       string              `json:"seguro"` // cuentas por cobrar generadas en la sesión
	RevisadaPor  *string             `json:"revisadaPor"`
	RevisadaEn   *time.Time          `json:"revisadaEn"`
	NotaRevision *string             `json:"notaRevision"`
	// Datos crudos para conciliar (no se envían).
	Teorico   map[string]string `json:"-"`
	Declarado map[string]string `json:"-"`
}

type CobroConcepto struct {
	ProcedimientoRealizadoID *string `json:"procedimientoRealizadoId"`
	ProcedimientoID          string  `json:"procedimientoId"`
	Descripcion              string  `json:"descripcion"`
	Monto                    string  `json:"monto"`
}

type CobroPago struct {
	Medio      string  `json:"medio"`
	Monto      string  `json:"monto"`
	Referencia *string `json:"referencia"`
	ConvenioID *string `json:"convenioId"`
	Convenio   *string `json:"convenio"`
}

type Anulacion struct {
	Motivo     string    `json:"motivo"`
	AnuladoPor string    `json:"anuladoPor"`
	AnuladoEn  time.Time `json:"anuladoEn"`
}

type Comprobante struct {
	ID            string    `json:"id"`
	Tipo          string    `json:"tipo"`
	Estado        string    `json:"estado"`
	Numero        *string   `json:"numero"`
	Autorizacion  *string   `json:"autorizacion"`
	Proveedor     string    `json:"proveedor"`
	Simulado      bool      `json:"simulado"`
	Mensaje       *string   `json:"mensaje"`
	EmitidoEn     time.Time `json:"emitidoEn"`
	HashDocumento string    `json:"hashDocumento"`
}

type Cobro struct {
	ID            string          `json:"id"`
	Numero        int64           `json:"numero"`
	SesionID      string          `json:"sesionId"`
	PacienteID    string          `json:"pacienteId"`
	Paciente      string          `json:"paciente"`
	Total         string          `json:"total"`
	RegistradoPor string          `json:"registradoPor"`
	RegistradoEn  time.Time       `json:"registradoEn"`
	Conceptos     []CobroConcepto `json:"conceptos"`
	Pagos         []CobroPago     `json:"pagos"`
	Anulacion     *Anulacion      `json:"anulacion"`
	Comprobantes  []Comprobante   `json:"comprobantes"` // todos los intentos, del más antiguo al más reciente
}

// PorCobrar: procedimiento realizado aún no cobrado, con el monto que corresponde.
type PorCobrar struct {
	ProcedimientoRealizadoID string    `json:"procedimientoRealizadoId"`
	ProcedimientoID          string    `json:"procedimientoId"`
	Codigo                   string    `json:"codigo"`
	Procedimiento            string    `json:"procedimiento"`
	Diente                   *int      `json:"diente"`
	RealizadoEn              time.Time `json:"realizadoEn"`
	Odontologo               string    `json:"odontologo"`
	Monto                    string    `json:"monto"`
	Origen                   string    `json:"origen"` // PRESUPUESTO | ARANCEL
}

type CuentaSeguro struct {
	ConvenioID  string  `json:"convenioId"`
	Convenio    string  `json:"convenio"`
	Aseguradora *string `json:"aseguradora"`
	Cobros      int     `json:"cobros"`
	Total       string  `json:"total"`
}

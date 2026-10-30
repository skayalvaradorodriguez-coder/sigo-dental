package models

import "time"

type Ubicacion struct {
	ID            string  `json:"id"`
	Nombre        string  `json:"nombre"`
	ConsultorioID *string `json:"consultorioId"`
	PorDefecto    bool    `json:"porDefecto"`
	Activo        bool    `json:"activo"`
}

type Insumo struct {
	ID            string `json:"id"`
	Codigo        string `json:"codigo"`
	Nombre        string `json:"nombre"`
	Unidad        string `json:"unidad"`
	CostoUnitario string `json:"costoUnitario"`
	StockMinimo   string `json:"stockMinimo"`
	StockTotal    string `json:"stockTotal"`
	Activo        bool   `json:"activo"`
}

type StockItem struct {
	InsumoID    string `json:"insumoId"`
	Codigo      string `json:"codigo"`
	Insumo      string `json:"insumo"`
	Unidad      string `json:"unidad"`
	UbicacionID string `json:"ubicacionId"`
	Ubicacion   string `json:"ubicacion"`
	Cantidad    string `json:"cantidad"`
	StockMinimo string `json:"stockMinimo"`
	Bajo        bool   `json:"bajo"`     // total del insumo bajo el mínimo
	Negativo    bool   `json:"negativo"` // saldo negativo en la ubicación: hay que regularizar
}

type Movimiento struct {
	ID                       string    `json:"id"`
	InsumoID                 string    `json:"insumoId"`
	Insumo                   string    `json:"insumo"`
	Unidad                   string    `json:"unidad"`
	Ubicacion                string    `json:"ubicacion"`
	Tipo                     string    `json:"tipo"`
	Cantidad                 string    `json:"cantidad"`
	CostoUnitario            string    `json:"costoUnitario"`
	ProcedimientoRealizadoID *string   `json:"procedimientoRealizadoId"`
	Procedimiento            *string   `json:"procedimiento"`
	Motivo                   *string   `json:"motivo"`
	RegistradoPor            string    `json:"registradoPor"`
	RegistradoEn             time.Time `json:"registradoEn"`
}

type LineaReceta struct {
	InsumoID string `json:"insumoId"`
	Codigo   string `json:"codigo"`
	Insumo   string `json:"insumo"`
	Unidad   string `json:"unidad"`
	Cantidad string `json:"cantidad"`
	Costo    string `json:"costo"` // cantidad × costo unitario actual
}

type RecetaConsumo struct {
	ProcedimientoID string        `json:"procedimientoId"`
	Codigo          string        `json:"codigo"`
	Procedimiento   string        `json:"procedimiento"`
	Lineas          []LineaReceta `json:"lineas"`
	CostoEstandar   string        `json:"costoEstandar"`
}

// ---------------------------------------------------------------- Honorarios

type ReglaHonorario struct {
	ID              string    `json:"id"`
	OdontologoID    *string   `json:"odontologoId"`
	Odontologo      *string   `json:"odontologo"`
	ProcedimientoID *string   `json:"procedimientoId"`
	Procedimiento   *string   `json:"procedimiento"`
	Tipo            string    `json:"tipo"`
	Valor           string    `json:"valor"`
	VigenteDesde    string    `json:"vigenteDesde"`
	Descripcion     string    `json:"descripcion"`
	CreadoPor       string    `json:"creadoPor"`
	CreadoEn        time.Time `json:"creadoEn"`
}

// LineaHonorario: un concepto cobrado de un procedimiento realizado por el odontólogo.
type LineaHonorario struct {
	CobroConceptoID          string    `json:"cobroConceptoId"`
	ProcedimientoRealizadoID string    `json:"procedimientoRealizadoId"`
	ProcedimientoID          string    `json:"procedimientoId"`
	Descripcion              string    `json:"descripcion"`
	Recibo                   int64     `json:"recibo"` // n.º de recibo interno (sin datos clínicos del paciente)
	CobradoEn                time.Time `json:"cobradoEn"`
	Monto                    string    `json:"monto"`
	Laboratorio              string    `json:"laboratorio"`
	Materiales               string    `json:"materiales"`
	Base                     string    `json:"base"`
	ReglaID                  *string   `json:"reglaId"`
	Regla                    string    `json:"regla"`
	Honorario                string    `json:"honorario"`
}

type Liquidacion struct {
	ID           string           `json:"id"`
	OdontologoID string           `json:"odontologoId"`
	Odontologo   string           `json:"odontologo"`
	Desde        string           `json:"desde"`
	Hasta        string           `json:"hasta"`
	Total        string           `json:"total"`
	CreadoPor    string           `json:"creadoPor"`
	CreadoEn     time.Time        `json:"creadoEn"`
	Lineas       []LineaHonorario `json:"lineas,omitempty"`
	CantLineas   int              `json:"cantidadLineas"`
}

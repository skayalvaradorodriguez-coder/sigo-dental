// Package facturacion es el puerto hacia la autoridad tributaria (RF-12). El SRS no dice el país
// (pregunta Q-01), así que no se asume ninguna autoridad ni formato (SRI, SAT, DIAN, SUNAT…):
// sólo existe el proveedor SIMULADO, rotulado como tal en la API y en la interfaz. Un proveedor
// real implementaría la misma interfaz con el formato, la firma y el envío que exija su país.
package facturacion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

type Receptor struct {
	Documento string `json:"documento"` // "Cédula DEMO000101"
	Nombre    string `json:"nombre"`
	Email     string `json:"email,omitempty"`
}

type Linea struct {
	Descripcion string `json:"descripcion"`
	Monto       string `json:"monto"` // decimal como texto
}

type Pago struct {
	Medio string `json:"medio"`
	Monto string `json:"monto"`
}

// Documento: lo que se envía al proveedor. Los impuestos dependen del país y de la exención de los
// servicios de salud: no se calculan hasta definir Q-01.
type Documento struct {
	Tipo          string   `json:"tipo"` // FACTURA | NOTA_CREDITO
	Secuencial    int64    `json:"secuencial"`
	Emisor        string   `json:"emisor"`
	Receptor      Receptor `json:"receptor"`
	Lineas        []Linea  `json:"lineas"`
	Pagos         []Pago   `json:"pagos,omitempty"`
	Total         string   `json:"total"`
	Fecha         string   `json:"fecha"`                // ISO-8601
	Referencia    string   `json:"referencia,omitempty"` // nota de crédito: número de la factura que anula
	Motivo        string   `json:"motivo,omitempty"`
	ReciboInterno int64    `json:"reciboInterno"`
}

type Resultado struct {
	Estado       string // AUTORIZADO | RECHAZADO
	Numero       string
	Autorizacion string
	Mensaje      string
}

type Proveedor interface {
	Nombre() string
	Simulado() bool // true ⇒ no tiene validez tributaria; la interfaz lo rotula "SIMULADO"
	Emitir(ctx context.Context, d Documento) (Resultado, error)
}

// Simulado: no se conecta a ninguna autoridad. Autoriza todo documento bien formado con un número
// "SIM-…" y una "autorización" derivada del hash del documento. FallarSiguiente sirve para probar
// el reintento (simula que el servicio no responde).
type Simulado struct {
	mu              sync.Mutex
	FallarSiguiente bool
}

func (s *Simulado) Nombre() string { return "simulado" }
func (s *Simulado) Simulado() bool { return true }

var ErrNoDisponible = errors.New("el servicio de facturación no respondió")

func (s *Simulado) Emitir(_ context.Context, d Documento) (Resultado, error) {
	s.mu.Lock()
	fallar := s.FallarSiguiente
	s.FallarSiguiente = false
	s.mu.Unlock()
	if fallar {
		return Resultado{}, ErrNoDisponible
	}
	if d.Secuencial <= 0 || d.Total == "" || len(d.Lineas) == 0 || d.Receptor.Nombre == "" {
		return Resultado{Estado: "RECHAZADO", Mensaje: "Documento incompleto."}, nil
	}
	if d.Tipo == "NOTA_CREDITO" && d.Referencia == "" {
		return Resultado{Estado: "RECHAZADO", Mensaje: "La nota de crédito debe referir a una factura."}, nil
	}
	prefijo := map[string]string{"FACTURA": "F", "NOTA_CREDITO": "NC"}[d.Tipo]
	if prefijo == "" {
		return Resultado{Estado: "RECHAZADO", Mensaje: "Tipo de comprobante desconocido."}, nil
	}
	h := sha256.Sum256(fmt.Appendf(nil, "%s|%d|%s|%s|%s", d.Tipo, d.Secuencial, d.Receptor.Documento, d.Total, d.Fecha))
	return Resultado{Estado: "AUTORIZADO", Numero: fmt.Sprintf("SIM-%s-%08d", prefijo, d.Secuencial),
		Autorizacion: "SIM" + hex.EncodeToString(h[:])[:29], Mensaje: "Autorizado por el proveedor SIMULADO (sin validez tributaria)."}, nil
}

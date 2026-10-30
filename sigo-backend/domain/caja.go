package domain

import "fmt"

// ---------------------------------------------------------------- Caja (RF-12, RN-02)

const (
	MedioEfectivo      = "EFECTIVO"
	MedioTarjeta       = "TARJETA"
	MedioTransferencia = "TRANSFERENCIA"
	MedioSeguro        = "SEGURO"
)

var MediosPago = []string{MedioEfectivo, MedioTarjeta, MedioTransferencia, MedioSeguro}

// MediosEnCaja: lo que el recepcionista cuenta al cerrar. El seguro no entra a la caja: es una
// cuenta por cobrar a la aseguradora (S-06).
var MediosEnCaja = []string{MedioEfectivo, MedioTarjeta, MedioTransferencia}

type PagoMedio struct {
	Medio string
	Monto Centavos
}

// ValidarPagoMixto: cada línea con monto positivo y la suma igual al total, al centavo.
func ValidarPagoMixto(total Centavos, pagos []PagoMedio) error {
	if len(pagos) == 0 {
		return fmt.Errorf("registre al menos un medio de pago")
	}
	var suma Centavos
	for _, p := range pagos {
		if p.Monto <= 0 {
			return fmt.Errorf("cada pago debe ser mayor que cero")
		}
		suma += p.Monto
	}
	if suma != total {
		return fmt.Errorf("los pagos suman %s y el total es %s", suma, total)
	}
	return nil
}

// Diferencia por medio: declarado − teórico (positivo = sobrante, negativo = faltante).
type Diferencia struct {
	Medio      string `json:"medio"`
	Teorico    string `json:"teorico"`
	Declarado  string `json:"declarado"`
	Diferencia string `json:"diferencia"`
	Cuadra     bool   `json:"cuadra"`
}

// ConciliarCierre compara lo declarado con lo calculado, con una tolerancia en centavos (S-05).
func ConciliarCierre(teorico, declarado map[string]Centavos, toleranciaCentavos int64) ([]Diferencia, bool) {
	out := []Diferencia{}
	todo := true
	for _, m := range MediosEnCaja {
		d := declarado[m] - teorico[m]
		abs := d
		if abs < 0 {
			abs = -abs
		}
		ok := int64(abs) <= toleranciaCentavos
		todo = todo && ok
		out = append(out, Diferencia{Medio: m, Teorico: teorico[m].String(), Declarado: declarado[m].String(), Diferencia: d.String(), Cuadra: ok})
	}
	return out, todo
}

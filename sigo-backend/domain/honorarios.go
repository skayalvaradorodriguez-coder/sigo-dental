package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------- Honorarios (RF-13)

const (
	ReglaPorcentaje     = "PORCENTAJE"      // % fijo sobre lo cobrado
	ReglaMontoFijo      = "MONTO_FIJO"      // monto fijo por procedimiento
	ReglaPorcentajeNeto = "PORCENTAJE_NETO" // % sobre lo cobrado menos laboratorio y materiales
)

var TiposRegla = []string{ReglaPorcentaje, ReglaMontoFijo, ReglaPorcentajeNeto}

// Regla: OdontologoID / ProcedimientoID vacíos = aplica a todos. Valor: porcentaje con 2 decimales
// expresado en centésimas (40.5 % = 4050) o monto en centavos.
type Regla struct {
	ID, OdontologoID, ProcedimientoID, Tipo string
	Valor                                   Centavos
	VigenteDesde                            time.Time
}

func (r Regla) especificidad() int {
	n := 0
	if r.OdontologoID != "" {
		n += 2
	}
	if r.ProcedimientoID != "" {
		n++
	}
	return n
}

// ElegirRegla: entre las reglas vigentes a la fecha que alcanzan al odontólogo y al procedimiento,
// gana la más específica (odontólogo + procedimiento > odontólogo > procedimiento > general) y,
// a igual especificidad, la de vigencia más reciente; a igual vigencia, la registrada después
// (las reglas llegan en orden de creación).
func ElegirRegla(reglas []Regla, odontologo, procedimiento string, fecha time.Time) *Regla {
	var mejor *Regla
	dia := time.Date(fecha.Year(), fecha.Month(), fecha.Day(), 0, 0, 0, 0, time.UTC)
	for i := range reglas {
		r := &reglas[i]
		if (r.OdontologoID != "" && r.OdontologoID != odontologo) || (r.ProcedimientoID != "" && r.ProcedimientoID != procedimiento) || r.VigenteDesde.After(dia) {
			continue
		}
		if mejor == nil || r.especificidad() > mejor.especificidad() ||
			(r.especificidad() == mejor.especificidad() && !r.VigenteDesde.Before(mejor.VigenteDesde)) {
			mejor = r
		}
	}
	return mejor
}

// porcentajeDe: base × (valor/100)%, redondeado al centavo (mitad hacia arriba).
func porcentajeDe(base, valor Centavos) Centavos {
	if base <= 0 {
		return 0
	}
	return Centavos((int64(base)*int64(valor) + 5000) / 10000)
}

// CalcularHonorario devuelve la base usada y el honorario.
func CalcularHonorario(r Regla, cobrado, laboratorio, materiales Centavos) (base, honorario Centavos) {
	switch r.Tipo {
	case ReglaMontoFijo:
		return cobrado, r.Valor
	case ReglaPorcentajeNeto:
		base = cobrado - laboratorio - materiales
		if base < 0 {
			base = 0
		}
		return base, porcentajeDe(base, r.Valor)
	default:
		return cobrado, porcentajeDe(cobrado, r.Valor)
	}
}

func fmtPorcentaje(v Centavos) string {
	s := strings.TrimSuffix(strings.TrimSuffix(v.String(), "0"), ".0")
	return strings.TrimSuffix(s, ".") + " %"
}

// DescribirRegla: texto legible para la liquidación.
func DescribirRegla(r Regla) string {
	switch r.Tipo {
	case ReglaMontoFijo:
		return fmt.Sprintf("monto fijo de %s por procedimiento", r.Valor)
	case ReglaPorcentajeNeto:
		return fmt.Sprintf("%s del neto (cobrado − laboratorio − materiales)", fmtPorcentaje(r.Valor))
	}
	return fmt.Sprintf("%s de lo cobrado", fmtPorcentaje(r.Valor))
}

// ---------------------------------------------------------------- Inventario (RF-14)

// Cantidad en milésimas (3 decimales), como NUMERIC(12,3) en la BD.
type Cantidad int64

// ParseCantidad acepta "2", "0.5", "-1.25" (signo sólo si permitirNegativa).
func ParseCantidad(s string, permitirNegativa bool) (Cantidad, error) {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "-")
	if neg {
		if !permitirNegativa {
			return 0, fmt.Errorf("la cantidad debe ser positiva")
		}
		s = s[1:]
	}
	ent, dec, punto := strings.Cut(s, ".")
	if ent == "" || len(ent) > 9 || (punto && (dec == "" || len(dec) > 3)) {
		return 0, fmt.Errorf("cantidad inválida")
	}
	for _, r := range ent + dec {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("cantidad inválida")
		}
	}
	for len(dec) < 3 {
		dec += "0"
	}
	e, _ := strconv.ParseInt(ent, 10, 64)
	d, _ := strconv.ParseInt(dec, 10, 64)
	c := Cantidad(e*1000 + d)
	if c == 0 {
		return 0, fmt.Errorf("la cantidad no puede ser cero")
	}
	if neg {
		c = -c
	}
	return c, nil
}

func (c Cantidad) String() string {
	signo := ""
	if c < 0 {
		signo, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%03d", signo, c/1000, c%1000)
}

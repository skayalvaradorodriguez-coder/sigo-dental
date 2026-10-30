package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------- Dinero
//
// Los montos se manejan en CENTAVOS (int64): nunca float, para que las sumas cuadren al centavo.
// En JSON viajan como texto con 2 decimales ("45.00"), igual que el arancel del catálogo.

type Centavos int64

// ParseDinero acepta "45", "45.5" o "45.50" (punto decimal, sin signo, hasta 2 decimales).
func ParseDinero(s string) (Centavos, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("monto vacío")
	}
	entero, dec, hayPunto := strings.Cut(s, ".")
	if entero == "" || len(entero) > 10 || (hayPunto && (dec == "" || len(dec) > 2)) {
		return 0, fmt.Errorf("monto inválido: %q", s)
	}
	for _, r := range entero + dec {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("monto inválido: %q", s)
		}
	}
	e, _ := strconv.ParseInt(entero, 10, 64)
	for len(dec) < 2 {
		dec += "0"
	}
	d, _ := strconv.ParseInt(dec, 10, 64)
	return Centavos(e*100 + d), nil
}

func (c Centavos) String() string {
	signo := ""
	if c < 0 {
		signo, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", signo, c/100, c%100)
}

// ---------------------------------------------------------------- Presupuestos (RF-07)

type ItemPresupuesto struct {
	Fase                         int
	Honorario, Laboratorio, Desc Centavos
	Decision                     string // PENDIENTE | APROBADO | RECHAZADO
}

// Subtotal de un ítem: honorario + laboratorio − descuento.
func (i ItemPresupuesto) Subtotal() Centavos { return i.Honorario + i.Laboratorio - i.Desc }

type TotalesFase struct {
	Fase        int    `json:"fase"`
	Honorarios  string `json:"honorarios"`
	Laboratorio string `json:"laboratorio"`
	Descuentos  string `json:"descuentos"`
	Total       string `json:"total"`
}

type Totales struct {
	Fases       []TotalesFase `json:"fases"`
	Honorarios  string        `json:"honorarios"`
	Laboratorio string        `json:"laboratorio"`
	Descuentos  string        `json:"descuentos"`
	Total       string        `json:"total"`
	Aprobado    string        `json:"aprobado"` // suma de los ítems aprobados
}

// CalcularTotales suma por fase (en el orden en que aparecen) y en total.
func CalcularTotales(items []ItemPresupuesto) Totales {
	type acum struct{ h, l, d Centavos }
	por := map[int]*acum{}
	orden := []int{}
	var t acum
	var aprobado Centavos
	for _, it := range items {
		a, ok := por[it.Fase]
		if !ok {
			a = &acum{}
			por[it.Fase] = a
			orden = append(orden, it.Fase)
		}
		a.h += it.Honorario
		a.l += it.Laboratorio
		a.d += it.Desc
		t.h += it.Honorario
		t.l += it.Laboratorio
		t.d += it.Desc
		if it.Decision == "APROBADO" {
			aprobado += it.Subtotal()
		}
	}
	out := Totales{Fases: []TotalesFase{}, Honorarios: t.h.String(), Laboratorio: t.l.String(), Descuentos: t.d.String(),
		Total: (t.h + t.l - t.d).String(), Aprobado: aprobado.String()}
	for _, f := range orden {
		a := por[f]
		out.Fases = append(out.Fases, TotalesFase{Fase: f, Honorarios: a.h.String(), Laboratorio: a.l.String(),
			Descuentos: a.d.String(), Total: (a.h + a.l - a.d).String()})
	}
	return out
}

// RequiereAutorizacion: el descuento supera el tope (porcentaje del honorario).
// Se compara en enteros: desc × 100 > honorario × tope.
func RequiereAutorizacion(honorario, descuento Centavos, topePorcentaje int) bool {
	return int64(descuento)*100 > int64(honorario)*int64(topePorcentaje)
}

// EstadoDecision: resultado de la decisión del paciente sobre todos los ítems (RF-08).
func EstadoDecision(decisiones []string) string {
	aprob, rech := 0, 0
	for _, d := range decisiones {
		switch d {
		case "APROBADO":
			aprob++
		case "RECHAZADO":
			rech++
		}
	}
	switch {
	case aprob == len(decisiones) && aprob > 0:
		return "APROBADO"
	case aprob == 0:
		return "RECHAZADO"
	default:
		return "APROBADO_PARCIAL"
	}
}

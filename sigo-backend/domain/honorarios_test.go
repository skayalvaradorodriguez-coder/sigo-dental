package domain

import (
	"testing"
	"time"
)

func dia(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }

func TestTresReglasDeHonorarios(t *testing.T) {
	// Cobrado 200.00, laboratorio 30.00, materiales 12.50.
	casos := []struct {
		r          Regla
		base, hono Centavos
	}{
		{Regla{Tipo: ReglaPorcentaje, Valor: 4000}, 20000, 8000},     // 40 % de 200 = 80.00
		{Regla{Tipo: ReglaMontoFijo, Valor: 2500}, 20000, 2500},      // 25.00 fijo
		{Regla{Tipo: ReglaPorcentajeNeto, Valor: 5000}, 15750, 7875}, // 50 % de (200 − 30 − 12.50) = 78.75
		{Regla{Tipo: ReglaPorcentaje, Valor: 3333}, 20000, 6666},     // 33.33 % de 200 = 66.66
		{Regla{Tipo: ReglaPorcentajeNeto, Valor: 3750}, 15750, 5906}, // 37.5 % de 157.50 = 59.0625 → 59.06
	}
	for i, c := range casos {
		b, h := CalcularHonorario(c.r, 20000, 3000, 1250)
		if b != c.base || h != c.hono {
			t.Errorf("caso %d (%s): base %s honorario %s; se esperaba %s y %s", i, c.r.Tipo, b, h, c.base, c.hono)
		}
	}
	// Redondeo mitad hacia arriba: 10 % de 0.05 = 0.005 → 0.01
	if _, h := CalcularHonorario(Regla{Tipo: ReglaPorcentaje, Valor: 1000}, 5, 0, 0); h != 1 {
		t.Errorf("redondeo: %s", h)
	}
	// Neto negativo ⇒ 0
	if b, h := CalcularHonorario(Regla{Tipo: ReglaPorcentajeNeto, Valor: 5000}, 1000, 3000, 0); b != 0 || h != 0 {
		t.Errorf("neto negativo: %s %s", b, h)
	}
	if DescribirRegla(Regla{Tipo: ReglaPorcentajeNeto, Valor: 5000}) != "50 % del neto (cobrado − laboratorio − materiales)" ||
		DescribirRegla(Regla{Tipo: ReglaPorcentaje, Valor: 3750}) != "37.5 % de lo cobrado" ||
		DescribirRegla(Regla{Tipo: ReglaMontoFijo, Valor: 1200}) != "monto fijo de 12.00 por procedimiento" {
		t.Error("descripciones")
	}
}

func TestReglaMasEspecificaYVigente(t *testing.T) {
	reglas := []Regla{
		{ID: "general", Tipo: ReglaPorcentaje, Valor: 4000, VigenteDesde: dia("2026-01-01")},
		{ID: "endo", ProcedimientoID: "END", Tipo: ReglaPorcentaje, Valor: 4500, VigenteDesde: dia("2026-01-01")},
		{ID: "valeria", OdontologoID: "V", Tipo: ReglaPorcentaje, Valor: 4200, VigenteDesde: dia("2026-01-01")},
		{ID: "valeria-endo", OdontologoID: "V", ProcedimientoID: "END", Tipo: ReglaPorcentajeNeto, Valor: 5000, VigenteDesde: dia("2026-01-01")},
		{ID: "general-nueva", Tipo: ReglaPorcentaje, Valor: 3500, VigenteDesde: dia("2026-07-01")},
		{ID: "futura", OdontologoID: "J", Tipo: ReglaMontoFijo, Valor: 1, VigenteDesde: dia("2027-01-01")},
	}
	casos := []struct {
		odo, proc, fecha, quiere string
	}{
		{"V", "END", "2026-10-30", "valeria-endo"},
		{"V", "PRO", "2026-10-30", "valeria"},
		{"J", "END", "2026-10-30", "endo"},
		{"J", "PRO", "2026-10-30", "general-nueva"},
		{"J", "PRO", "2026-03-01", "general"}, // antes de la vigencia de la nueva
		{"J", "PRO", "2026-12-31", "general-nueva"},
	}
	for _, c := range casos {
		r := ElegirRegla(reglas, c.odo, c.proc, dia(c.fecha))
		if r == nil || r.ID != c.quiere {
			t.Errorf("%s/%s/%s: %+v, se esperaba %s", c.odo, c.proc, c.fecha, r, c.quiere)
		}
	}
	if ElegirRegla(reglas[5:], "J", "PRO", dia("2026-10-30")) != nil {
		t.Error("una regla futura no aplica")
	}
}

func TestCantidades(t *testing.T) {
	ok := map[string]string{"2": "2.000", "0.5": "0.500", "1.25": "1.250", "0.001": "0.001"}
	for in, out := range ok {
		c, err := ParseCantidad(in, false)
		if err != nil || c.String() != out {
			t.Errorf("%s: %v %v", in, c, err)
		}
	}
	if c, err := ParseCantidad("-3.5", true); err != nil || c.String() != "-3.500" {
		t.Errorf("ajuste negativo: %v %v", c, err)
	}
	for _, mal := range []string{"", "0", "-1", "1.2345", "abc", "1,5", "."} {
		if _, err := ParseCantidad(mal, false); err == nil {
			t.Errorf("%q debía fallar", mal)
		}
	}
}

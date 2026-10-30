package domain

import "testing"

// Tabla completa FDI ↔ Universal (52 dientes) — RF-03.
func TestFDIUniversalTablaCompleta(t *testing.T) {
	esperado := map[int]string{
		18: "1", 17: "2", 16: "3", 15: "4", 14: "5", 13: "6", 12: "7", 11: "8",
		21: "9", 22: "10", 23: "11", 24: "12", 25: "13", 26: "14", 27: "15", 28: "16",
		38: "17", 37: "18", 36: "19", 35: "20", 34: "21", 33: "22", 32: "23", 31: "24",
		41: "25", 42: "26", 43: "27", 44: "28", 45: "29", 46: "30", 47: "31", 48: "32",
		55: "A", 54: "B", 53: "C", 52: "D", 51: "E", 61: "F", 62: "G", 63: "H", 64: "I", 65: "J",
		75: "K", 74: "L", 73: "M", 72: "N", 71: "O", 81: "P", 82: "Q", 83: "R", 84: "S", 85: "T",
	}
	if len(DientesPermanentes) != 32 || len(DientesTemporales) != 20 || len(esperado) != 52 {
		t.Fatalf("cantidades: %d permanentes, %d temporales", len(DientesPermanentes), len(DientesTemporales))
	}
	for fdi, u := range esperado {
		got, err := FDIaUniversal(fdi)
		if err != nil || got != u {
			t.Errorf("FDI %d → %q (%v), se esperaba %q", fdi, got, err, u)
		}
		back, err := UniversalAFDI(u)
		if err != nil || back != fdi {
			t.Errorf("Universal %q → %d (%v), se esperaba %d", u, back, err, fdi)
		}
	}
	for _, d := range append(append([]int{}, DientesPermanentes...), DientesTemporales...) {
		if _, ok := esperado[d]; !ok {
			t.Errorf("diente %d de la lista no está en la tabla", d)
		}
	}
	for _, malo := range []int{0, 10, 19, 49, 56, 66, 90, 9} {
		if DienteValido(malo) {
			t.Errorf("%d no debería ser válido", malo)
		}
		if _, err := FDIaUniversal(malo); err == nil {
			t.Errorf("FDIaUniversal(%d) debería fallar", malo)
		}
	}
	for _, malo := range []string{"0", "33", "U", "", "AA"} {
		if _, err := UniversalAFDI(malo); err == nil {
			t.Errorf("UniversalAFDI(%q) debería fallar", malo)
		}
	}
}

func TestSuperficiesYEtiquetas(t *testing.T) {
	for in, want := range map[string]string{"m": "M", "I": "O", "o": "O", "P": "L", "l": "L", "v": "V", "D": "D"} {
		if got, ok := NormalizarSuperficie(in); !ok || got != want {
			t.Errorf("%s → %s", in, got)
		}
	}
	if _, ok := NormalizarSuperficie("X"); ok {
		t.Error("X no es superficie")
	}
	casos := []struct {
		d        int
		s, label string
	}{{11, "O", "I"}, {16, "O", "O"}, {16, "L", "P"}, {46, "L", "L"}, {53, "O", "I"}, {83, "L", "L"}, {64, "L", "P"}}
	for _, c := range casos {
		if got := EtiquetaSuperficie(c.d, c.s); got != c.label {
			t.Errorf("diente %d %s → %s, se esperaba %s", c.d, c.s, got, c.label)
		}
	}
}

func TestValidarMarca(t *testing.T) {
	if s, e := ValidarMarca(11, "I", "CARIES"); e != "" || s != "O" {
		t.Errorf("caries incisal en 11: %q %q", s, e)
	}
	if _, e := ValidarMarca(16, "", "CARIES"); e == "" {
		t.Error("caries sin superficie debe rechazarse")
	}
	if _, e := ValidarMarca(16, "M", "ENDODONCIA_REQUERIDA"); e == "" {
		t.Error("endodoncia con superficie debe rechazarse")
	}
	if _, e := ValidarMarca(54, "", "IMPLANTE"); e == "" {
		t.Error("implante en temporal debe rechazarse")
	}
	if _, e := ValidarMarca(36, "M", "SELLANTE"); e == "" {
		t.Error("sellante fuera de oclusal debe rechazarse")
	}
	if _, e := ValidarMarca(19, "M", "CARIES"); e == "" {
		t.Error("diente 19 no existe")
	}
	if _, e := ValidarMarca(16, "", "FRACTURA"); e == "" {
		t.Error("hallazgo desconocido")
	}
}

func TestEstadoActualConservaHistorial(t *testing.T) {
	ev := []EventoOdontograma{
		{16, "O", "CARIES", "AGREGAR"},
		{16, "M", "CARIES", "AGREGAR"},
		{16, "O", "CARIES", "QUITAR"},        // se trató
		{16, "O", "RESTAURACION", "AGREGAR"}, // y se restauró
		{21, "", "AUSENTE", "AGREGAR"},
	}
	got := EstadoActual(ev)
	if len(got) != 3 || got[0].Superficie != "M" || got[1].Hallazgo != "RESTAURACION" || got[2].Diente != 21 {
		t.Fatalf("estado: %+v", got)
	}
}

// Ejemplos de 03-modelo-datos §3.5 — RF-04.
func TestNICYResumen(t *testing.T) {
	if NIC(4, 2) != 6 || NIC(5, -2) != 3 || NIC(3, 0) != 3 {
		t.Fatal("NIC = PS + MG")
	}
	r := Resumir([]SitioPerio{{3, 0, false}, {5, 1, true}, {4, -1, true}, {2, 0, false}})
	if r.Sitios != 4 || r.PorcentajeSangrado != 50 || r.BolsasMayor4 != 2 || r.NICMaximo != 6 {
		t.Errorf("resumen: %+v", r)
	}
	if !SitioValido("DL") || SitioValido("X") {
		t.Error("sitios")
	}
}

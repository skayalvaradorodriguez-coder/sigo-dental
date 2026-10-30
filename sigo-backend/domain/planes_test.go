package domain

import "testing"

func TestDineroEnCentavos(t *testing.T) {
	for in, want := range map[string]Centavos{"45": 4500, "45.5": 4550, "45.50": 4550, "0.01": 1, "1234567890.99": 123456789099} {
		got, err := ParseDinero(in)
		if err != nil || got != want {
			t.Errorf("%q → %d (%v), se esperaba %d", in, got, err, want)
		}
	}
	for _, malo := range []string{"", "-1", "1.234", "1,50", "abc", ".5", "1.", "12345678901"} {
		if _, err := ParseDinero(malo); err == nil {
			t.Errorf("%q debería rechazarse", malo)
		}
	}
	if Centavos(4550).String() != "45.50" || Centavos(7).String() != "0.07" || Centavos(-150).String() != "-1.50" {
		t.Error("formato")
	}
}

// RF-07: totales por fase con honorarios, laboratorio y descuentos; sin errores de coma flotante.
func TestTotalesPorFase(t *testing.T) {
	items := []ItemPresupuesto{
		{Fase: 1, Honorario: 3500, Decision: "APROBADO"},                      // profilaxis 35.00
		{Fase: 2, Honorario: 4500, Desc: 450, Decision: "APROBADO"},           // resina 45.00 − 4.50
		{Fase: 2, Honorario: 18000, Laboratorio: 6000, Decision: "RECHAZADO"}, // corona: 180 + 60 lab
		{Fase: 1, Honorario: 10, Laboratorio: 20, Decision: "APROBADO"},       // 0.10 + 0.20 = 0.30 exactos
	}
	tot := CalcularTotales(items)
	// 35.00 + 45.00 + 180.00 + 0.10 = 260.10 de honorarios; 60.20 de laboratorio; 4.50 de descuento.
	if tot.Total != "315.80" || tot.Honorarios != "260.10" || tot.Laboratorio != "60.20" || tot.Descuentos != "4.50" {
		t.Errorf("totales: %+v", tot)
	}
	if tot.Aprobado != "75.80" {
		t.Errorf("aprobado: %s", tot.Aprobado)
	}
	if len(tot.Fases) != 2 || tot.Fases[0].Total != "35.30" || tot.Fases[1].Total != "280.50" {
		t.Errorf("fases: %+v", tot.Fases)
	}
}

func TestTopeDeDescuento(t *testing.T) {
	if RequiereAutorizacion(10000, 1000, 10) || !RequiereAutorizacion(10000, 1001, 10) || !RequiereAutorizacion(10000, 1, 0) {
		t.Error("tope 10 %: 10.00 de 100.00 no requiere; 10.01 sí; con tope 0 cualquier descuento requiere")
	}
}

// RF-08: aprobación total, parcial o rechazo.
func TestEstadoDecision(t *testing.T) {
	casos := map[string][]string{
		"APROBADO":         {"APROBADO", "APROBADO"},
		"APROBADO_PARCIAL": {"APROBADO", "RECHAZADO", "APROBADO"},
		"RECHAZADO":        {"RECHAZADO", "RECHAZADO"},
	}
	for want, ds := range casos {
		if got := EstadoDecision(ds); got != want {
			t.Errorf("%v → %s", ds, got)
		}
	}
}

func TestAlertasReceta(t *testing.T) {
	d := &DatosAlerta{Alergias: []Alergia{{Sustancia: "Penicilina", Tipo: "MEDICAMENTO"}, {Sustancia: "Maní", Tipo: "ALIMENTO"}}, Anticoagulado: true}
	al := AlertasReceta([]string{"Amoxicilina 500 mg", "Paracetamol", "Ibuprofeno", "Penicilina G"}, d)
	motivos := map[string]int{}
	for _, a := range al {
		motivos[a.Medicamento]++
	}
	if motivos["Amoxicilina 500 mg"] != 1 || motivos["Paracetamol"] != 0 || motivos["Ibuprofeno"] != 1 || motivos["Penicilina G"] != 1 {
		t.Errorf("alertas: %+v", al)
	}
	if len(AlertasReceta([]string{"Amoxicilína"}, d)) != 1 {
		t.Error("debe ignorar tildes")
	}
	if len(AlertasReceta([]string{"Amoxicilina"}, nil)) != 0 {
		t.Error("sin anamnesis no hay datos para comparar")
	}
}

package domain

import "testing"

func TestPagoMixtoCuadraAlCentavo(t *testing.T) {
	// 0.10 + 0.20 = 0.30 exacto en centavos (en float daría 0.30000000000000004).
	if err := ValidarPagoMixto(30, []PagoMedio{{MedioEfectivo, 10}, {MedioTarjeta, 20}}); err != nil {
		t.Fatal(err)
	}
	casos := []struct {
		total Centavos
		pagos []PagoMedio
	}{
		{100, nil},
		{100, []PagoMedio{{MedioEfectivo, 99}}},
		{100, []PagoMedio{{MedioEfectivo, 101}}},
		{100, []PagoMedio{{MedioEfectivo, 150}, {MedioTarjeta, -50}}},
		{100, []PagoMedio{{MedioEfectivo, 100}, {MedioTarjeta, 0}}},
	}
	for i, c := range casos {
		if ValidarPagoMixto(c.total, c.pagos) == nil {
			t.Errorf("caso %d debía fallar", i)
		}
	}
	// Efectivo + tarjeta + transferencia + seguro = 350.00
	if err := ValidarPagoMixto(35000, []PagoMedio{{MedioEfectivo, 5025}, {MedioTarjeta, 12000}, {MedioTransferencia, 7975}, {MedioSeguro, 10000}}); err != nil {
		t.Error(err)
	}
}

func TestConciliarCierreCiego(t *testing.T) {
	teo := map[string]Centavos{MedioEfectivo: 12550, MedioTarjeta: 8000, MedioTransferencia: 0}
	d, ok := ConciliarCierre(teo, map[string]Centavos{MedioEfectivo: 12550, MedioTarjeta: 8000}, 0)
	if !ok || len(d) != 3 || d[0].Diferencia != "0.00" {
		t.Errorf("cuadrada: %v %v", ok, d)
	}
	d, ok = ConciliarCierre(teo, map[string]Centavos{MedioEfectivo: 12000, MedioTarjeta: 8000}, 0)
	if ok || d[0].Diferencia != "-5.50" || d[0].Cuadra || !d[1].Cuadra {
		t.Errorf("faltante de 5.50: %v %v", ok, d)
	}
	if _, ok = ConciliarCierre(teo, map[string]Centavos{MedioEfectivo: 12600, MedioTarjeta: 8000}, 50); !ok {
		t.Error("un sobrante de 0.50 cabe en una tolerancia de 50 centavos")
	}
	if _, ok = ConciliarCierre(teo, map[string]Centavos{MedioEfectivo: 12601, MedioTarjeta: 8000}, 50); ok {
		t.Error("0.51 supera la tolerancia de 0.50")
	}
}

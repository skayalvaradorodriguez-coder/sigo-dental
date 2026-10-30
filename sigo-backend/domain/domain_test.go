package domain

import (
	"testing"
	"time"
)

func TestAlertasPenicilinaYAnticoagulado(t *testing.T) {
	a := CalcularAlertas(&DatosAlerta{Alergias: []Alergia{{Sustancia: "Penicilina", Tipo: "MEDICAMENTO"}}})
	if len(a) != 1 || a[0].Severidad != "CRITICA" || a[0].Etiqueta != "Alergia a Penicilina" {
		t.Fatalf("penicilina debe ser crítica: %+v", a)
	}
	b := CalcularAlertas(&DatosAlerta{Anticoagulado: true, Coagulopatia: true})
	if len(b) != 2 || b[0].Severidad != "CRITICA" || b[1].Severidad != "CRITICA" {
		t.Fatalf("anticoagulado y coagulopatía críticas: %+v", b)
	}
	if len(CalcularAlertas(nil)) != 0 {
		t.Error("sin anamnesis no hay alertas")
	}
}

func TestAlertasOrdenCriticasPrimero(t *testing.T) {
	si := true
	a := CalcularAlertas(&DatosAlerta{
		EnfermedadesSistemicas: []string{"Hipertensión"},
		Embarazo:               &si,
		Alergias:               []Alergia{{Sustancia: "Maní", Tipo: "ALIMENTO"}, {Sustancia: "Látex", Tipo: "MATERIAL", Reaccion: "urticaria"}},
		OtraCondicionCritica:   "  Marcapasos ",
	})
	if a[0].Etiqueta != "Alergia a Látex (urticaria)" || a[1].Etiqueta != "Marcapasos" {
		t.Fatalf("orden inesperado: %+v", a)
	}
	for _, x := range a[2:] {
		if x.Severidad != "ADVERTENCIA" {
			t.Errorf("debió ser advertencia: %+v", x)
		}
	}
	if got := SoloCriticas(a); len(got) != 2 {
		t.Errorf("SoloCriticas: %v", got)
	}
}

func TestTransicionesCita(t *testing.T) {
	if len(Transiciones) != 7 {
		t.Fatal("deben existir los 7 estados del SRS")
	}
	validas := [][2]string{{Programado, Confirmado}, {Confirmado, EnSalaEspera}, {EnSalaEspera, EnAtencion}, {EnAtencion, Atendido}, {Programado, Cancelado}, {Confirmado, NoAsistio}}
	for _, v := range validas {
		if !TransicionValida(v[0], v[1]) {
			t.Errorf("debe permitir %s → %s", v[0], v[1])
		}
	}
	invalidas := [][2]string{{Atendido, Programado}, {Cancelado, Confirmado}, {NoAsistio, EnSalaEspera}, {EnAtencion, Cancelado}, {Programado, Atendido}}
	for _, v := range invalidas {
		if TransicionValida(v[0], v[1]) {
			t.Errorf("debe rechazar %s → %s", v[0], v[1])
		}
	}
	if OcupaSillon(Cancelado) || OcupaSillon(NoAsistio) || !OcupaSillon(Atendido) {
		t.Error("cancelado y no asistió liberan el sillón")
	}
	if !Reprogramable(Confirmado) || Reprogramable(EnAtencion) {
		t.Error("reprogramación sólo en Programado/Confirmado")
	}
}

func TestEdadYDiaLocal(t *testing.T) {
	n := time.Date(2008, 11, 15, 0, 0, 0, 0, time.UTC)
	if Edad(n, time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)) != 17 || Edad(n, time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)) != 18 {
		t.Error("edad incorrecta en el cumpleaños")
	}
	gye, _ := time.LoadLocation("America/Guayaquil")
	ini, fin := DiaLocal(time.Date(2026, 10, 30, 2, 0, 0, 0, time.UTC), gye) // 21:00 del 29 en Guayaquil
	if ini != time.Date(2026, 10, 29, 5, 0, 0, 0, time.UTC) || fin != time.Date(2026, 10, 30, 5, 0, 0, 0, time.UTC) {
		t.Errorf("día local incorrecto: %v %v", ini, fin)
	}
}

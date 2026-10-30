package notificaciones

import (
	"context"
	"strings"
	"testing"
	"time"
)

func tel(s string) *string { return &s }

func TestPlanificarRecordatorios(t *testing.T) {
	ahora := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	inicio := ahora.Add(72 * time.Hour)
	p := Planificar(inicio, Contacto{Canal: "WHATSAPP", Telefono: tel("+000 1"), Autoriza: true}, ahora)
	if len(p) != 3 {
		t.Fatalf("deben ser 3 mensajes, hay %d", len(p))
	}
	if !p[1].ProgramadaPara.Equal(inicio.Add(-48*time.Hour)) || !p[2].ProgramadaPara.Equal(inicio.Add(-24*time.Hour)) {
		t.Error("horarios −48 h / −24 h incorrectos")
	}
	for _, x := range p {
		if x.Estado != "PENDIENTE" {
			t.Errorf("%s debería estar pendiente", x.Tipo)
		}
	}
}

func TestPlanificarOmiteLoQueNoCabe(t *testing.T) {
	ahora := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	p := Planificar(ahora.Add(30*time.Hour), Contacto{Canal: "SMS", Telefono: tel("+000 1"), Autoriza: true}, ahora)
	if p[1].Estado != "OMITIDO" || !strings.Contains(*p[1].Motivo, "48 h") || p[2].Estado != "PENDIENTE" {
		t.Errorf("cita en 30 h: 48 h omitido y 24 h pendiente; got %+v", p)
	}
	sin := Planificar(ahora.Add(96*time.Hour), Contacto{Canal: "WHATSAPP", Telefono: tel("+000 1"), Autoriza: false}, ahora)
	for _, x := range sin {
		if x.Estado != "OMITIDO" {
			t.Error("sin autorización todo se omite")
		}
	}
	correo := Planificar(ahora.Add(96*time.Hour), Contacto{Canal: "EMAIL", Telefono: tel("+000 1"), Autoriza: true}, ahora)
	if correo[0].Estado != "OMITIDO" || !strings.Contains(*correo[0].Motivo, "correo") {
		t.Error("canal correo sin email debe omitirse")
	}
}

func TestPlantillaSinDatosClinicos(t *testing.T) {
	zona, _ := time.LoadLocation("America/Guayaquil")
	m := Plantilla(Confirmacion, DatosPlantilla{Nombre: "Ana", Clinica: "Clínica Demo", Sede: "Central", Odontologo: "Dra. Ríos",
		Inicio: time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC), Zona: zona})
	if !strings.Contains(m, "viernes 30 de octubre a las 09:00") || !strings.Contains(m, "Ana") {
		t.Errorf("mensaje inesperado: %s", m)
	}
	s := &Simulada{}
	id, err := s.Enviar(context.Background(), Mensaje{Canal: "SMS", Destinatario: "+1", Texto: m})
	if err != nil || !strings.HasPrefix(id, "SIM-") || !s.Simulado() {
		t.Error("el proveedor simulado debe devolver un id SIM- y declararse simulado")
	}
	if _, err := s.Enviar(context.Background(), Mensaje{Canal: "SMS"}); err == nil {
		t.Error("sin destinatario debe fallar")
	}
}

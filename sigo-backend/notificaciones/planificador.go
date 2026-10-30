// Package notificaciones: recordatorios de citas (RF-11). La agenda sólo conoce la interfaz
// Mensajeria; el proveedor concreto se elige por configuración (aquí sólo existe el SIMULADO).
package notificaciones

import (
	"fmt"
	"time"
)

const (
	Confirmacion    = "CONFIRMACION"
	Recordatorio48h = "RECORDATORIO_48H"
	Recordatorio24h = "RECORDATORIO_24H"
)

// Planificada: un mensaje a insertar en `notificaciones`.
type Planificada struct {
	Tipo, Canal, Estado string
	Destinatario        *string
	ProgramadaPara      time.Time
	Motivo              *string
}

// Contacto mínimo del paciente.
type Contacto struct {
	Canal           string
	Telefono, Email *string
	Autoriza        bool
}

// Planificar devuelve confirmación + recordatorios −48 h y −24 h (supuesto S-08):
//   - sin autorización de contacto o sin teléfono/correo ⇒ OMITIDO con el motivo;
//   - si el momento del recordatorio ya pasó (cita con poca anticipación) ⇒ OMITIDO.
//
// Es una función pura: se prueba sin base de datos.
func Planificar(inicio time.Time, c Contacto, ahora time.Time) []Planificada {
	dest := c.Telefono
	if c.Canal == "EMAIL" {
		dest = c.Email
	}
	tipos := []struct {
		tipo  string
		horas int
	}{{Confirmacion, 0}, {Recordatorio48h, 48}, {Recordatorio24h, 24}}
	out := make([]Planificada, 0, 3)
	for _, t := range tipos {
		cuando := ahora
		if t.horas > 0 {
			cuando = inicio.Add(-time.Duration(t.horas) * time.Hour)
		}
		p := Planificada{Tipo: t.tipo, Canal: c.Canal, Destinatario: dest, ProgramadaPara: cuando, Estado: "PENDIENTE"}
		motivo := ""
		switch {
		case !c.Autoriza:
			motivo = "El paciente no autorizó recibir mensajes."
		case dest == nil || *dest == "":
			if c.Canal == "EMAIL" {
				motivo = "El paciente no tiene correo registrado."
			} else {
				motivo = "El paciente no tiene teléfono registrado."
			}
		case t.horas > 0 && !cuando.After(ahora):
			motivo = fmt.Sprintf("La cita se agendó con menos de %d h de anticipación.", t.horas)
		}
		if motivo != "" {
			p.Estado = "OMITIDO"
			p.Motivo = &motivo
		}
		out = append(out, p)
	}
	return out
}

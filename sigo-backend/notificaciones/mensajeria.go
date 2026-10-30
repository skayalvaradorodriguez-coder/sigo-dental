package notificaciones

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"sigo-backend/seguridad"
)

// Mensaje a entregar por un proveedor.
type Mensaje struct {
	Canal, Destinatario, Texto string
}

// Mensajeria es el puerto (interfaz). Un proveedor real (Meta Cloud API, Twilio, SMTP)
// implementaría esta misma interfaz con sus credenciales.
type Mensajeria interface {
	Nombre() string
	Simulado() bool // true = no entrega mensajes reales; la interfaz lo rotula "SIMULADO"
	Enviar(ctx context.Context, m Mensaje) (idProveedor string, err error)
}

// Simulada: NO se conecta a ningún servicio ni envía nada. Deja el mensaje en el log
// (igual que el LogMailer de MultiCatálogo) y devuelve un id con prefijo SIM-.
type Simulada struct{ Enviados []Mensaje }

func (s *Simulada) Nombre() string { return "simulado" }
func (s *Simulada) Simulado() bool { return true }
func (s *Simulada) Enviar(_ context.Context, m Mensaje) (string, error) {
	if strings.TrimSpace(m.Destinatario) == "" {
		return "", errors.New("destinatario vacío")
	}
	s.Enviados = append(s.Enviados, m)
	log.Printf("[mensajeria SIMULADA] %s → %s: %s", m.Canal, m.Destinatario, m.Texto)
	return "SIM-" + seguridad.TokenAleatorio(9), nil
}

// DatosPlantilla: sólo fecha, hora, lugar y profesional; nunca datos clínicos.
type DatosPlantilla struct {
	Nombre, Clinica, Sede, Odontologo string
	Inicio                            time.Time
	Zona                              *time.Location
}

var (
	diasSemana = []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}
	meses      = []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
)

func Plantilla(tipo string, d DatosPlantilla) string {
	t := d.Inicio.In(d.Zona)
	dia := fmt.Sprintf("%s %d de %s", diasSemana[t.Weekday()], t.Day(), meses[t.Month()-1])
	hora := t.Format("15:04")
	lugar := d.Clinica + " – " + d.Sede
	switch tipo {
	case Recordatorio48h:
		return fmt.Sprintf("Hola %s, le recordamos su cita en %s el %s a las %s (en 2 días) con %s.", d.Nombre, lugar, dia, hora, d.Odontologo)
	case Recordatorio24h:
		return fmt.Sprintf("Hola %s, su cita en %s es mañana %s a las %s con %s. Le esperamos.", d.Nombre, lugar, dia, hora, d.Odontologo)
	default:
		return fmt.Sprintf("Hola %s, su cita en %s quedó agendada para el %s a las %s con %s. Si necesita cambiarla, comuníquese con la clínica.", d.Nombre, lugar, dia, hora, d.Odontologo)
	}
}

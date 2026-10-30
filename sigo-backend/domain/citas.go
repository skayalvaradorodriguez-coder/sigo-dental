package domain

import "slices"

// Estados de cita (RF-10).
//
//	PROGRAMADO ─► CONFIRMADO ─► EN_SALA_ESPERA ─► EN_ATENCION ─► ATENDIDO
//	    └──────────────┴───────────────┴──► CANCELADO
//	    └──────────────┴──────────────────► NO_ASISTIO (sólo desde la hora de la cita)
//
// ATENDIDO, CANCELADO y NO_ASISTIO son finales.
const (
	Programado   = "PROGRAMADO"
	Confirmado   = "CONFIRMADO"
	EnSalaEspera = "EN_SALA_ESPERA"
	EnAtencion   = "EN_ATENCION"
	Atendido     = "ATENDIDO"
	Cancelado    = "CANCELADO"
	NoAsistio    = "NO_ASISTIO"
)

var EstadosCita = []string{Programado, Confirmado, EnSalaEspera, EnAtencion, Atendido, Cancelado, NoAsistio}

var Transiciones = map[string][]string{
	Programado:   {Confirmado, EnSalaEspera, Cancelado, NoAsistio},
	Confirmado:   {EnSalaEspera, Cancelado, NoAsistio},
	EnSalaEspera: {EnAtencion, Cancelado},
	EnAtencion:   {Atendido},
	Atendido:     {},
	Cancelado:    {},
	NoAsistio:    {},
}

var EtiquetaEstado = map[string]string{
	Programado: "Programado", Confirmado: "Confirmado", EnSalaEspera: "En sala de espera",
	EnAtencion: "En atención", Atendido: "Atendido", Cancelado: "Cancelado", NoAsistio: "No asistió",
}

func TransicionValida(actual, nuevo string) bool { return slices.Contains(Transiciones[actual], nuevo) }

// OcupaSillon: Cancelado y No asistió liberan el sillón (supuesto S-14; igual que la restricción EXCLUDE).
func OcupaSillon(estado string) bool { return estado != Cancelado && estado != NoAsistio }

// Reprogramable: sólo una cita aún no iniciada cambia de horario, sillón u odontólogo.
func Reprogramable(estado string) bool { return estado == Programado || estado == Confirmado }

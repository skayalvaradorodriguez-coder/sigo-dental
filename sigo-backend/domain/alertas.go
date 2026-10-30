// Package domain contiene reglas de negocio PURAS (sin BD ni HTTP), probadas con go test.
package domain

import (
	"sort"
	"strings"
)

type Alergia struct {
	Sustancia string `json:"sustancia"`
	Tipo      string `json:"tipo"` // MEDICAMENTO | MATERIAL | ALIMENTO | OTRO
	Reaccion  string `json:"reaccion,omitempty"`
}

var TiposAlergia = []string{"MEDICAMENTO", "MATERIAL", "ALIMENTO", "OTRO"}

// DatosAlerta: la parte de la anamnesis que genera alertas.
type DatosAlerta struct {
	Alergias               []Alergia
	Anticoagulado          bool
	Coagulopatia           bool
	OtraCondicionCritica   string
	Embarazo               *bool
	EnfermedadesSistemicas []string
}

type Alerta struct {
	Codigo    string `json:"codigo"`
	Etiqueta  string `json:"etiqueta"`
	Severidad string `json:"severidad"` // CRITICA | ADVERTENCIA
}

// CalcularAlertas (RF-02). Criterio (supuesto S-16):
//   - CRÍTICA: alergia a medicamento o material dental, anticoagulado, coagulopatía u otra condición crítica.
//   - ADVERTENCIA: enfermedades sistémicas, embarazo, alergias alimentarias u otras.
//
// Se calculan siempre desde la anamnesis vigente (no se guardan): nunca quedan desactualizadas.
func CalcularAlertas(d *DatosAlerta) []Alerta {
	if d == nil {
		return []Alerta{}
	}
	out := []Alerta{}
	for _, a := range d.Alergias {
		sev := "ADVERTENCIA"
		if a.Tipo == "MEDICAMENTO" || a.Tipo == "MATERIAL" {
			sev = "CRITICA"
		}
		et := "Alergia a " + a.Sustancia
		if a.Reaccion != "" {
			et += " (" + a.Reaccion + ")"
		}
		out = append(out, Alerta{"ALERGIA_" + a.Tipo, et, sev})
	}
	if d.Anticoagulado {
		out = append(out, Alerta{"ANTICOAGULADO", "Paciente anticoagulado", "CRITICA"})
	}
	if d.Coagulopatia {
		out = append(out, Alerta{"COAGULOPATIA", "Coagulopatía", "CRITICA"})
	}
	if s := strings.TrimSpace(d.OtraCondicionCritica); s != "" {
		out = append(out, Alerta{"OTRA_CRITICA", s, "CRITICA"})
	}
	if d.Embarazo != nil && *d.Embarazo {
		out = append(out, Alerta{"EMBARAZO", "Embarazo", "ADVERTENCIA"})
	}
	for _, e := range d.EnfermedadesSistemicas {
		out = append(out, Alerta{"SISTEMICA", e, "ADVERTENCIA"})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Severidad == "CRITICA" && out[j].Severidad != "CRITICA" })
	return out
}

func SoloCriticas(as []Alerta) []string {
	out := []string{}
	for _, a := range as {
		if a.Severidad == "CRITICA" {
			out = append(out, a.Etiqueta)
		}
	}
	return out
}

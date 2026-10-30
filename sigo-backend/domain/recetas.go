package domain

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// ---------------------------------------------------------------- Recetas (propuesta P-01)
//
// Aviso de seguridad al prescribir: compara cada medicamento con las alergias a medicamentos y
// con la condición de anticoagulación de la anamnesis vigente. Es una AYUDA con una lista corta
// de grupos frecuentes en odontología; no reemplaza el criterio clínico ni una base de
// interacciones profesional.

// Grupos de reactividad cruzada frecuentes en la práctica odontológica.
var gruposMedicamentos = map[string][]string{
	"penicilinas (betalactámicos)":   {"penicilina", "amoxicilina", "ampicilina", "dicloxacilina", "oxacilina", "piperacilina", "bencilpenicilina", "fenoximetilpenicilina"},
	"cefalosporinas":                 {"cefalexina", "cefadroxilo", "cefuroxima", "ceftriaxona", "cefazolina"},
	"AINE":                           {"ibuprofeno", "naproxeno", "diclofenaco", "ketorolaco", "aspirina", "acido acetilsalicilico", "ketoprofeno", "meloxicam", "piroxicam", "nimesulida", "celecoxib"},
	"macrólidos":                     {"eritromicina", "azitromicina", "claritromicina"},
	"sulfas":                         {"sulfametoxazol", "cotrimoxazol", "trimetoprim-sulfametoxazol"},
	"anestésicos locales tipo amida": {"lidocaina", "mepivacaina", "articaina", "bupivacaina", "prilocaina"},
}

// Medicamentos que aumentan el riesgo de sangrado en un paciente anticoagulado.
var riesgoSangrado = []string{"ibuprofeno", "naproxeno", "diclofenaco", "ketorolaco", "aspirina", "acido acetilsalicilico", "ketoprofeno", "meloxicam", "piroxicam"}

// normalizar: minúsculas y sin tildes, para comparar "Amoxicilina" con "amoxicilína".
func normalizar(s string) string {
	t := norm.NFD.String(strings.ToLower(strings.TrimSpace(s)))
	var b strings.Builder
	for _, r := range t {
		if r >= 0x300 && r <= 0x36f { // marcas diacríticas
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func contiene(a, b string) bool {
	return a != "" && b != "" && (strings.Contains(a, b) || strings.Contains(b, a))
}

func grupoDe(medicamento string) string {
	m := normalizar(medicamento)
	for g, lista := range gruposMedicamentos {
		for _, x := range lista {
			if contiene(m, x) {
				return g
			}
		}
	}
	return ""
}

type AlertaReceta struct {
	Medicamento string `json:"medicamento"`
	Motivo      string `json:"motivo"`
}

// AlertasReceta revisa los medicamentos contra la anamnesis vigente (nil = sin anamnesis).
func AlertasReceta(medicamentos []string, d *DatosAlerta) []AlertaReceta {
	out := []AlertaReceta{}
	if d == nil {
		return out
	}
	for _, med := range medicamentos {
		nm := normalizar(med)
		g := grupoDe(med)
		for _, a := range d.Alergias {
			if a.Tipo != "MEDICAMENTO" {
				continue
			}
			na := normalizar(a.Sustancia)
			switch {
			case contiene(nm, na):
				out = append(out, AlertaReceta{med, "Alergia registrada a " + a.Sustancia + "."})
			case g != "" && (grupoDe(a.Sustancia) == g || contiene(normalizar(g), na)):
				out = append(out, AlertaReceta{med, "Mismo grupo que la alergia registrada a " + a.Sustancia + " (" + g + "): posible reacción cruzada."})
			}
		}
		if d.Anticoagulado || d.Coagulopatia {
			for _, x := range riesgoSangrado {
				if contiene(nm, x) {
					out = append(out, AlertaReceta{med, "Paciente anticoagulado o con coagulopatía: aumenta el riesgo de sangrado."})
					break
				}
			}
		}
	}
	return out
}

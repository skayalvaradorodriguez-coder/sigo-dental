package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------- Numeración dental (RF-03)
//
// Se guarda SIEMPRE la notación FDI (dos dígitos: cuadrante + diente). Universal es sólo una
// forma de mostrar: permanentes 1–32 y temporales A–T.
//
//	FDI permanente: cuadrantes 1–4, dientes 1–8   (18…11 | 21…28 / 48…41 | 31…38)
//	FDI temporal:   cuadrantes 5–8, dientes 1–5   (55…51 | 61…65 / 85…81 | 71…75)

// EsPermanente indica si el código FDI corresponde a un diente permanente.
func EsPermanente(fdi int) bool { q, d := fdi/10, fdi%10; return q >= 1 && q <= 4 && d >= 1 && d <= 8 }

// EsTemporal indica si el código FDI corresponde a un diente temporal (de leche).
func EsTemporal(fdi int) bool { q, d := fdi/10, fdi%10; return q >= 5 && q <= 8 && d >= 1 && d <= 5 }

// DienteValido: permanente o temporal en notación FDI.
func DienteValido(fdi int) bool { return EsPermanente(fdi) || EsTemporal(fdi) }

// EsSuperior: arcada superior (cuadrantes 1, 2, 5, 6).
func EsSuperior(fdi int) bool { q := fdi / 10; return q == 1 || q == 2 || q == 5 || q == 6 }

// EsAnterior: incisivos y caninos (dígito 1–3), donde la cara oclusal se llama incisal.
func EsAnterior(fdi int) bool { return fdi%10 >= 1 && fdi%10 <= 3 }

// DientesPermanentes y DientesTemporales en el orden del odontograma (de derecha a izquierda del
// paciente, arcada superior y luego inferior).
var DientesPermanentes = []int{18, 17, 16, 15, 14, 13, 12, 11, 21, 22, 23, 24, 25, 26, 27, 28,
	48, 47, 46, 45, 44, 43, 42, 41, 31, 32, 33, 34, 35, 36, 37, 38}
var DientesTemporales = []int{55, 54, 53, 52, 51, 61, 62, 63, 64, 65, 85, 84, 83, 82, 81, 71, 72, 73, 74, 75}

// FDIaUniversal convierte un código FDI a Universal ("1"…"32" o "A"…"T").
func FDIaUniversal(fdi int) (string, error) {
	q, d := fdi/10, fdi%10
	switch {
	case EsPermanente(fdi):
		var n int
		switch q {
		case 1:
			n = 9 - d // 18→1 … 11→8
		case 2:
			n = 8 + d // 21→9 … 28→16
		case 3:
			n = 25 - d // 38→17 … 31→24
		case 4:
			n = 24 + d // 41→25 … 48→32
		}
		return strconv.Itoa(n), nil
	case EsTemporal(fdi):
		var i int
		switch q {
		case 5:
			i = 5 - d // 55→A … 51→E
		case 6:
			i = 4 + d // 61→F … 65→J
		case 7:
			i = 15 - d // 75→K … 71→O
		case 8:
			i = 14 + d // 81→P … 85→T
		}
		return string(rune('A' + i)), nil
	}
	return "", fmt.Errorf("diente FDI inválido: %d", fdi)
}

// UniversalAFDI convierte "1"…"32" o "A"…"T" a FDI.
func UniversalAFDI(u string) (int, error) {
	u = strings.ToUpper(strings.TrimSpace(u))
	if n, err := strconv.Atoi(u); err == nil {
		switch {
		case n >= 1 && n <= 8:
			return 10 + (9 - n), nil
		case n >= 9 && n <= 16:
			return 20 + (n - 8), nil
		case n >= 17 && n <= 24:
			return 30 + (25 - n), nil
		case n >= 25 && n <= 32:
			return 40 + (n - 24), nil
		}
	} else if len(u) == 1 && u[0] >= 'A' && u[0] <= 'T' {
		i := int(u[0] - 'A')
		switch {
		case i <= 4:
			return 50 + (5 - i), nil
		case i <= 9:
			return 60 + (i - 4), nil
		case i <= 14:
			return 70 + (15 - i), nil
		default:
			return 80 + (i - 14), nil
		}
	}
	return 0, fmt.Errorf("diente Universal inválido: %q", u)
}

// ---------------------------------------------------------------- Superficies
//
// Se guardan 5 códigos canónicos: M (mesial), D (distal), O (oclusal / incisal), V (vestibular)
// y L (lingual / palatino). Al mostrarlas, O es "I" en anteriores y L es "P" en la arcada superior.
var Superficies = []string{"M", "D", "O", "V", "L"}

// NormalizarSuperficie acepta también I (incisal) y P (palatino) y devuelve el código canónico.
func NormalizarSuperficie(s string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "M":
		return "M", true
	case "D":
		return "D", true
	case "O", "I":
		return "O", true
	case "V":
		return "V", true
	case "L", "P":
		return "L", true
	}
	return "", false
}

// EtiquetaSuperficie devuelve cómo se rotula la superficie en ese diente (O/I, L/P).
func EtiquetaSuperficie(fdi int, s string) string {
	switch {
	case s == "O" && EsAnterior(fdi):
		return "I"
	case s == "L" && EsSuperior(fdi):
		return "P"
	}
	return s
}

// ---------------------------------------------------------------- Catálogo de hallazgos
//
// Convención de color (propuesta P-04 del análisis): ROJO = hallazgo o tratamiento pendiente;
// AZUL = existente o realizado. El SRS sólo fija caries en rojo y restauración en azul.

type Hallazgo struct {
	Codigo     string `json:"codigo"`
	Etiqueta   string `json:"etiqueta"`
	Nivel      string `json:"nivel"` // SUPERFICIE | DIENTE
	Color      string `json:"color"` // ROJO | AZUL
	Superficie string `json:"superficie,omitempty"`
	SoloPerm   bool   `json:"soloPermanentes,omitempty"`
}

var Hallazgos = []Hallazgo{
	{"CARIES", "Caries", "SUPERFICIE", "ROJO", "", false},
	{"RESTAURACION", "Restauración", "SUPERFICIE", "AZUL", "", false},
	{"SELLANTE", "Sellante", "SUPERFICIE", "AZUL", "O", false},
	{"ENDODONCIA_REQUERIDA", "Endodoncia requerida", "DIENTE", "ROJO", "", false},
	{"ENDODONCIA_REALIZADA", "Endodoncia realizada", "DIENTE", "AZUL", "", false},
	{"EXODONCIA_INDICADA", "Exodoncia indicada", "DIENTE", "ROJO", "", false},
	{"AUSENTE", "Ausente / extraído", "DIENTE", "AZUL", "", false},
	{"IMPLANTE", "Implante", "DIENTE", "AZUL", "", true},
	{"PROTESIS_FIJA", "Prótesis fija (corona / puente)", "DIENTE", "AZUL", "", false},
	{"PROTESIS_REMOVIBLE", "Prótesis removible", "DIENTE", "AZUL", "", false},
}

func HallazgoPor(codigo string) (Hallazgo, bool) {
	for _, h := range Hallazgos {
		if h.Codigo == codigo {
			return h, true
		}
	}
	return Hallazgo{}, false
}

// ValidarMarca comprueba que el hallazgo corresponda al diente y a la superficie indicados.
// Devuelve la superficie canónica ("" para hallazgos de todo el diente) o un mensaje de error.
func ValidarMarca(fdi int, superficie, codigo string) (string, string) {
	if !DienteValido(fdi) {
		return "", "Diente inválido: use notación FDI (11–48 permanentes, 51–85 temporales)."
	}
	h, ok := HallazgoPor(codigo)
	if !ok {
		return "", "Hallazgo desconocido."
	}
	if h.SoloPerm && !EsPermanente(fdi) {
		return "", h.Etiqueta + ": sólo en dientes permanentes."
	}
	if h.Nivel == "DIENTE" {
		if strings.TrimSpace(superficie) != "" {
			return "", h.Etiqueta + " se registra en todo el diente, sin superficie."
		}
		return "", ""
	}
	s, ok := NormalizarSuperficie(superficie)
	if !ok {
		return "", h.Etiqueta + " requiere una superficie (M, D, O/I, V, L/P)."
	}
	if h.Superficie != "" && s != h.Superficie {
		return "", h.Etiqueta + " sólo se registra en la cara oclusal."
	}
	return s, ""
}

// EventoOdontograma: una marca o su retiro.
type EventoOdontograma struct {
	Diente     int
	Superficie string
	Hallazgo   string
	Accion     string // AGREGAR | QUITAR
}

type claveMarca struct {
	diente           int
	superficie, hall string
}

// EstadoActual pliega los eventos (en orden cronológico) y devuelve las marcas vigentes,
// en el mismo orden en que se agregaron por primera vez.
func EstadoActual(eventos []EventoOdontograma) []EventoOdontograma {
	vigente := map[claveMarca]bool{}
	orden := []claveMarca{}
	for _, e := range eventos {
		k := claveMarca{e.Diente, e.Superficie, e.Hallazgo}
		if _, visto := vigente[k]; !visto {
			orden = append(orden, k)
		}
		vigente[k] = e.Accion == "AGREGAR"
	}
	out := []EventoOdontograma{}
	for _, k := range orden {
		if vigente[k] {
			out = append(out, EventoOdontograma{Diente: k.diente, Superficie: k.superficie, Hallazgo: k.hall, Accion: "AGREGAR"})
		}
	}
	return out
}

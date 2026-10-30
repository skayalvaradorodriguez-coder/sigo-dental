package domain

// ---------------------------------------------------------------- Periodontograma (RF-04)
//
// Seis sitios por diente: mesio-vestibular, vestibular, disto-vestibular y sus equivalentes
// linguales/palatinos. Profundidad de sondaje (PS) en mm, 0–15. Margen gingival (MG) medido
// desde la línea amelocementaria (LAC): POSITIVO = margen apical a la LAC (recesión),
// NEGATIVO = margen coronal (agrandamiento). Con esa convención: NIC = PS + MG.

var SitiosPerio = []string{"MV", "V", "DV", "ML", "L", "DL"}

func SitioValido(s string) bool {
	for _, x := range SitiosPerio {
		if x == s {
			return true
		}
	}
	return false
}

// NIC: nivel de inserción clínica. Ejemplos: PS 4 + recesión 2 (MG +2) = 6; PS 5 con margen
// 2 mm coronal (MG −2) = 3. En la base es una columna generada con la misma fórmula.
func NIC(profundidad, margen int) int { return profundidad + margen }

type SitioPerio struct {
	Profundidad, Margen int
	Sangrado            bool
}

// ResumenPerio: indicadores que se muestran junto a cada examen.
type ResumenPerio struct {
	Sitios             int     `json:"sitios"`
	PorcentajeSangrado float64 `json:"porcentajeSangrado"`
	BolsasMayor4       int     `json:"bolsasDe4oMas"` // PS ≥ 4 mm
	NICMaximo          int     `json:"nicMaximo"`
}

func Resumir(sitios []SitioPerio) ResumenPerio {
	r := ResumenPerio{Sitios: len(sitios)}
	sangran := 0
	for i, s := range sitios {
		if s.Sangrado {
			sangran++
		}
		if s.Profundidad >= 4 {
			r.BolsasMayor4++
		}
		if n := NIC(s.Profundidad, s.Margen); i == 0 || n > r.NICMaximo {
			r.NICMaximo = n
		}
	}
	if len(sitios) > 0 {
		r.PorcentajeSangrado = float64(int(float64(sangran)*1000/float64(len(sitios))+0.5)) / 10
	}
	return r
}

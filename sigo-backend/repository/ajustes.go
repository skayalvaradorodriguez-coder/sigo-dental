package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

// Definición de un parámetro configurable con su validación.
type DefAjuste struct {
	Clave    string `json:"clave"`
	Etiqueta string `json:"etiqueta"`
	Ayuda    string `json:"ayuda"`
	Min      int    `json:"min"`
	Max      int    `json:"max"`
	Defecto  int    `json:"defecto"`
}

const (
	AjusteInactividad   = "seguridad.minutosInactividad"
	AjusteBloqueoNotas  = "clinica.horasBloqueoNotas"
	AjusteMaxIntentos   = "seguridad.maxIntentosFallidos"
	AjusteMinBloqueo    = "seguridad.minutosBloqueo"
	AjusteTopeDescuento = "presupuestos.topeDescuento"
)

var DefsAjustes = []DefAjuste{
	{AjusteInactividad, "Cierre de sesión por inactividad (minutos)", "RNF-05 fija 15 minutos; puede reducirse, no ampliarse.", 5, 15, 15},
	{AjusteBloqueoNotas, "Bloqueo de notas SOEP (horas)", "RF-05: plazo tras el cual una nota no puede editarse (por defecto 24).", 1, 72, 24},
	{AjusteMaxIntentos, "Intentos fallidos antes de bloquear la cuenta", "Supuesto S-10 (el SRS no lo especifica).", 3, 10, 5},
	{AjusteMinBloqueo, "Duración del bloqueo de cuenta (minutos)", "Supuesto S-10.", 5, 120, 15},
	{AjusteTopeDescuento, "Tope de descuento sin autorización (% del honorario)", "RF-07 / S-25: por encima de este porcentaje, el descuento debe autorizarlo otra persona con su código MFA.", 0, 100, 10},
}

func DefAjustePor(clave string) (DefAjuste, bool) {
	for _, d := range DefsAjustes {
		if d.Clave == clave {
			return d, true
		}
	}
	return DefAjuste{}, false
}

// LeerAjuste devuelve el valor guardado o el valor por defecto si no existe o está fuera de rango.
func (r *Postgres) LeerAjuste(ctx context.Context, clave string) (int, error) {
	def, ok := DefAjustePor(clave)
	if !ok {
		return 0, ErrNoEncontrado
	}
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT valor FROM ajustes WHERE clave=$1`, clave).Scan(&raw)
	if err == pgx.ErrNoRows {
		return def.Defecto, nil
	}
	if err != nil {
		return 0, err
	}
	var v int
	if json.Unmarshal(raw, &v) != nil || v < def.Min || v > def.Max {
		return def.Defecto, nil
	}
	return v, nil
}

func (r *Postgres) GuardarAjuste(ctx context.Context, clave string, valor int, usuarioID string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO ajustes (clave, valor, actualizado_por, actualizado_en) VALUES ($1, to_jsonb($2::int), $3, now())
		ON CONFLICT (clave) DO UPDATE SET valor = EXCLUDED.valor, actualizado_por = EXCLUDED.actualizado_por, actualizado_en = now()`,
		clave, valor, usuarioID)
	return err
}

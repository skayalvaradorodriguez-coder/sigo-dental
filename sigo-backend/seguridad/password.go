// Package seguridad: contraseñas (bcrypt), JWT, TOTP y tokens aleatorios.
package seguridad

import (
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// Costo 12: ~250 ms por verificación; frena ataques de fuerza bruta sin molestar al usuario.
const CostoBcrypt = 12

func HashPassword(plano string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plano), CostoBcrypt)
	return string(h), err
}

func VerificarPassword(hash, plano string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plano)) == nil
}

// hashFicticio iguala el tiempo de respuesta cuando el usuario no existe (sin enumeración).
var hashFicticio, _ = bcrypt.GenerateFromPassword([]byte("contraseña-inexistente-para-tiempo-constante"), CostoBcrypt)

func CompararFicticio(plano string) { _ = bcrypt.CompareHashAndPassword(hashFicticio, []byte(plano)) }

// ErroresPolitica aplica la política de contraseñas (supuesto S-09):
// mínimo 10 caracteres, al menos una letra y un número, sin contener el usuario del correo.
// bcrypt sólo usa los primeros 72 bytes, por eso el máximo es 72.
func ErroresPolitica(clave, email string) []string {
	var errs []string
	if len([]rune(clave)) < 10 {
		errs = append(errs, "Debe tener al menos 10 caracteres.")
	}
	if len(clave) > 72 {
		errs = append(errs, "No puede superar 72 caracteres.")
	}
	var letra, numero bool
	for _, r := range clave {
		if unicode.IsLetter(r) {
			letra = true
		}
		if unicode.IsDigit(r) {
			numero = true
		}
	}
	if !letra {
		errs = append(errs, "Debe incluir al menos una letra.")
	}
	if !numero {
		errs = append(errs, "Debe incluir al menos un número.")
	}
	if local, _, ok := strings.Cut(strings.ToLower(email), "@"); ok && len(local) >= 3 && strings.Contains(strings.ToLower(clave), local) {
		errs = append(errs, "No debe contener su usuario de correo.")
	}
	return errs
}

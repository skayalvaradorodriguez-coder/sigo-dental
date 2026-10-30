package seguridad

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims del token. El estado de la sesión (MFA verificado, inactividad, revocación)
// NO va en el token: se consulta en la tabla `sesiones` por el `jti`. Así un token se
// puede revocar al instante (logout, desactivar usuario) aunque no haya vencido.
type Claims struct {
	Email string `json:"email"`
	Rol   string `json:"rol"`
	jwt.RegisteredClaims
}

var ErrTokenInvalido = errors.New("token inválido")

func GenerarToken(secreto, usuarioID, email, rol, jti string, expira time.Time) (string, error) {
	c := Claims{
		Email: email,
		Rol:   rol,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   usuarioID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expira),
			Issuer:    "sigo-dental",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secreto))
}

func ValidarToken(secreto, token string) (*Claims, error) {
	c := &Claims{}
	t, err := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (any, error) {
		return []byte(secreto), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("sigo-dental"))
	if err != nil || !t.Valid || c.ID == "" || c.Subject == "" {
		return nil, ErrTokenInvalido
	}
	return c, nil
}

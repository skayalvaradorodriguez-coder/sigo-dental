package seguridad

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math/big"
)

// TokenAleatorio devuelve n bytes aleatorios en base64url.
func TokenAleatorio(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func SHA256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// PasswordTemporal genera una clave legible para restablecimientos (p. ej. "KxmPqRtz-482").
func PasswordTemporal() string {
	const letras = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz"
	out := make([]byte, 0, 12)
	for i := 0; i < 8; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letras))))
		out = append(out, letras[n.Int64()])
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(900))
	return string(out) + "-" + big.NewInt(n.Int64()+100).String()
}

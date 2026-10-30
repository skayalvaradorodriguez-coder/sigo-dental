package seguridad

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP según RFC 6238 (HMAC-SHA1, 6 dígitos, 30 s): compatible con Google/Microsoft Authenticator.
// Implementado con la biblioteca estándar para que el mecanismo sea transparente; las
// pruebas usan los vectores oficiales del RFC.
const (
	PeriodoTOTP = 30
	DigitosTOTP = 6
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func GenerarSecretoTOTP() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return b32.EncodeToString(b)
}

func DecodificarSecreto(s string) ([]byte, error) {
	return b32.DecodeString(strings.ToUpper(strings.ReplaceAll(strings.TrimRight(s, "="), " ", "")))
}

// HOTP (RFC 4226) para un contador dado.
func HOTP(clave []byte, contador uint64, digitos int) string {
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, contador)
	m := hmac.New(sha1.New, clave)
	m.Write(msg)
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < digitos; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digitos, v%mod)
}

func Paso(t time.Time) int64 { return t.Unix() / PeriodoTOTP }

func CodigoTOTP(secreto string, t time.Time) string {
	k, err := DecodificarSecreto(secreto)
	if err != nil {
		return ""
	}
	return HOTP(k, uint64(Paso(t)), DigitosTOTP)
}

// VerificarTOTP acepta ±1 intervalo de desfase. Devuelve el paso aceptado para impedir
// reutilizar el mismo código (anti-replay); ok=false si no coincide.
func VerificarTOTP(secreto, codigo string, t time.Time, ultimoPaso *int64) (int64, bool) {
	if len(codigo) != DigitosTOTP {
		return 0, false
	}
	for _, r := range codigo {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	k, err := DecodificarSecreto(secreto)
	if err != nil {
		return 0, false
	}
	actual := Paso(t)
	for d := int64(-1); d <= 1; d++ {
		p := actual + d
		if ultimoPaso != nil && p <= *ultimoPaso {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(HOTP(k, uint64(p), DigitosTOTP)), []byte(codigo)) == 1 {
			return p, true
		}
	}
	return 0, false
}

func OtpauthURL(secreto, cuenta, emisor string) string {
	v := url.Values{}
	v.Set("secret", secreto)
	v.Set("issuer", emisor)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")
	return "otpauth://totp/" + url.PathEscape(emisor+":"+cuenta) + "?" + v.Encode()
}

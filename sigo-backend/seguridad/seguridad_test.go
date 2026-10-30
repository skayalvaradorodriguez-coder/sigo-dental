package seguridad

import (
	"strings"
	"testing"
	"time"
)

// Vectores oficiales del RFC 6238, Apéndice B (SHA1, 8 dígitos).
func TestTOTPVectoresRFC6238(t *testing.T) {
	clave := []byte("12345678901234567890")
	casos := []struct {
		seg  int64
		want string
	}{{59, "94287082"}, {1111111109, "07081804"}, {1111111111, "14050471"}, {1234567890, "89005924"}, {2000000000, "69279037"}, {20000000000, "65353130"}}
	for _, c := range casos {
		if got := HOTP(clave, uint64(c.seg/30), 8); got != c.want {
			t.Errorf("t=%d: got %s want %s", c.seg, got, c.want)
		}
	}
}

func TestTOTPVentanaYAntiReplay(t *testing.T) {
	s := GenerarSecretoTOTP()
	ahora := time.Unix(1_700_000_000, 0)
	if _, ok := VerificarTOTP(s, CodigoTOTP(s, ahora.Add(-30*time.Second)), ahora, nil); !ok {
		t.Error("debe aceptar el intervalo anterior")
	}
	if _, ok := VerificarTOTP(s, CodigoTOTP(s, ahora.Add(-90*time.Second)), ahora, nil); ok {
		t.Error("no debe aceptar fuera de la ventana")
	}
	paso, ok := VerificarTOTP(s, CodigoTOTP(s, ahora), ahora, nil)
	if !ok {
		t.Fatal("código actual rechazado")
	}
	if _, ok := VerificarTOTP(s, CodigoTOTP(s, ahora), ahora, &paso); ok {
		t.Error("el mismo código no debe aceptarse dos veces")
	}
	for _, malo := range []string{"12345", "abcdef", "1234567"} {
		if _, ok := VerificarTOTP(s, malo, ahora, nil); ok {
			t.Errorf("formato inválido aceptado: %s", malo)
		}
	}
}

func TestOtpauth(t *testing.T) {
	u := OtpauthURL("ABC", "ana@demo.sigo", "SIGO-Dental")
	if !strings.HasPrefix(u, "otpauth://totp/SIGO-Dental:ana@demo.sigo?") || !strings.Contains(u, "secret=ABC") {
		t.Errorf("url inesperada: %s", u)
	}
}

func TestBcrypt(t *testing.T) {
	h, err := HashPassword("Clave-Segura-123")
	if err != nil || !strings.HasPrefix(h, "$2a$12$") {
		t.Fatalf("hash inesperado %q %v", h, err)
	}
	if !VerificarPassword(h, "Clave-Segura-123") || VerificarPassword(h, "otra-123456") {
		t.Error("verificación incorrecta")
	}
	if VerificarPassword("no-es-hash", "x") {
		t.Error("hash corrupto no debe validar")
	}
}

func TestPoliticaPassword(t *testing.T) {
	if len(ErroresPolitica("Sillon-Azul-42", "ana@demo.sigo")) != 0 {
		t.Error("clave válida rechazada")
	}
	for _, c := range []string{"corta1", "solamenteletras", "1234567890"} {
		if len(ErroresPolitica(c, "")) == 0 {
			t.Errorf("debió rechazar %q", c)
		}
	}
	if len(ErroresPolitica("carla2026xyz", "carla@demo.sigo")) == 0 {
		t.Error("debió rechazar clave con el usuario del correo")
	}
}

func TestJWT(t *testing.T) {
	sec := strings.Repeat("s", 32)
	tok, _ := GenerarToken(sec, "u1", "a@b.c", "ADMIN_TI", "jti-1", time.Now().Add(time.Hour))
	c, err := ValidarToken(sec, tok)
	if err != nil || c.Subject != "u1" || c.ID != "jti-1" || c.Rol != "ADMIN_TI" {
		t.Fatalf("token válido rechazado: %v", err)
	}
	if _, err := ValidarToken(strings.Repeat("x", 32), tok); err == nil {
		t.Error("token firmado con otra clave aceptado")
	}
	if _, err := ValidarToken(sec, tok+"x"); err == nil {
		t.Error("token adulterado aceptado")
	}
	vencido, _ := GenerarToken(sec, "u1", "a@b.c", "ADMIN_TI", "j", time.Now().Add(-time.Minute))
	if _, err := ValidarToken(sec, vencido); err == nil {
		t.Error("token vencido aceptado")
	}
}

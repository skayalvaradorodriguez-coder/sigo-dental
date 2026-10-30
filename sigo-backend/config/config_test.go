package config

import (
	"strings"
	"testing"
)

func TestProduccionExigeTLSYSecretoPropio(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("CLINIC_TIMEZONE", "America/Guayaquil")
	t.Setenv("WEB_ORIGIN", "http://clinica.example.com")
	t.Setenv("JWT_SECRET", "cambie-este-secreto-por-uno-largo-y-aleatorio-de-desarrollo")
	t.Setenv("DATABASE_URL", "postgres://u:p@host/db?sslmode=disable")
	_, err := Cargar("no-existe.env")
	if err == nil {
		t.Fatal("la configuración de producción insegura debe rechazarse")
	}
	for _, m := range []string{"sslmode=require", "JWT_SECRET de ejemplo", "https://"} {
		if !strings.Contains(err.Error(), m) {
			t.Errorf("falta el aviso %q en: %v", m, err)
		}
	}
	t.Setenv("WEB_ORIGIN", "https://clinica.example.com")
	t.Setenv("JWT_SECRET", strings.Repeat("x", 48))
	t.Setenv("DATABASE_URL", "postgres://u:p@host/db?sslmode=require")
	if c, err := Cargar("no-existe.env"); err != nil || !c.EsProduccion() {
		t.Errorf("configuración de producción válida: %v", err)
	}
}

// El modo demo (despliegue público con datos ficticios) tiene las mismas exigencias que producción.
func TestDemoExigeLoMismoQueProduccion(t *testing.T) {
	t.Setenv("APP_ENV", "demo")
	t.Setenv("CLINIC_TIMEZONE", "America/Guayaquil")
	t.Setenv("WEB_ORIGIN", "https://demo.example.com")
	t.Setenv("JWT_SECRET", strings.Repeat("x", 48))
	t.Setenv("DATABASE_URL", "postgres://u:p@host/db?sslmode=disable")
	if _, err := Cargar("no-existe.env"); err == nil || !strings.Contains(err.Error(), "sslmode=require") {
		t.Fatalf("demo sin TLS debe rechazarse: %v", err)
	}
	t.Setenv("DATABASE_URL", "postgres://u:p@host/db?sslmode=require")
	c, err := Cargar("no-existe.env")
	if err != nil || !c.EsProduccion() || !c.EsDemo() {
		t.Fatalf("demo válida: %v", err)
	}
}

func TestRedisURL(t *testing.T) {
	t.Setenv("CLINIC_TIMEZONE", "America/Guayaquil")
	t.Setenv("REDIS_URL", "redis://red-abc123:6379")
	t.Setenv("DATABASE_URL", "postgres://u:p@host/db")
	t.Setenv("JWT_SECRET", strings.Repeat("x", 48))
	c, err := Cargar("no-existe.env")
	if err != nil || c.RedisAddr != "red-abc123:6379" || c.RedisDB != 0 {
		t.Fatalf("REDIS_URL: %v %+v", err, c.RedisAddr)
	}
}

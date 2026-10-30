// Package config carga y valida la configuración desde variables de entorno (.env).
// Si falta algo obligatorio, el backend se niega a arrancar con un mensaje claro.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Puerto              string
	Entorno             string // dev | test | prod | demo (producción con datos ficticios)
	DatabaseURL         string
	JWTSecret           string
	SesionMaxima        time.Duration // duración absoluta máxima de una sesión
	RedisAddr           string
	RedisPass           string
	RedisDB             int
	WebOrigin           string
	MFAEmisor           string
	ZonaHoraria         *time.Location
	NombreZona          string
	NombreClinica       string
	ProveedorMensajeria string
	DespachoCada        time.Duration // 0 = despachador desactivado
	ArchivosDir         string        // carpeta del almacenamiento local de archivos del paciente
	StaticDir           string        // carpeta de la build del frontend (producción)
}

// EsProduccion: postura de seguridad de producción (TLS, HSTS, secretos propios). También en "demo".
func (c Config) EsProduccion() bool { return c.Entorno == "prod" || c.Entorno == "demo" }

// EsDemo: despliegue público de demostración con datos ficticios (semilla y código MFA de prueba).
func (c Config) EsDemo() bool { return c.Entorno == "demo" }

func env(clave, defecto string) string {
	if v := strings.TrimSpace(os.Getenv(clave)); v != "" {
		return v
	}
	return defecto
}

// Cargar lee `.env` (si existe; las variables del sistema tienen prioridad) y valida.
func Cargar(archivos ...string) (Config, error) {
	if len(archivos) == 0 {
		archivos = []string{".env"}
	}
	for _, a := range archivos {
		_ = godotenv.Load(a) // no sobrescribe variables ya definidas
	}
	var errs []string
	c := Config{
		Puerto:              env("PORT", "3000"),
		Entorno:             env("APP_ENV", "dev"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		RedisAddr:           env("REDIS_ADDR", "localhost:6380"),
		RedisPass:           os.Getenv("REDIS_PASSWORD"),
		WebOrigin:           env("WEB_ORIGIN", "http://localhost:5173"),
		MFAEmisor:           env("MFA_ISSUER", "SIGO-Dental"),
		NombreClinica:       env("CLINIC_NAME", "Clínica Dental Demo"),
		ProveedorMensajeria: env("MESSAGING_PROVIDER", "simulated"),
		StaticDir:           os.Getenv("STATIC_DIR"),
		ArchivosDir:         env("ARCHIVOS_DIR", "./archivos"),
	}
	// Base de datos: DATABASE_URL (nube) o DB_* (local), igual que en MultiCatálogo.
	c.DatabaseURL = os.Getenv("DATABASE_URL")
	if c.DatabaseURL == "" {
		c.DatabaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			env("DB_USER", "sigo"), os.Getenv("DB_PASSWORD"), env("DB_HOST", "localhost"),
			env("DB_PORT", "5434"), env("DB_NAME", "sigo_dental"), env("DB_SSLMODE", "disable"))
		if os.Getenv("DB_PASSWORD") == "" {
			errs = append(errs, "DB_PASSWORD es obligatoria (o DATABASE_URL)")
		}
	}
	if len(c.JWTSecret) < 32 {
		errs = append(errs, "JWT_SECRET debe tener al menos 32 caracteres")
	}
	horas, err := strconv.ParseFloat(env("SESSION_ABSOLUTE_HOURS", "12"), 64)
	if err != nil || horas <= 0 || horas > 24 {
		errs = append(errs, "SESSION_ABSOLUTE_HOURS debe estar entre 0 y 24")
	}
	c.SesionMaxima = time.Duration(horas * float64(time.Hour))
	c.NombreZona = env("CLINIC_TIMEZONE", "UTC")
	if loc, err := time.LoadLocation(c.NombreZona); err != nil {
		errs = append(errs, "CLINIC_TIMEZONE inválida (ej. America/Guayaquil)")
	} else {
		c.ZonaHoraria = loc
	}
	seg, err := strconv.Atoi(env("NOTIFICATIONS_DISPATCH_SECONDS", "30"))
	if err != nil || seg < 0 {
		errs = append(errs, "NOTIFICATIONS_DISPATCH_SECONDS debe ser un entero >= 0")
	}
	c.DespachoCada = time.Duration(seg) * time.Second
	if c.RedisDB, err = strconv.Atoi(env("REDIS_DB", "0")); err != nil || c.RedisDB < 0 || c.RedisDB > 15 {
		errs = append(errs, "REDIS_DB debe ser un entero entre 0 y 15")
	}
	// REDIS_URL (redis://[usuario:clave@]host:puerto[/db]), como la entrega Render Key Value, tiene
	// prioridad sobre REDIS_ADDR / REDIS_PASSWORD / REDIS_DB.
	if ru := os.Getenv("REDIS_URL"); ru != "" {
		if u, e := url.Parse(ru); e != nil || u.Scheme != "redis" || u.Host == "" {
			errs = append(errs, "REDIS_URL inválida (se espera redis://host:puerto)")
		} else {
			c.RedisAddr = u.Host
			if p, ok := u.User.Password(); ok {
				c.RedisPass = p
			}
			if db := strings.Trim(u.Path, "/"); db != "" {
				if c.RedisDB, err = strconv.Atoi(db); err != nil || c.RedisDB < 0 || c.RedisDB > 15 {
					errs = append(errs, "REDIS_URL: la base debe ser un entero entre 0 y 15")
				}
			}
		}
	}
	if c.ProveedorMensajeria != "simulated" {
		errs = append(errs, `MESSAGING_PROVIDER: sólo "simulated" está implementado`)
	}
	switch c.Entorno {
	case "dev", "test", "prod", "demo":
	default:
		errs = append(errs, "APP_ENV debe ser dev, test, prod o demo")
	}
	if c.EsProduccion() {
		// RNF-04: en producción la conexión a la base debe ir cifrada (TLS) y los secretos no pueden
		// ser los de ejemplo.
		if !strings.Contains(c.DatabaseURL, "sslmode=require") && !strings.Contains(c.DatabaseURL, "sslmode=verify") {
			errs = append(errs, "en producción la base de datos exige TLS: DATABASE_URL con sslmode=require (o verify-full)")
		}
		if strings.Contains(c.JWTSecret, "cambie-este-secreto") || strings.Contains(c.JWTSecret, "pruebas") {
			errs = append(errs, "JWT_SECRET de ejemplo: genere uno propio para producción")
		}
		if strings.HasPrefix(c.WebOrigin, "http://") && !strings.Contains(c.WebOrigin, "localhost") {
			errs = append(errs, "WEB_ORIGIN debe usar https:// en producción")
		}
	}
	if len(errs) > 0 {
		return c, fmt.Errorf("configuración inválida:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return c, nil
}

// Package validacion centraliza la validación de entrada. Acumula errores por campo
// para responder 400 con todos los problemas a la vez.
package validacion

import (
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type Errores map[string]string

func (e Errores) Ok() bool { return len(e) == 0 }

func (e Errores) Agregar(campo, msg string) {
	if _, ya := e[campo]; !ya {
		e[campo] = msg
	}
}

func (e Errores) Mensaje() string {
	partes := make([]string, 0, len(e))
	for _, m := range e {
		partes = append(partes, m)
	}
	slices.Sort(partes)
	return strings.Join(partes, " ")
}

var (
	reTelefono  = regexp.MustCompile(`^\+?[0-9 ()-]{7,20}$`)
	reDocumento = regexp.MustCompile(`^[A-Za-z0-9-]{5,20}$`)
	reCodigo    = regexp.MustCompile(`^[A-Z0-9-]{2,20}$`)
	reDinero    = regexp.MustCompile(`^\d{1,10}(\.\d{1,2})?$`)
	reUUID      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func (e Errores) Requerido(campo, valor, msg string) {
	if strings.TrimSpace(valor) == "" {
		e.Agregar(campo, msg)
	}
}

func (e Errores) Longitud(campo, valor string, min, max int, msg string) {
	n := utf8.RuneCountInString(strings.TrimSpace(valor))
	if n < min || n > max {
		e.Agregar(campo, msg)
	}
}

func (e Errores) Email(campo, valor string, obligatorio bool) {
	if valor == "" && !obligatorio {
		return
	}
	a, err := mail.ParseAddress(valor)
	if err != nil || a.Address != valor || !strings.Contains(valor, ".") || len(valor) > 200 {
		e.Agregar(campo, "Correo inválido.")
	}
}

func (e Errores) Telefono(campo, valor string, obligatorio bool, msg string) {
	if valor == "" && !obligatorio {
		return
	}
	if !reTelefono.MatchString(valor) {
		e.Agregar(campo, msg)
	}
}

func (e Errores) Documento(campo, valor, msg string) {
	if !reDocumento.MatchString(valor) {
		e.Agregar(campo, msg)
	}
}

func (e Errores) Codigo(campo, valor string) {
	if !reCodigo.MatchString(valor) {
		e.Agregar(campo, "Código: 2 a 20 caracteres en mayúsculas, números o guiones.")
	}
}

func (e Errores) Dinero(campo, valor string) {
	if !reDinero.MatchString(valor) {
		e.Agregar(campo, "Monto: número positivo con hasta 2 decimales.")
	}
}

func (e Errores) EnLista(campo, valor string, lista []string, msg string) {
	if !slices.Contains(lista, valor) {
		e.Agregar(campo, msg)
	}
}

func (e Errores) Rango(campo string, v, min, max int, msg string) {
	if v < min || v > max {
		e.Agregar(campo, msg)
	}
}

// Fecha valida AAAA-MM-DD.
func (e Errores) Fecha(campo, valor, msg string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", valor)
	if err != nil {
		e.Agregar(campo, msg)
		return time.Time{}, false
	}
	return t, true
}

// Instante valida ISO-8601 (RFC 3339).
func (e Errores) Instante(campo, valor, msg string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, valor)
	if err != nil {
		e.Agregar(campo, msg)
		return time.Time{}, false
	}
	return t, true
}

func EsUUID(s string) bool { return reUUID.MatchString(s) }

func (e Errores) UUID(campo, valor string, obligatorio bool) {
	if valor == "" && !obligatorio {
		return
	}
	if !EsUUID(valor) {
		e.Agregar(campo, fmt.Sprintf("%s inválido.", campo))
	}
}

func Limpio(s string) string { return strings.TrimSpace(s) }

// Opcional convierte "" en nil (para columnas NULL).
func Opcional(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

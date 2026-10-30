package repository

import (
	"testing"
	"time"
)

func TestJSONCanonicoIgnoraOrden(t *testing.T) {
	a := map[string]any{"b": 1, "a": map[string]any{"d": 1, "c": 2}}
	b := map[string]any{"a": map[string]any{"c": 2, "d": 1}, "b": 1}
	if JSONCanonico(a) != JSONCanonico(b) {
		t.Error("el orden de las claves no debe cambiar el texto canónico")
	}
}

func TestHashAuditoriaDeterministaYSensible(t *testing.T) {
	ts := time.Date(2026, 10, 29, 15, 0, 0, 0, time.UTC)
	u, e, ip := "u1", "p-1", "10.0.0.5"
	h1 := HashAuditoria(HashGenesis, ts, &u, &e, &ip, nil, "LECTURA", "paciente", map[string]any{"b": 2, "a": 1})
	h2 := HashAuditoria(HashGenesis, ts, &u, &e, &ip, nil, "LECTURA", "paciente", map[string]any{"a": 1, "b": 2})
	if h1 != h2 {
		t.Error("debe ser determinista")
	}
	otro := "p-2"
	if HashAuditoria(HashGenesis, ts, &u, &otro, &ip, nil, "LECTURA", "paciente", nil) == h1 {
		t.Error("cambiar un campo debe cambiar el hash")
	}
	if HashAuditoria("f", ts, &u, &e, &ip, nil, "LECTURA", "paciente", map[string]any{"a": 1, "b": 2}) == h1 {
		t.Error("cambiar el hash anterior debe cambiar el hash")
	}
}

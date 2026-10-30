package pruebas

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigo-backend/routes"
)

// Etapa 7 — cabeceras de seguridad y la app servida por la API (hallazgos de OWASP ZAP corregidos).
func TestCabecerasYFrontendServido(t *testing.T) {
	requiereBD(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>SIGO</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "imprimir.css"), []byte("body{}"), 0o600)
	copia := *ctl
	copia.Cfg.StaticDir = dir
	app2, _ := routes.NuevaApp(&copia)
	pedir := func(ruta string) (int, map[string]string) {
		res, err := app2.Test(httptest.NewRequest("GET", ruta, nil), -1)
		if err != nil {
			t.Fatal(err)
		}
		h := map[string]string{}
		for k, v := range res.Header {
			h[k] = strings.Join(v, ",")
		}
		return res.StatusCode, h
	}
	st, h := pedir("/")
	if st != 200 || !strings.Contains(h["Content-Security-Policy"], "default-src 'self'") || strings.Contains(h["Content-Security-Policy"], "unsafe-inline") {
		t.Errorf("CSP estricta en la app: %d %q", st, h["Content-Security-Policy"])
	}
	for _, k := range []string{"X-Frame-Options", "X-Content-Type-Options", "Referrer-Policy", "Permissions-Policy"} {
		if h[k] == "" {
			t.Errorf("falta la cabecera %s", k)
		}
	}
	if h["Strict-Transport-Security"] != "" {
		t.Error("HSTS sólo en producción (detrás de TLS)")
	}
	for ruta, quiere := range map[string]int{"/pacientes/123": 200, "/caja": 200, "/imprimir.css": 200, "/.git/config": 404, "/.env": 404, "/BitKeeper": 404, "/admin/": 404} {
		if st, _ := pedir(ruta); st != quiere {
			t.Errorf("%s: %d, se esperaba %d", ruta, st, quiere)
		}
	}
	if _, h := pedir("/api/health"); h["Cache-Control"] != "no-store" {
		t.Errorf("la API no se guarda en cachés: %q", h["Cache-Control"])
	}
}

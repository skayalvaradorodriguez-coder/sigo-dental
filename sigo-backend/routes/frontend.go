package routes

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// rutasApp: primer segmento de las rutas de la SPA (App.jsx). Cualquier otra ruta responde 404
// (con la app, que muestra "Página no encontrada"), para no aparentar archivos inexistentes.
var rutasApp = map[string]bool{"": true, "login": true, "pacientes": true, "agenda": true, "expediente": true, "presupuestos": true,
	"consentimientos": true, "caja": true, "inventario": true, "honorarios": true, "usuarios": true, "catalogos": true,
	"configuracion": true, "auditoria": true}

// ServirFrontend: con STATIC_DIR el backend entrega también la app React (un solo servidor
// en la nube, igual que MultiCatálogo). Rutas de la app → index.html (SPA).
func ServirFrontend(app *fiber.App, dir string) {
	app.Static("/assets", filepath.Join(dir, "assets"), fiber.Static{MaxAge: 31536000})
	app.Get("/*", func(c *fiber.Ctx) error {
		ruta := filepath.Join(dir, filepath.Clean("/"+c.Params("*")))
		if !strings.HasPrefix(ruta, filepath.Clean(dir)) {
			return c.SendStatus(404)
		}
		// Archivos y carpetas ocultos (.git, .env, .hg…): 404, nunca la app (hallazgo de OWASP ZAP).
		for _, parte := range strings.Split(c.Params("*"), "/") {
			if strings.HasPrefix(parte, ".") {
				return c.SendStatus(404)
			}
		}
		if info, err := os.Stat(ruta); err == nil && !info.IsDir() {
			if strings.HasSuffix(ruta, "sw.js") || strings.HasSuffix(ruta, ".webmanifest") {
				c.Set("Cache-Control", "no-cache")
			}
			return c.SendFile(ruta)
		}
		c.Set("Cache-Control", "no-cache")
		primero, _, _ := strings.Cut(c.Params("*"), "/")
		if !rutasApp[primero] {
			c.Status(404)
		}
		return c.SendFile(filepath.Join(dir, "index.html"))
	})
}

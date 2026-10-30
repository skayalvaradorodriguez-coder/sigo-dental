package routes

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ServirFrontend: con STATIC_DIR el backend entrega también la app React (un solo servidor
// en la nube, igual que MultiCatálogo). Rutas desconocidas → index.html (SPA).
func ServirFrontend(app *fiber.App, dir string) {
	app.Static("/assets", filepath.Join(dir, "assets"), fiber.Static{MaxAge: 31536000})
	app.Get("/*", func(c *fiber.Ctx) error {
		ruta := filepath.Join(dir, filepath.Clean("/"+c.Params("*")))
		if !strings.HasPrefix(ruta, filepath.Clean(dir)) {
			return c.SendStatus(404)
		}
		if info, err := os.Stat(ruta); err == nil && !info.IsDir() {
			if strings.HasSuffix(ruta, "sw.js") || strings.HasSuffix(ruta, ".webmanifest") {
				c.Set("Cache-Control", "no-cache")
			}
			return c.SendFile(ruta)
		}
		c.Set("Cache-Control", "no-cache")
		return c.SendFile(filepath.Join(dir, "index.html"))
	})
}

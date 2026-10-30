package controllers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/cache"
	"sigo-backend/middleware"
	"sigo-backend/permisos"
	"sigo-backend/repository"
	"sigo-backend/validacion"
)

// cacheado aplica Cache-Aside: Redis primero; si no está, PostgreSQL y se guarda 10 min.
func cacheado[T any](h *Controllers, c *fiber.Ctx, clave string, cargar func() (T, error)) error {
	var v T
	if h.Cache.Obtener(c.UserContext(), cache.ClaveCatalogos+clave, &v) {
		c.Set("X-Cache", "HIT")
		return c.JSON(v)
	}
	v, err := cargar()
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Cache.Guardar(c.UserContext(), cache.ClaveCatalogos+clave, v, cache.TTLCatalogos)
	c.Set("X-Cache", "MISS")
	return c.JSON(v)
}

// Estructura — GET /api/catalogos/estructura[?todos=true] (todos sólo para catalogos.gestionar)
func (h *Controllers) Estructura(c *fiber.Ctx) error {
	ctx := c.UserContext()
	if c.Query("todos") == "true" && middleware.Actual(c).Puede(permisos.CatalogosGestionar) {
		s, err := h.Repo.EstructuraSedes(ctx, false)
		if err != nil {
			return h.errorInterno(c, err)
		}
		return c.JSON(s)
	}
	return cacheado(h, c, "estructura", func() (any, error) { return h.Repo.EstructuraSedes(ctx, true) })
}

func (h *Controllers) Sillones(c *fiber.Ctx) error {
	return cacheado(h, c, "sillones", func() (any, error) { return h.Repo.SillonesActivos(c.UserContext()) })
}

func (h *Controllers) ListarProcedimientos(c *fiber.Ctx) error {
	if c.Query("todos") == "true" {
		p, err := h.Repo.Procedimientos(c.UserContext(), false)
		if err != nil {
			return h.errorInterno(c, err)
		}
		return c.JSON(p)
	}
	return cacheado(h, c, "procedimientos", func() (any, error) { return h.Repo.Procedimientos(c.UserContext(), true) })
}

func (h *Controllers) ListarConvenios(c *fiber.Ctx) error {
	if c.Query("todos") == "true" {
		p, err := h.Repo.Convenios(c.UserContext(), false)
		if err != nil {
			return h.errorInterno(c, err)
		}
		return c.JSON(p)
	}
	return cacheado(h, c, "convenios", func() (any, error) { return h.Repo.Convenios(c.UserContext(), true) })
}

func (h *Controllers) ListarOdontologos(c *fiber.Ctx) error {
	return cacheado(h, c, "odontologos", func() (any, error) { return h.Repo.Odontologos(c.UserContext()) })
}

// validarCatalogo convierte el JSON recibido en columnas permitidas, validando cada campo.
func validarCatalogo(cat string, in map[string]any, creando bool) (map[string]any, validacion.Errores) {
	errs := validacion.Errores{}
	out := map[string]any{}
	texto := func(campo, col string, min, max int, msg string, obligatorio bool) {
		v, ok := in[campo]
		if !ok {
			if obligatorio && creando {
				errs.Agregar(campo, msg)
			}
			return
		}
		s, esTexto := v.(string)
		if !esTexto {
			errs.Agregar(campo, msg)
			return
		}
		s = strings.TrimSpace(s)
		if len([]rune(s)) < min || len([]rune(s)) > max {
			errs.Agregar(campo, msg)
			return
		}
		if s == "" {
			out[col] = nil
		} else {
			out[col] = s
		}
	}
	uuid := func(campo, col string) {
		v, ok := in[campo]
		if !ok {
			if creando {
				errs.Agregar(campo, campo+" es obligatorio.")
			}
			return
		}
		s, _ := v.(string)
		if !validacion.EsUUID(s) {
			errs.Agregar(campo, campo+" inválido.")
			return
		}
		out[col] = s
	}
	if v, ok := in["activo"]; ok {
		b, esBool := v.(bool)
		if !esBool {
			errs.Agregar("activo", "activo debe ser verdadero o falso.")
		} else {
			out["activo"] = b
		}
	}
	permitidos := map[string][]string{
		"sedes": {"nombre", "direccion", "activo"}, "consultorios": {"sedeId", "nombre", "activo"},
		"sillones": {"consultorioId", "nombre", "activo"}, "procedimientos": {"codigo", "nombre", "duracionMin", "arancel", "activo"},
		"convenios": {"nombre", "aseguradora", "activo"},
	}
	for k := range in {
		ok := false
		for _, p := range permitidos[cat] {
			if p == k {
				ok = true
			}
		}
		if !ok {
			errs.Agregar(k, fmt.Sprintf("Campo no permitido: %s.", k))
		}
	}
	switch cat {
	case "sedes":
		texto("nombre", "nombre", 2, 100, "Nombre: 2 a 100 caracteres.", true)
		texto("direccion", "direccion", 0, 200, "Dirección: hasta 200 caracteres.", false)
	case "consultorios":
		if creando {
			uuid("sedeId", "sede_id")
		}
		texto("nombre", "nombre", 2, 80, "Nombre: 2 a 80 caracteres.", true)
	case "sillones":
		uuid("consultorioId", "consultorio_id")
		if !creando {
			if _, ok := in["consultorioId"]; !ok {
				delete(errs, "consultorioId")
			}
		}
		texto("nombre", "nombre", 1, 60, "Nombre: 1 a 60 caracteres.", true)
	case "procedimientos":
		if creando {
			v, _ := in["codigo"].(string)
			errs.Codigo("codigo", v)
			out["codigo"] = v
		} else if _, ok := in["codigo"]; ok {
			errs.Agregar("codigo", "El código no se puede cambiar.")
		}
		texto("nombre", "nombre", 2, 120, "Nombre: 2 a 120 caracteres.", true)
		if v, ok := in["duracionMin"]; ok || creando {
			f, esNum := v.(float64)
			if !esNum || f != float64(int(f)) || f < 5 || f > 480 {
				errs.Agregar("duracionMin", "Duración: entero entre 5 y 480 minutos.")
			} else {
				out["duracion_min"] = int(f)
			}
		}
		if v, ok := in["arancel"]; ok || creando {
			s, _ := v.(string)
			errs.Dinero("arancel", s)
			out["arancel"] = s
		}
	case "convenios":
		texto("nombre", "nombre", 2, 120, "Nombre: 2 a 120 caracteres.", true)
		texto("aseguradora", "aseguradora", 0, 120, "Aseguradora: hasta 120 caracteres.", false)
	}
	if creando {
		delete(out, "activo")
	}
	return out, errs
}

var entidadCatalogo = map[string]string{"sedes": "sede", "consultorios": "consultorio", "sillones": "sillon", "procedimientos": "procedimiento", "convenios": "convenio"}

// CrearCatalogo — POST /api/catalogos/:tipo
func (h *Controllers) CrearCatalogo(c *fiber.Ctx) error {
	cat := c.Params("tipo")
	if _, ok := entidadCatalogo[cat]; !ok {
		return noEncontrado(c, "Catálogo desconocido.")
	}
	var in map[string]any
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	vals, errs := validarCatalogo(cat, in, true)
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	fila, err := h.Repo.CrearCatalogo(c.UserContext(), cat, vals)
	if err != nil {
		return h.errorRepo(c, err, "", "Ya existe un registro con ese nombre o código.")
	}
	h.Cache.EliminarPatron(c.UserContext(), cache.ClaveCatalogos+"*")
	h.Auditar(c, auditCatalogo("CREACION", cat, fila["id"], map[string]any{"valores": in}))
	return c.Status(201).JSON(camel(fila))
}

// ActualizarCatalogo — PATCH /api/catalogos/:tipo/:id (desactivar en lugar de borrar)
func (h *Controllers) ActualizarCatalogo(c *fiber.Ctx) error {
	cat, id := c.Params("tipo"), c.Params("id")
	if _, ok := entidadCatalogo[cat]; !ok || !validacion.EsUUID(id) {
		return noEncontrado(c, "Registro no encontrado.")
	}
	var in map[string]any
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	vals, errs := validarCatalogo(cat, in, false)
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	antes, despues, err := h.Repo.ActualizarCatalogo(c.UserContext(), cat, id, vals)
	if err != nil {
		return h.errorRepo(c, err, "Registro no encontrado.", "Ya existe un registro con ese nombre o código.")
	}
	h.Cache.EliminarPatron(c.UserContext(), cache.ClaveCatalogos+"*")
	dif := map[string]any{}
	for k, v := range despues {
		if fmt.Sprint(antes[k]) != fmt.Sprint(v) {
			dif[k] = map[string]any{"de": antes[k], "a": v}
		}
	}
	h.Auditar(c, auditCatalogo("MODIFICACION", cat, id, map[string]any{"cambios": dif}))
	return c.JSON(camel(despues))
}

func auditCatalogo(accion, cat string, id any, det map[string]any) repository.Evento {
	return repository.Evento{Accion: accion, Entidad: entidadCatalogo[cat], EntidadID: fmt.Sprint(id), Detalle: det}
}

// camel convierte las columnas snake_case de row_to_json al formato camelCase de la API.
func camel(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		partes := strings.Split(k, "_")
		for i := 1; i < len(partes); i++ {
			if partes[i] != "" {
				partes[i] = strings.ToUpper(partes[i][:1]) + partes[i][1:]
			}
		}
		out[strings.Join(partes, "")] = v
	}
	if v, ok := out["arancel"]; ok {
		out["arancel"] = fmt.Sprintf("%.2f", toFloat(v))
	}
	return out
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		var f float64
		if _, err := fmt.Sscan(x, &f); err != nil {
			return 0
		}
		return f
	}
	return 0
}

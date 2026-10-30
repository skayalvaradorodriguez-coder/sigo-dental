package controllers

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/domain"
	"sigo-backend/middleware"
	"sigo-backend/models"
	"sigo-backend/repository"
	"sigo-backend/validacion"
)

// ================================================================ Inventario (RF-14)
//
// El stock no es un número que se edita: es la suma de los movimientos (entradas, salidas por
// procedimiento, ajustes con motivo y transferencias), que sólo se insertan.

var reCosto = regexp.MustCompile(`^\d{1,8}(\.\d{1,4})?$`)

func (h *Controllers) ListarUbicaciones(c *fiber.Ctx) error {
	us, err := h.Repo.Ubicaciones(c.UserContext())
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(us)
}

func (h *Controllers) ListarInsumos(c *fiber.Ctx) error {
	xs, err := h.Repo.Insumos(c.UserContext(), c.Query("todos") == "true")
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(xs)
}

type insumoIn struct {
	Codigo        string `json:"codigo"`
	Nombre        string `json:"nombre"`
	Unidad        string `json:"unidad"`
	CostoUnitario string `json:"costoUnitario"`
	StockMinimo   string `json:"stockMinimo"`
	Activo        *bool  `json:"activo"`
}

func validarInsumo(in *insumoIn, nuevo bool) (repository.InsumoDatos, validacion.Errores) {
	errs := validacion.Errores{}
	in.Codigo = strings.ToUpper(strings.TrimSpace(in.Codigo))
	if nuevo {
		errs.Codigo("codigo", in.Codigo)
	}
	errs.Longitud("nombre", strings.TrimSpace(in.Nombre), 2, 120, "Nombre: 2 a 120 caracteres.")
	errs.Longitud("unidad", strings.TrimSpace(in.Unidad), 1, 20, "Unidad: 1 a 20 caracteres (p. ej. unidad, ml, g).")
	if !reCosto.MatchString(strings.TrimSpace(in.CostoUnitario)) {
		errs.Agregar("costoUnitario", "Costo unitario: número positivo con hasta 4 decimales.")
	}
	minimo := "0"
	if strings.TrimSpace(in.StockMinimo) != "" && strings.TrimSpace(in.StockMinimo) != "0" {
		q, err := domain.ParseCantidad(in.StockMinimo, false)
		if err != nil {
			errs.Agregar("stockMinimo", "Stock mínimo: cantidad positiva con hasta 3 decimales.")
		}
		minimo = q.String()
	}
	activo := true
	if in.Activo != nil {
		activo = *in.Activo
	}
	return repository.InsumoDatos{Codigo: in.Codigo, Nombre: strings.TrimSpace(in.Nombre), Unidad: strings.TrimSpace(in.Unidad),
		Costo: strings.TrimSpace(in.CostoUnitario), Minimo: minimo, Activo: activo}, errs
}

// CrearInsumo — POST /api/inventario/insumos
func (h *Controllers) CrearInsumo(c *fiber.Ctx) error {
	var in insumoIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	d, errs := validarInsumo(&in, true)
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	id, err := h.Repo.CrearInsumo(c.UserContext(), d)
	if err != nil {
		return h.errorRepo(c, err, "", "Ya existe un insumo con ese código.")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "insumo", EntidadID: id, Detalle: map[string]any{"codigo": d.Codigo, "costo": d.Costo}})
	return c.Status(201).JSON(fiber.Map{"id": id})
}

// ActualizarInsumo — PATCH /api/inventario/insumos/:id (el código no cambia; nunca se borra)
func (h *Controllers) ActualizarInsumo(c *fiber.Ctx) error {
	var in insumoIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Insumo no encontrado.")
	}
	d, errs := validarInsumo(&in, false)
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	if err := h.Repo.ActualizarInsumo(c.UserContext(), id, d); err != nil {
		return h.errorRepo(c, err, "Insumo no encontrado.", "")
	}
	h.Auditar(c, repository.Evento{Accion: "MODIFICACION", Entidad: "insumo", EntidadID: id,
		Detalle: map[string]any{"nombre": d.Nombre, "costo": d.Costo, "minimo": d.Minimo, "activo": d.Activo}})
	return c.JSON(fiber.Map{"id": id})
}

// Stock — GET /api/inventario/stock?ubicacionId=
func (h *Controllers) Stock(c *fiber.Ctx) error {
	u := c.Query("ubicacionId")
	if u != "" && !validacion.EsUUID(u) {
		return malaPeticion(c, "VALIDACION", "Ubicación inválida.")
	}
	xs, err := h.Repo.Stock(c.UserContext(), u)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(xs)
}

// Kardex — GET /api/inventario/movimientos?insumoId=&procedimientoRealizadoId=
func (h *Controllers) Kardex(c *fiber.Ctx) error {
	ins, rz := c.Query("insumoId"), c.Query("procedimientoRealizadoId")
	if (ins != "" && !validacion.EsUUID(ins)) || (rz != "" && !validacion.EsUUID(rz)) {
		return malaPeticion(c, "VALIDACION", "Filtro inválido.")
	}
	xs, err := h.Repo.Movimientos(c.UserContext(), ins, rz)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(xs)
}

type movimientoIn struct {
	Tipo          string `json:"tipo"` // ENTRADA | AJUSTE | TRANSFERENCIA
	InsumoID      string `json:"insumoId"`
	UbicacionID   string `json:"ubicacionId"` // destino en una transferencia
	OrigenID      string `json:"origenId"`    // sólo TRANSFERENCIA
	Cantidad      string `json:"cantidad"`    // AJUSTE admite negativa
	CostoUnitario string `json:"costoUnitario"`
	Motivo        string `json:"motivo"`
}

// RegistrarMovimiento — POST /api/inventario/movimientos
func (h *Controllers) RegistrarMovimiento(c *fiber.Ctx) error {
	var in movimientoIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	errs.EnLista("tipo", in.Tipo, []string{"ENTRADA", "AJUSTE", "TRANSFERENCIA"}, "Tipo: ENTRADA, AJUSTE o TRANSFERENCIA (las salidas por procedimiento son automáticas).")
	errs.UUID("insumoId", in.InsumoID, true)
	errs.UUID("ubicacionId", in.UbicacionID, true)
	q, err := domain.ParseCantidad(in.Cantidad, in.Tipo == "AJUSTE")
	if err != nil {
		errs.Agregar("cantidad", "Cantidad inválida: hasta 3 decimales, distinta de cero"+map[bool]string{true: " (negativa para descontar).", false: " y positiva."}[in.Tipo == "AJUSTE"])
	}
	motivo := strings.TrimSpace(in.Motivo)
	errs.Longitud("motivo", motivo, 0, 300, "Motivo: hasta 300 caracteres.")
	var costo *string
	switch in.Tipo {
	case "ENTRADA":
		if in.CostoUnitario != "" {
			if !reCosto.MatchString(strings.TrimSpace(in.CostoUnitario)) {
				errs.Agregar("costoUnitario", "Costo unitario: número positivo con hasta 4 decimales.")
			}
			v := strings.TrimSpace(in.CostoUnitario)
			costo = &v
		}
	case "AJUSTE":
		if len([]rune(motivo)) < 5 {
			errs.Agregar("motivo", "Explique el ajuste (al menos 5 caracteres): conteo físico, vencimiento, rotura…")
		}
	case "TRANSFERENCIA":
		errs.UUID("origenId", in.OrigenID, true)
		if in.OrigenID == in.UbicacionID && in.OrigenID != "" {
			errs.Agregar("ubicacionId", "El destino debe ser distinto del origen.")
		}
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	ctx := c.UserContext()
	if ok, err := h.Repo.InsumoActivo(ctx, in.InsumoID); err != nil || !ok {
		return malaPeticion(c, "INSUMO_INVALIDO", "El insumo no existe o está inactivo.")
	}
	ubics := []string{in.UbicacionID}
	if in.Tipo == "TRANSFERENCIA" {
		ubics = append(ubics, in.OrigenID)
	}
	for _, u := range ubics {
		if ok, err := h.Repo.UbicacionActiva(ctx, u); err != nil || !ok {
			return malaPeticion(c, "UBICACION_INVALIDA", "La ubicación no existe o está inactiva.")
		}
	}
	uid := middleware.Actual(c).Usuario.ID
	var id string
	if in.Tipo == "TRANSFERENCIA" {
		id, err = h.Repo.Transferir(ctx, in.InsumoID, in.OrigenID, in.UbicacionID, q.String(), uid, validacion.Opcional(motivo))
	} else {
		id, err = h.Repo.RegistrarMovimiento(ctx, repository.NuevoMovimiento{Tipo: in.Tipo, InsumoID: in.InsumoID, UbicacionID: in.UbicacionID,
			Cantidad: q.String(), UsuarioID: uid, CostoUnitario: costo, Motivo: validacion.Opcional(motivo)})
	}
	if err != nil {
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "MOVIMIENTO_STOCK", Entidad: "insumo", EntidadID: in.InsumoID,
		Detalle: map[string]any{"tipo": in.Tipo, "cantidad": q.String(), "ubicacion": in.UbicacionID, "origen": in.OrigenID, "motivo": motivo}})
	return c.Status(201).JSON(fiber.Map{"id": id})
}

// RecetasConsumo — GET /api/inventario/recetas
func (h *Controllers) RecetasConsumo(c *fiber.Ctx) error {
	xs, err := h.Repo.RecetasConsumo(c.UserContext())
	if err != nil {
		return h.errorInterno(c, err)
	}
	for i := range xs {
		var t domain.Centavos
		for _, l := range xs[i].Lineas {
			v, _ := domain.ParseDinero(l.Costo)
			t += v
		}
		xs[i].CostoEstandar = t.String()
	}
	return c.JSON(xs)
}

// GuardarReceta — PUT /api/inventario/recetas/:procedimientoId {lineas:[{insumoId, cantidad}]}
func (h *Controllers) GuardarReceta(c *fiber.Ctx) error {
	var in struct {
		Lineas []struct {
			InsumoID string `json:"insumoId"`
			Cantidad string `json:"cantidad"`
		} `json:"lineas"`
	}
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	pid := c.Params("procedimientoId")
	ctx := c.UserContext()
	if ok, err := h.Repo.ProcedimientoActivo(ctx, pid); !validacion.EsUUID(pid) || err != nil || !ok {
		return noEncontrado(c, "Procedimiento no encontrado.")
	}
	errs := validacion.Errores{}
	if len(in.Lineas) > 30 {
		errs.Agregar("lineas", "Máximo 30 insumos por receta.")
	}
	lineas := []repository.LineaRecetaNueva{}
	vistos := map[string]bool{}
	for i, l := range in.Lineas {
		k := fmt.Sprintf("lineas[%d]", i)
		if !validacion.EsUUID(l.InsumoID) || vistos[l.InsumoID] {
			errs.Agregar(k+".insumoId", "Insumo inválido o repetido.")
			continue
		}
		vistos[l.InsumoID] = true
		if ok, err := h.Repo.InsumoActivo(ctx, l.InsumoID); err != nil || !ok {
			errs.Agregar(k+".insumoId", "El insumo no existe o está inactivo.")
			continue
		}
		q, err := domain.ParseCantidad(l.Cantidad, false)
		if err != nil {
			errs.Agregar(k+".cantidad", "Cantidad positiva con hasta 3 decimales.")
			continue
		}
		lineas = append(lineas, repository.LineaRecetaNueva{InsumoID: l.InsumoID, Cantidad: q.String()})
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	antes, _ := h.Repo.RecetasConsumo(ctx)
	var previa []models.LineaReceta
	for _, r := range antes {
		if r.ProcedimientoID == pid {
			previa = r.Lineas
		}
	}
	if err := h.Repo.ReemplazarReceta(ctx, pid, lineas); err != nil {
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "MODIFICACION", Entidad: "receta_consumo", EntidadID: pid, Detalle: map[string]any{"antes": previa, "despues": lineas}})
	return h.RecetasConsumo(c)
}

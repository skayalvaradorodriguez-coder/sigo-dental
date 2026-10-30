// Etapa 6: inventario (RF-14) y honorarios (RF-13).
import { apiFetch } from "./http";

export const getUbicaciones = () => apiFetch("/inventario/ubicaciones");
export const getInsumos = (todos = false) => apiFetch("/inventario/insumos", { query: { todos: todos ? "true" : "" } });
export const crearInsumo = (d) => apiFetch("/inventario/insumos", { method: "POST", body: d });
export const actualizarInsumo = (id, d) => apiFetch(`/inventario/insumos/${id}`, { method: "PATCH", body: d });
export const getStock = (ubicacionId) => apiFetch("/inventario/stock", { query: { ubicacionId } });
export const getMovimientos = (filtro) => apiFetch("/inventario/movimientos", { query: filtro });
export const registrarMovimiento = (d) => apiFetch("/inventario/movimientos", { method: "POST", body: d });
export const getRecetasConsumo = () => apiFetch("/inventario/recetas");
export const guardarRecetaConsumo = (procedimientoId, lineas) =>
  apiFetch(`/inventario/recetas/${procedimientoId}`, { method: "PUT", body: { lineas } });

export const getReglas = () => apiFetch("/honorarios/reglas");
export const crearRegla = (d) => apiFetch("/honorarios/reglas", { method: "POST", body: d });
export const getCalculo = (q) => apiFetch("/honorarios/calculo", { query: q });
export const getLiquidaciones = (odontologoId) => apiFetch("/honorarios/liquidaciones", { query: { odontologoId } });
export const getLiquidacion = (id) => apiFetch(`/honorarios/liquidaciones/${id}`);
export const crearLiquidacion = (d) => apiFetch("/honorarios/liquidaciones", { method: "POST", body: d });

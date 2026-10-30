// Catálogos: sedes → consultorios → sillones, procedimientos, convenios y odontólogos.
// El backend los sirve con caché Redis (cabecera X-Cache: HIT/MISS).
import { apiFetch } from "./http";

export const getEstructura = (todos = false) =>
  apiFetch("/catalogos/estructura", { query: { todos: todos ? "true" : "" } });
export const getSillones = () => apiFetch("/catalogos/sillones");
export const getProcedimientos = (todos = false) =>
  apiFetch("/catalogos/procedimientos", { query: { todos: todos ? "true" : "" } });
export const getConvenios = (todos = false) =>
  apiFetch("/catalogos/convenios", { query: { todos: todos ? "true" : "" } });
export const getOdontologos = () => apiFetch("/catalogos/odontologos");

// tipo: sedes | consultorios | sillones | procedimientos | convenios
export const crearCatalogo = (tipo, datos) => apiFetch(`/catalogos/${tipo}`, { method: "POST", body: datos });
export const actualizarCatalogo = (tipo, id, cambios) =>
  apiFetch(`/catalogos/${tipo}/${id}`, { method: "PATCH", body: cambios });

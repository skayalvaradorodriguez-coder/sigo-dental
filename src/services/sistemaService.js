// Panel, usuarios, ajustes y bitácora de auditoría (Etapa 1).
import { apiFetch } from "./http";

export const getPanel = () => apiFetch("/panel");

export const getUsuarios = () => apiFetch("/usuarios");
export const getRoles = () => apiFetch("/usuarios/roles");
export const crearUsuario = (datos) => apiFetch("/usuarios", { method: "POST", body: datos });
export const actualizarUsuario = (id, cambios) => apiFetch(`/usuarios/${id}`, { method: "PATCH", body: cambios });
export const restablecerPassword = (id) => apiFetch(`/usuarios/${id}/restablecer-password`, { method: "POST" });
export const restablecerMFA = (id) => apiFetch(`/usuarios/${id}/restablecer-mfa`, { method: "POST" });

export const getAjustes = () => apiFetch("/ajustes");
export const guardarAjuste = (clave, valor) =>
  apiFetch(`/ajustes/${encodeURIComponent(clave)}`, { method: "PUT", body: { valor } });

export const getAuditoria = (filtros) => apiFetch("/auditoria", { query: filtros });
export const verificarAuditoria = () => apiFetch("/auditoria/verificar", { method: "POST" });

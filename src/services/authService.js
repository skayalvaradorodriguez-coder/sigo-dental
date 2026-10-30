// Inicio de sesión en pasos: contraseña → (cambio de contraseña) → MFA → sesión completa.
import { apiFetch } from "./http";

export const login = (email, password) => apiFetch("/login", { method: "POST", body: { email, password } });
export const getPerfil = () => apiFetch("/me");
export const verificarMFA = (codigo) => apiFetch("/mfa/verificar", { method: "POST", body: { codigo } });
export const configurarMFA = () => apiFetch("/mfa/configurar", { method: "POST" });
export const activarMFA = (codigo) => apiFetch("/mfa/activar", { method: "POST", body: { codigo } });
export const cambiarPassword = (actual, nueva) => apiFetch("/password", { method: "POST", body: { actual, nueva } });
export const logout = () => apiFetch("/logout", { method: "POST" });
export const codigoDemo = () => apiFetch("/demo/mfa"); // sólo existe en el despliegue de demostración
export const estado = () => apiFetch("/health");
export const heartbeat = () => apiFetch("/heartbeat", { method: "POST" });

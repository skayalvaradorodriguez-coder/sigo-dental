// Cliente HTTP único de la aplicación. Todos los servicios llaman a la API a través de apiFetch:
// - agrega "Authorization: Bearer <token>" si hay sesión,
// - convierte las respuestas de error (APIError del backend en Go) en un ApiError con status y code,
// - si el servidor da la sesión por terminada (401), avisa a AuthContext para volver al login,
// - sin conexión responde con un mensaje claro en vez de un error genérico.
import { API_URL } from "../config";

// El token vive en sessionStorage (no localStorage): al cerrar la pestaña o el navegador la
// sesión desaparece del equipo. En un consultorio los equipos suelen ser compartidos.
const CLAVE_TOKEN = "sigo_token";

export const leerToken = () => {
  try {
    return sessionStorage.getItem(CLAVE_TOKEN);
  } catch {
    return null;
  }
};

export const guardarToken = (token) => {
  try {
    if (token) sessionStorage.setItem(CLAVE_TOKEN, token);
    else sessionStorage.removeItem(CLAVE_TOKEN);
  } catch {
    // sessionStorage no disponible: la sesión dura sólo mientras la página esté abierta.
  }
};

// Evento que escucha AuthContext cuando el backend rechaza el token.
export const EVENTO_SESION_PERDIDA = "sigo:sesion-perdida";

export const MENSAJE_SIN_CONEXION =
  "Sin conexión. Esta acción necesita internet; inténtelo cuando vuelva a estar en línea.";

export class ApiError extends Error {
  constructor(status, code, message, cuerpo = {}) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.extra = cuerpo?.extra ?? null;
    // Errores de validación por campo: { campo: "mensaje" } (code VALIDACION)
    this.campos = cuerpo?.extra?.campos ?? null;
  }
}

export const mensajeDe = (e) => (e instanceof ApiError ? e.message : "Ocurrió un error inesperado.");

export const apiFetch = async (ruta, { method = "GET", body, query } = {}) => {
  const url = new URL(`${API_URL}/api${ruta}`, window.location.origin);
  Object.entries(query ?? {}).forEach(([clave, valor]) => {
    if (valor !== undefined && valor !== null && valor !== "") url.searchParams.set(clave, valor);
  });

  if (typeof navigator !== "undefined" && navigator.onLine === false) {
    throw new ApiError(0, "SIN_CONEXION", MENSAJE_SIN_CONEXION);
  }

  const headers = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const token = leerToken();
  if (token) headers.Authorization = `Bearer ${token}`;

  let response;
  try {
    response = await fetch(url, { method, headers, body: body !== undefined ? JSON.stringify(body) : undefined });
  } catch {
    throw new ApiError(0, "RED", "No se pudo conectar con el servidor. Verifique que el backend esté en ejecución.");
  }

  if (response.status === 204) return null;
  const data = await response.json().catch(() => null);
  if (!response.ok) {
    const code = data?.code ?? (response.status === 403 ? "PROHIBIDO" : "ERROR");
    if (response.status === 401 && token && ruta !== "/login") {
      guardarToken(null);
      window.dispatchEvent(new CustomEvent(EVENTO_SESION_PERDIDA, { detail: code }));
    }
    throw new ApiError(response.status, code, data?.message ?? `Error ${response.status}`, data ?? {});
  }
  return data;
};

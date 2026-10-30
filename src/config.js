// URL base del backend. Vacía (recomendado) = mismo origen: el navegador llama a /api y Vite
// lo reenvía al backend en Go (proxy en vite.config.js). Se configura con VITE_API_URL en .env.
export const API_URL = (import.meta.env.VITE_API_URL ?? "").replace(/\/$/, "");

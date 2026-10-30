// Etapa 4: presupuestos y tareas (RF-07, RF-08), consentimientos (RF-06), recetas y archivos.
import { apiFetch } from "./http";

// Presupuestos
export const getPresupuestos = (estado) => apiFetch("/presupuestos", { query: { estado } });
export const getPresupuesto = (id) => apiFetch(`/presupuestos/${id}`);
export const getPresupuestosPaciente = (pacienteId) => apiFetch(`/pacientes/${pacienteId}/presupuestos`);
export const crearPresupuesto = (pacienteId, datos) =>
  apiFetch(`/pacientes/${pacienteId}/presupuestos`, { method: "POST", body: datos });
export const actualizarPresupuesto = (id, datos) => apiFetch(`/presupuestos/${id}`, { method: "PUT", body: datos });
export const autorizarDescuentos = (id, codigo) =>
  apiFetch(`/presupuestos/${id}/autorizar-descuentos`, { method: "POST", body: { codigo } });
export const presentarPresupuesto = (id) => apiFetch(`/presupuestos/${id}/presentar`, { method: "POST" });
export const decidirPresupuesto = (id, decisiones, nota) =>
  apiFetch(`/presupuestos/${id}/decision`, { method: "POST", body: { decisiones, nota } });
export const getTareas = (pacienteId) => apiFetch(`/pacientes/${pacienteId}/tareas`);

// Consentimientos
export const getPlantillas = () => apiFetch("/consentimientos/plantillas");
export const crearPlantilla = (datos) => apiFetch("/consentimientos/plantillas", { method: "POST", body: datos });
export const getConsentimientos = (estado) => apiFetch("/consentimientos", { query: { estado } });
export const getConsentimiento = (id) => apiFetch(`/consentimientos/${id}`);
export const getConsentimientosPaciente = (pacienteId) => apiFetch(`/pacientes/${pacienteId}/consentimientos`);
export const vistaPreviaConsentimiento = (pacienteId, datos) =>
  apiFetch(`/pacientes/${pacienteId}/consentimientos/vista-previa`, { method: "POST", body: datos });
export const crearConsentimiento = (pacienteId, datos) =>
  apiFetch(`/pacientes/${pacienteId}/consentimientos`, { method: "POST", body: datos });
export const verificarOTP = (id, codigo) =>
  apiFetch(`/consentimientos/${id}/verificar-otp`, { method: "POST", body: { codigo } });

// Recetas
export const getRecetas = (pacienteId) => apiFetch(`/pacientes/${pacienteId}/recetas`);
export const crearReceta = (pacienteId, datos) =>
  apiFetch(`/pacientes/${pacienteId}/recetas`, { method: "POST", body: datos });

// Archivos
export const getArchivos = (pacienteId) => apiFetch(`/pacientes/${pacienteId}/archivos`);
export const subirArchivo = (pacienteId, { archivo, categoria, descripcion }) => {
  const f = new FormData();
  f.append("archivo", archivo);
  f.append("categoria", categoria);
  f.append("descripcion", descripcion ?? "");
  return apiFetch(`/pacientes/${pacienteId}/archivos`, { method: "POST", body: f });
};
export const descargarArchivo = (id) => apiFetch(`/archivos/${id}/contenido`, { respuesta: "blob" });

// Agenda por sillón y profesional (RF-09/10/11, RN-03).
import { apiFetch } from "./http";

export const getCitas = ({ desde, hasta, sillonId, odontologoId, pacienteId }) =>
  apiFetch("/citas", { query: { desde, hasta, sillonId, odontologoId, pacienteId } });
export const getCita = (id) => apiFetch(`/citas/${id}`);
export const crearCita = (datos) => apiFetch("/citas", { method: "POST", body: datos });
export const reprogramarCita = (id, datos) => apiFetch(`/citas/${id}/reprogramar`, { method: "PATCH", body: datos });
export const cambiarEstadoCita = (id, estado, nota) =>
  apiFetch(`/citas/${id}/estado`, { method: "POST", body: { estado, nota } });

export const getBandeja = () => apiFetch("/notificaciones");
export const procesarNotificaciones = () => apiFetch("/notificaciones/procesar", { method: "POST" });

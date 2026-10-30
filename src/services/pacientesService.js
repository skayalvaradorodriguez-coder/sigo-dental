// Ficha única del paciente (RF-01) y anamnesis versionada (RF-02).
import { apiFetch } from "./http";

export const buscarPacientes = ({ q, page, limit }) => apiFetch("/pacientes", { query: { q, page, limit } });
export const getPaciente = (id) => apiFetch(`/pacientes/${id}`);
export const crearPaciente = (datos) => apiFetch("/pacientes", { method: "POST", body: datos });
export const actualizarPaciente = (id, datos) => apiFetch(`/pacientes/${id}`, { method: "PUT", body: datos });

export const getAnamnesis = (id) => apiFetch(`/pacientes/${id}/anamnesis`);
export const registrarAnamnesis = (id, datos) =>
  apiFetch(`/pacientes/${id}/anamnesis`, { method: "POST", body: datos });

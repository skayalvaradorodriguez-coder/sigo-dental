// Expediente clínico (Etapa 3): odontograma, periodontograma, notas SOEP y procedimientos realizados.
import { apiFetch } from "./http";

export const getOdontograma = (pacienteId) => apiFetch(`/pacientes/${pacienteId}/odontograma`);
export const marcarOdontograma = (pacienteId, marca) =>
  apiFetch(`/pacientes/${pacienteId}/odontograma`, { method: "POST", body: marca });

export const getPeriodontogramas = (pacienteId) => apiFetch(`/pacientes/${pacienteId}/periodontogramas`);
export const crearPeriodontograma = (pacienteId, examen) =>
  apiFetch(`/pacientes/${pacienteId}/periodontogramas`, { method: "POST", body: examen });

export const getNotas = (pacienteId) => apiFetch(`/pacientes/${pacienteId}/notas`);
export const crearNota = (pacienteId, nota) =>
  apiFetch(`/pacientes/${pacienteId}/notas`, { method: "POST", body: nota });
export const actualizarNota = (id, nota) => apiFetch(`/notas/${id}`, { method: "PUT", body: nota });
export const firmarNota = (id, codigo) => apiFetch(`/notas/${id}/firmar`, { method: "POST", body: { codigo } });
export const adendaNota = (id, texto, codigo) =>
  apiFetch(`/notas/${id}/adendas`, { method: "POST", body: { texto, codigo } });

export const getRealizados = (pacienteId) => apiFetch(`/pacientes/${pacienteId}/procedimientos`);
export const registrarRealizado = (pacienteId, datos) =>
  apiFetch(`/pacientes/${pacienteId}/procedimientos`, { method: "POST", body: datos });
export const adendaRealizado = (id, texto, codigo) =>
  apiFetch(`/procedimientos-realizados/${id}/adendas`, { method: "POST", body: { texto, codigo } });

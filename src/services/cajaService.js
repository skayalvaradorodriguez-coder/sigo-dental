// Etapa 5: caja con pagos mixtos (RF-12), cierre ciego (RN-02) y comprobantes (proveedor SIMULADO).
import { apiFetch } from "./http";

export const getMiCaja = () => apiFetch("/caja/sesion");
export const abrirCaja = (fondoInicial) => apiFetch("/caja/sesion", { method: "POST", body: { fondoInicial } });
export const cerrarCaja = (declarado) => apiFetch("/caja/sesion/cerrar", { method: "POST", body: declarado });
export const getMisCobros = () => apiFetch("/caja/cobros");
export const getPorCobrar = (pacienteId) => apiFetch(`/pacientes/${pacienteId}/por-cobrar`);
export const getCobrosPaciente = (pacienteId) => apiFetch(`/pacientes/${pacienteId}/cobros`);
export const crearCobro = (datos) => apiFetch("/cobros", { method: "POST", body: datos });
export const getCobro = (id) => apiFetch(`/cobros/${id}`);
export const anularCobro = (id, motivo) => apiFetch(`/cobros/${id}/anular`, { method: "POST", body: { motivo } });
export const reintentarComprobante = (id) => apiFetch(`/cobros/${id}/comprobante`, { method: "POST" });
export const getProveedorFacturacion = () => apiFetch("/facturacion/proveedor");

// Auditor
export const getCierres = (estado) => apiFetch("/caja/cierres", { query: { estado } });
export const getCierre = (id) => apiFetch(`/caja/cierres/${id}`);
export const revisarCierre = (id, nota) => apiFetch(`/caja/cierres/${id}/revisar`, { method: "POST", body: { nota } });
export const getCuentasPorCobrar = () => apiFetch("/caja/cuentas-por-cobrar");

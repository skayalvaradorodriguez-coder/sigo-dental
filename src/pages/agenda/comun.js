// Constantes de la cuadrícula de la agenda.
export const HORA_INICIO = 7; // horario operativo del SRS: 07:00 a 21:00 (RNF-02)
export const HORA_FIN = 21;
export const PX_POR_MIN = 1.1;
export const ALTO_CUADRICULA = (HORA_FIN - HORA_INICIO) * 60 * PX_POR_MIN;

// Colores por estado: los finales negativos se atenúan; "en atención" destaca.
export const ESTILO_ESTADO = {
  PROGRAMADO: "bg-white border-brand-500 text-ink",
  CONFIRMADO: "bg-brand-50 border-brand-700 text-brand-900",
  EN_SALA_ESPERA: "bg-warn-50 border-warn-700 text-warn-700",
  EN_ATENCION: "bg-brand-700 border-brand-900 text-white",
  ATENDIDO: "bg-ok-50 border-ok-700 text-ok-700",
  CANCELADO: "bg-canvas border-line text-muted line-through opacity-70",
  NO_ASISTIO: "bg-danger-50 border-danger-600/40 text-danger-700 opacity-80",
};

// Estados que ya no ocupan el sillón (la restricción EXCLUDE de la BD tampoco los considera).
export const INACTIVOS = ["CANCELADO", "NO_ASISTIO"];

export const minutosDesdeInicio = (d) => (d.getHours() - HORA_INICIO) * 60 + d.getMinutes();

// Próximo cuarto de hora + 15 min (hora sugerida para "Nueva cita").
export const proximoCuarto = () => {
  const d = new Date();
  d.setMinutes(Math.ceil(d.getMinutes() / 15) * 15 + 15, 0, 0);
  return d;
};

// Etiquetas legibles de los valores que devuelve la API (los códigos no se muestran crudos).
export const ETIQUETA_ROL = {
  ADMIN_TI: "Administrador TI",
  RECEPCIONISTA: "Recepción / Caja",
  ODONTOLOGO: "Odontólogo",
  ASISTENTE: "Asistente dental",
  AUDITOR: "Auditor / Contador",
};

export const ETIQUETA_ACCION = {
  LOGIN_EXITOSO: "Inicio de sesión",
  LOGIN_FALLIDO: "Intento fallido",
  CUENTA_BLOQUEADA: "Cuenta bloqueada",
  MFA_VERIFICADO: "MFA verificado",
  MFA_FALLIDO: "Código MFA inválido",
  MFA_ACTIVADO: "MFA activado",
  MFA_RESTABLECIDO: "Reset de MFA",
  LOGOUT: "Cierre de sesión",
  SESION_EXPIRADA: "Sesión expirada",
  PASSWORD_CAMBIADO: "Cambio de contraseña",
  PASSWORD_RESTABLECIDO: "Reset de contraseña",
  LECTURA: "Consulta",
  CREACION: "Creación",
  MODIFICACION: "Modificación",
  AUDITORIA_VERIFICADA: "Verificación de bitácora",
};

export const ESTADOS_CITA = [
  "PROGRAMADO",
  "CONFIRMADO",
  "EN_SALA_ESPERA",
  "EN_ATENCION",
  "ATENDIDO",
  "CANCELADO",
  "NO_ASISTIO",
];

export const ETIQUETA_ESTADO = {
  PROGRAMADO: "Programado",
  CONFIRMADO: "Confirmado",
  EN_SALA_ESPERA: "En sala de espera",
  EN_ATENCION: "En atención",
  ATENDIDO: "Atendido",
  CANCELADO: "Cancelado",
  NO_ASISTIO: "No asistió",
};

// Copia de domain/citas.go (Transiciones) sólo para mostrar los botones válidos.
// La regla la aplica el backend: si no coincide, responde 400 TRANSICION_INVALIDA.
export const TRANSICIONES = {
  PROGRAMADO: ["CONFIRMADO", "EN_SALA_ESPERA", "CANCELADO", "NO_ASISTIO"],
  CONFIRMADO: ["EN_SALA_ESPERA", "CANCELADO", "NO_ASISTIO"],
  EN_SALA_ESPERA: ["EN_ATENCION", "CANCELADO"],
  EN_ATENCION: ["ATENDIDO"],
  ATENDIDO: [],
  CANCELADO: [],
  NO_ASISTIO: [],
};

export const TONO_ESTADO = {
  PROGRAMADO: "brand",
  CONFIRMADO: "brand",
  EN_SALA_ESPERA: "warn",
  EN_ATENCION: "ok",
  ATENDIDO: "neutral",
  CANCELADO: "neutral",
  NO_ASISTIO: "danger",
};

export const ETIQUETA_DOCUMENTO = { CEDULA: "Cédula", PASAPORTE: "Pasaporte", OTRO: "Otro" };
export const ETIQUETA_SEXO = {
  FEMENINO: "Femenino",
  MASCULINO: "Masculino",
  OTRO: "Otro",
  NO_ESPECIFICA: "Prefiere no indicar",
};
export const ETIQUETA_CANAL = { WHATSAPP: "WhatsApp", SMS: "SMS", EMAIL: "Correo" };
export const ETIQUETA_ALERGIA = {
  MEDICAMENTO: "Medicamento",
  MATERIAL: "Material dental",
  ALIMENTO: "Alimento",
  OTRO: "Otro",
};
export const ETIQUETA_NOTIFICACION = {
  CONFIRMACION: "Confirmación",
  RECORDATORIO_48H: "Recordatorio 48 h",
  RECORDATORIO_24H: "Recordatorio 24 h",
};
export const TONO_NOTIFICACION = {
  PENDIENTE: "brand",
  ENVIADO: "ok",
  FALLIDO: "danger",
  CANCELADO: "neutral",
  OMITIDO: "warn",
};

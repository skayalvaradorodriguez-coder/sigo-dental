// Fechas en la hora local del navegador (la agenda se muestra en la zona del equipo).
export const dos = (n) => String(n).padStart(2, "0");

export const inicioDelDia = (d) => {
  const x = new Date(d);
  x.setHours(0, 0, 0, 0);
  return x;
};
export const sumarDias = (d, n) => {
  const x = new Date(d);
  x.setDate(x.getDate() + n);
  return x;
};
// Lunes de la semana de d.
export const inicioDeSemana = (d) => {
  const x = inicioDelDia(d);
  return sumarDias(x, -((x.getDay() + 6) % 7));
};
export const fechaISO = (d) => `${d.getFullYear()}-${dos(d.getMonth() + 1)}-${dos(d.getDate())}`;
export const horaMin = (d) => {
  const x = typeof d === "string" ? new Date(d) : d;
  return `${dos(x.getHours())}:${dos(x.getMinutes())}`;
};
export const combinar = (fecha, hora) => {
  const [a, m, d] = fecha.split("-").map(Number);
  const [h, mi] = hora.split(":").map(Number);
  return new Date(a, m - 1, d, h, mi, 0, 0);
};
export const mismoDia = (a, b) => fechaISO(a) === fechaISO(b);

export const fmtDiaLargo = (d) =>
  new Intl.DateTimeFormat("es", { weekday: "long", day: "numeric", month: "long", year: "numeric" }).format(d);
export const fmtDiaCorto = (d) => new Intl.DateTimeFormat("es", { weekday: "short", day: "numeric" }).format(d);
export const fmtDiaMes = (d) => new Intl.DateTimeFormat("es", { day: "numeric", month: "short" }).format(d);

// Edad cumplida a partir de AAAA-MM-DD.
export const edadDesde = (nacimiento, hoy = new Date()) => {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(nacimiento)) return null;
  const [a, m, d] = nacimiento.split("-").map(Number);
  let edad = hoy.getFullYear() - a;
  if (hoy.getMonth() + 1 < m || (hoy.getMonth() + 1 === m && hoy.getDate() < d)) edad--;
  return edad;
};

const fechaHora = new Intl.DateTimeFormat("es", { dateStyle: "medium", timeStyle: "short" });
const fechaHoraSeg = new Intl.DateTimeFormat("es", { dateStyle: "short", timeStyle: "medium" });
export const fmtFechaHora = (iso) => (iso ? fechaHora.format(new Date(iso)) : "—");
export const fmtFechaHoraSeg = (iso) => (iso ? fechaHoraSeg.format(new Date(iso)) : "—");

export const fmtRelativo = (iso) => {
  if (!iso) return "nunca";
  const seg = (Date.now() - new Date(iso).getTime()) / 1000;
  const rtf = new Intl.RelativeTimeFormat("es", { numeric: "auto" });
  if (seg < 60) return "hace un momento";
  if (seg < 3600) return rtf.format(-Math.round(seg / 60), "minute");
  if (seg < 86400) return rtf.format(-Math.round(seg / 3600), "hour");
  return rtf.format(-Math.round(seg / 86400), "day");
};

// Tiempo que falta hasta una fecha futura ("en 3 h", "en 25 min"); "ya" si pasó.
export const fmtFaltan = (iso) => {
  const seg = (new Date(iso).getTime() - Date.now()) / 1000;
  if (seg <= 0) return "ya";
  const rtf = new Intl.RelativeTimeFormat("es", { numeric: "auto" });
  if (seg < 3600) return rtf.format(Math.ceil(seg / 60), "minute");
  if (seg < 86400) return rtf.format(Math.round(seg / 3600), "hour");
  return rtf.format(Math.round(seg / 86400), "day");
};

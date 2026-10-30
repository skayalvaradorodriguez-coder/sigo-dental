// Dinero en centavos enteros (nunca float), igual que el backend. Los montos viajan como texto "150.00".

// aCentavos("12.5") → 1250; aCentavos("abc") → null. Acepta coma decimal.
export const aCentavos = (texto) => {
  const s = String(texto ?? "")
    .trim()
    .replace(",", ".");
  if (s === "") return 0;
  const m = /^(\d{1,9})(?:\.(\d{1,2}))?$/.exec(s);
  if (!m) return null;
  return Number(m[1]) * 100 + Number((m[2] ?? "").padEnd(2, "0"));
};

// deCentavos(1250) → "12.50"
export const deCentavos = (c) => {
  const n = Math.round(Number(c) || 0);
  const signo = n < 0 ? "-" : "";
  const a = Math.abs(n);
  return `${signo}${Math.floor(a / 100)}.${String(a % 100).padStart(2, "0")}`;
};

// fmtDinero("1234.5") → "1.234,50" (sin símbolo: no se asume país ni moneda).
export const fmtDinero = (texto) => {
  const c = typeof texto === "number" ? texto : aCentavos(texto);
  if (c === null) return "—";
  const [ent, dec] = deCentavos(c).split(".");
  return `${ent.replace(/\B(?=(\d{3})+(?!\d))/g, ".")},${dec}`;
};

// Totales del editor (antes de guardar). El backend recalcula y es la fuente de verdad.
export const totalesItems = (items, topePct = 10) => {
  const fases = new Map();
  let hon = 0;
  let lab = 0;
  let des = 0;
  const requiere = [];
  items.forEach((it, i) => {
    const h = aCentavos(it.honorario) ?? 0;
    const l = aCentavos(it.costoLaboratorio) ?? 0;
    const d = aCentavos(it.descuento) ?? 0;
    hon += h;
    lab += l;
    des += d;
    if (d > 0 && d * 100 > h * topePct) requiere.push(i);
    const f = fases.get(it.fase) ?? 0;
    fases.set(it.fase, f + h + l - d);
  });
  return {
    honorarios: hon,
    laboratorio: lab,
    descuentos: des,
    total: hon + lab - des,
    fases: [...fases.entries()].map(([fase, total]) => ({ fase, total })),
    requiereAutorizacion: requiere,
  };
};

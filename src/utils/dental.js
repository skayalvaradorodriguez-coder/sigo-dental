// Numeración y superficies dentales (copia de domain/dental.go para dibujar; el backend valida).
// Se guarda SIEMPRE FDI; Universal es sólo una forma de mostrar.

export const PERMANENTES_SUP = [18, 17, 16, 15, 14, 13, 12, 11, 21, 22, 23, 24, 25, 26, 27, 28];
export const PERMANENTES_INF = [48, 47, 46, 45, 44, 43, 42, 41, 31, 32, 33, 34, 35, 36, 37, 38];
export const TEMPORALES_SUP = [55, 54, 53, 52, 51, 61, 62, 63, 64, 65];
export const TEMPORALES_INF = [85, 84, 83, 82, 81, 71, 72, 73, 74, 75];

export const esPermanente = (d) => {
  const q = Math.floor(d / 10),
    n = d % 10;
  return q >= 1 && q <= 4 && n >= 1 && n <= 8;
};
export const esTemporal = (d) => {
  const q = Math.floor(d / 10),
    n = d % 10;
  return q >= 5 && q <= 8 && n >= 1 && n <= 5;
};
export const esSuperior = (d) => [1, 2, 5, 6].includes(Math.floor(d / 10));
export const esAnterior = (d) => d % 10 >= 1 && d % 10 <= 3;
// Cuadrantes del lado derecho del paciente (se dibujan a la izquierda): la cara mesial queda a la derecha.
export const ladoDerecho = (d) => [1, 4, 5, 8].includes(Math.floor(d / 10));

export const fdiAUniversal = (d) => {
  const q = Math.floor(d / 10),
    n = d % 10;
  if (esPermanente(d)) return String({ 1: 9 - n, 2: 8 + n, 3: 25 - n, 4: 24 + n }[q]);
  if (esTemporal(d)) return String.fromCharCode(65 + { 5: 5 - n, 6: 4 + n, 7: 15 - n, 8: 14 + n }[q]);
  return "?";
};

export const nombreDiente = (d, notacion) => (notacion === "UNIVERSAL" ? fdiAUniversal(d) : String(d));

// Superficies canónicas M, D, O, V, L. O se rotula I en anteriores y L se rotula P en la arcada superior.
export const SUPERFICIES = ["M", "D", "O", "V", "L"];
export const etiquetaSuperficie = (d, s) => {
  if (s === "O" && esAnterior(d)) return "I";
  if (s === "L" && esSuperior(d)) return "P";
  return s;
};
export const NOMBRE_SUPERFICIE = {
  M: "Mesial",
  D: "Distal",
  O: "Oclusal",
  I: "Incisal",
  V: "Vestibular",
  L: "Lingual",
  P: "Palatino",
};

// Periodontograma: 6 sitios por diente (vestibulares y linguales/palatinos).
export const SITIOS_V = ["MV", "V", "DV"];
export const SITIOS_L = ["ML", "L", "DL"];
export const nic = (ps, mg) => ps + mg; // NIC = PS + MG (MG positivo = recesión)

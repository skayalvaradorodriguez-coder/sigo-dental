// La copia de la numeración dental en el frontend debe coincidir con domain/dental.go.
import { describe, expect, it } from "vitest";
import {
  etiquetaSuperficie,
  fdiAUniversal,
  ladoDerecho,
  nic,
  PERMANENTES_INF,
  PERMANENTES_SUP,
  TEMPORALES_INF,
  TEMPORALES_SUP,
} from "../utils/dental";

describe("Numeración dental (RF-03)", () => {
  it("32 permanentes y 20 temporales, sin repetidos", () => {
    const todos = [...PERMANENTES_SUP, ...PERMANENTES_INF, ...TEMPORALES_SUP, ...TEMPORALES_INF];
    expect(todos).toHaveLength(52);
    expect(new Set(todos).size).toBe(52);
  });
  it("FDI → Universal en los extremos de cada cuadrante", () => {
    const casos = { 18: "1", 11: "8", 21: "9", 28: "16", 38: "17", 31: "24", 41: "25", 48: "32" };
    Object.entries(casos).forEach(([fdi, u]) => expect(fdiAUniversal(Number(fdi))).toBe(u));
    const temporales = { 55: "A", 51: "E", 61: "F", 65: "J", 75: "K", 71: "O", 81: "P", 85: "T" };
    Object.entries(temporales).forEach(([fdi, u]) => expect(fdiAUniversal(Number(fdi))).toBe(u));
  });
  it("Universal recorre 1…32 y A…T en el orden del odontograma", () => {
    expect([...PERMANENTES_SUP, ...[...PERMANENTES_INF].reverse()].map(fdiAUniversal)).toEqual(
      Array.from({ length: 32 }, (_, i) => String(i + 1)),
    );
    expect([...TEMPORALES_SUP, ...[...TEMPORALES_INF].reverse()].map(fdiAUniversal).join("")).toBe(
      "ABCDEFGHIJKLMNOPQRST",
    );
  });
  it("rotula incisal en anteriores y palatino en la arcada superior", () => {
    expect(etiquetaSuperficie(11, "O")).toBe("I");
    expect(etiquetaSuperficie(16, "O")).toBe("O");
    expect(etiquetaSuperficie(16, "L")).toBe("P");
    expect(etiquetaSuperficie(46, "L")).toBe("L");
  });
  it("lado derecho del paciente = cuadrantes 1, 4, 5 y 8", () => {
    expect([16, 46, 54, 84].every(ladoDerecho)).toBe(true);
    expect([26, 36, 64, 74].some(ladoDerecho)).toBe(false);
  });
  it("NIC = PS + MG", () => {
    expect(nic(4, 2)).toBe(6);
    expect(nic(5, -2)).toBe(3);
  });
});

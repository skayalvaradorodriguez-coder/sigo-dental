// Pruebas unitarias del frontend (Vitest + Testing Library). Ejecutar con: npm test
import { describe, expect, it } from "vitest";
import { MODULOS } from "../components/modulos";
import { combinar, edadDesde, fechaISO, inicioDeSemana } from "../utils/fechas";

describe("Fechas", () => {
  it("calcula la edad cumplida", () => {
    expect(edadDesde("2010-06-15", new Date(2026, 5, 14))).toBe(15);
    expect(edadDesde("2010-06-15", new Date(2026, 5, 15))).toBe(16);
    expect(edadDesde("no-es-fecha")).toBeNull();
  });
  it("la semana empieza el lunes", () => {
    expect(fechaISO(inicioDeSemana(new Date(2026, 9, 29)))).toBe("2026-10-26"); // jueves → lunes
    expect(fechaISO(inicioDeSemana(new Date(2026, 10, 1)))).toBe("2026-10-26"); // domingo → lunes anterior
  });
  it("combina fecha y hora locales", () => {
    const d = combinar("2026-11-03", "16:30");
    expect([d.getFullYear(), d.getMonth(), d.getDate(), d.getHours(), d.getMinutes()]).toEqual([2026, 10, 3, 16, 30]);
  });
});

describe("Menú", () => {
  it("cada módulo declara ruta, etapa y permisos con nombres del backend (español)", () => {
    MODULOS.forEach((m) => {
      expect(m.ruta.startsWith("/")).toBe(true);
      expect(m.etapa).toBeGreaterThanOrEqual(1);
      m.permisos.forEach((p) => expect(p).toMatch(/^[a-z]+\.[a-z]+$/));
    });
  });
});

// Etapa 4: dinero en centavos, escape al imprimir, editor de presupuesto y lienzo de firma.
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import LienzoFirma from "../pages/consentimientos/LienzoFirma";
import { fmtTamano } from "../utils/documentos";
import { aCentavos, deCentavos, fmtDinero, totalesItems } from "../utils/dinero";
import { escapar } from "../utils/imprimir";

describe("Dinero en centavos (igual que domain/presupuestos.go)", () => {
  it("convierte texto a centavos sin usar float", () => {
    expect(aCentavos("45")).toBe(4500);
    expect(aCentavos("45.5")).toBe(4550);
    expect(aCentavos("0,05")).toBe(5);
    expect(aCentavos("")).toBe(0);
    expect(aCentavos("1.234")).toBeNull();
    expect(aCentavos("-3")).toBeNull();
    expect(aCentavos("abc")).toBeNull();
    // 0.1 + 0.2 en float da 0.30000000000000004; en centavos es exacto.
    expect(deCentavos(aCentavos("0.1") + aCentavos("0.2"))).toBe("0.30");
  });
  it("da formato con separador de miles y sin moneda", () => {
    expect(fmtDinero("1234567.5")).toBe("1.234.567,50");
    expect(fmtDinero(5)).toBe("0,05");
    expect(fmtDinero("x")).toBe("—");
  });
  it("suma por fase: honorario + laboratorio − descuento", () => {
    const t = totalesItems([
      { fase: 1, honorario: "35.00" },
      { fase: 2, honorario: "45.00", descuento: "10.00" },
      { fase: 3, honorario: "180.00", costoLaboratorio: "20.00" },
    ]);
    expect(deCentavos(t.total)).toBe("270.00");
    expect(t.fases.map((f) => deCentavos(f.total))).toEqual(["35.00", "35.00", "200.00"]);
  });
});

describe("Impresión segura", () => {
  it("escapa el HTML de los datos del paciente", () => {
    expect(escapar(`<img src=x onerror="alert(1)">`)).toBe("&lt;img src=x onerror=&quot;alert(1)&quot;&gt;");
  });
  it("muestra tamaños legibles", () => {
    expect(fmtTamano(500)).toBe("500 B");
    expect(fmtTamano(2048)).toBe("2 KB");
    expect(fmtTamano(3.5 * 1024 * 1024)).toBe("3.5 MB");
  });
});

describe("Lienzo de firma (RF-06)", () => {
  it("empieza vacío y no cuenta un toque accidental como firma", () => {
    const ref = { current: null };
    const ctx = {
      scale: vi.fn(),
      fillRect: vi.fn(),
      beginPath: vi.fn(),
      moveTo: vi.fn(),
      lineTo: vi.fn(),
      stroke: vi.fn(),
      save: vi.fn(),
      restore: vi.fn(),
      setTransform: vi.fn(),
    };
    HTMLCanvasElement.prototype.getContext = () => ctx;
    render(<LienzoFirma ref={ref} />);
    expect(screen.getByText("Firme aquí")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: /Recuadro de firma/ })).toBeInTheDocument();
    expect(ref.current.vacio()).toBe(true);
  });
});

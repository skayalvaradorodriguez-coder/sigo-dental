// Etapa 6: formatos de inventario y tipos de regla de honorarios.
import { describe, expect, it } from "vitest";
import { fmtCantidad } from "../utils/dinero";
import { ETIQUETA_MOVIMIENTO, ETIQUETA_REGLA, TIPOS_REGLA } from "../utils/etiquetas";

describe("Inventario y honorarios", () => {
  it("muestra cantidades con hasta 3 decimales y signo", () => {
    expect(fmtCantidad("1.500")).toBe("1,5");
    expect(fmtCantidad("-2.000")).toBe("−2");
    expect(fmtCantidad("0.300")).toBe("0,3");
    expect(fmtCantidad("abc")).toBe("—");
  });
  it("las tres reglas del SRS (RF-13) y los cuatro tipos de movimiento tienen etiqueta", () => {
    expect(TIPOS_REGLA).toEqual(["PORCENTAJE", "MONTO_FIJO", "PORCENTAJE_NETO"]);
    TIPOS_REGLA.forEach((t) => expect(ETIQUETA_REGLA[t]).toBeTruthy());
    ["ENTRADA", "SALIDA_PROCEDIMIENTO", "AJUSTE", "TRANSFERENCIA"].forEach((t) =>
      expect(ETIQUETA_MOVIMIENTO[t]).toBeTruthy(),
    );
  });
});

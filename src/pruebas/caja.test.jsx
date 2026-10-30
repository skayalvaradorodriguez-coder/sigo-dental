// Etapa 5: estado del comprobante y suma de pagos mixtos en centavos.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EstadoComprobante } from "../pages/caja/ListaCobros";
import { aCentavos, deCentavos } from "../utils/dinero";
import { ETIQUETA_MEDIO, MEDIOS_PAGO } from "../utils/etiquetas";

const cobro = (comprobantes, anulacion = null) => ({ comprobantes, anulacion });

describe("Comprobantes (RF-12)", () => {
  it("rotula la factura del proveedor SIMULADO", () => {
    render(
      <EstadoComprobante
        cb={cobro([{ tipo: "FACTURA", estado: "AUTORIZADO", numero: "SIM-F-00000001", simulado: true }])}
      />,
    );
    expect(screen.getByText(/SIM-F-00000001/)).toHaveTextContent("SIMULADO");
  });
  it("muestra la falla de emisión y la nota de crédito del cobro anulado", () => {
    const { rerender } = render(
      <EstadoComprobante cb={cobro([{ tipo: "FACTURA", estado: "ERROR", simulado: true }])} />,
    );
    expect(screen.getByText(/sin respuesta/)).toBeInTheDocument();
    rerender(
      <EstadoComprobante
        cb={cobro(
          [
            { tipo: "FACTURA", estado: "AUTORIZADO", numero: "SIM-F-1", simulado: true },
            { tipo: "NOTA_CREDITO", estado: "AUTORIZADO", numero: "SIM-NC-1", simulado: true },
          ],
          { motivo: "Error" },
        )}
      />,
    );
    expect(screen.getByText(/SIM-NC-1/)).toBeInTheDocument();
    expect(screen.getByText("Anulado")).toBeInTheDocument();
  });
});

describe("Pago mixto", () => {
  it("los cuatro medios del SRS tienen etiqueta", () => {
    expect(MEDIOS_PAGO).toEqual(["EFECTIVO", "TARJETA", "TRANSFERENCIA", "SEGURO"]);
    MEDIOS_PAGO.forEach((m) => expect(ETIQUETA_MEDIO[m]).toBeTruthy());
  });
  it("reparte el total al centavo", () => {
    const total = aCentavos("350.00");
    const pagos = ["50.25", "120.00", "79.75", "100.00"].map(aCentavos);
    expect(pagos.reduce((a, b) => a + b, 0)).toBe(total);
    expect(deCentavos(total - aCentavos("50.25"))).toBe("299.75");
  });
});

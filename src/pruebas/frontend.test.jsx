// Pruebas unitarias del frontend (Vitest + Testing Library). Ejecutar con: npm test
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AlertasMedicas, PuntoAlerta } from "../components/AlertasMedicas";
import { MODULOS } from "../components/modulos";
import { ETIQUETA_ESTADO, ESTADOS_CITA, TRANSICIONES } from "../utils/etiquetas";
import { combinar, edadDesde, fechaISO, inicioDeSemana } from "../utils/fechas";

describe("AlertasMedicas (RF-02)", () => {
  it("muestra las críticas en un bloque role=alert y las advertencias aparte", () => {
    render(
      <AlertasMedicas
        alertas={[
          { codigo: "ALERGIA", etiqueta: "Alergia a Penicilina", severidad: "CRITICA" },
          { codigo: "SISTEMICA", etiqueta: "Asma", severidad: "ADVERTENCIA" },
        ]}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Alergia a Penicilina");
    expect(screen.getByRole("alert")).not.toHaveTextContent("Asma");
    expect(screen.getByText(/Antecedentes:/).parentElement).toHaveTextContent("Asma");
  });

  it("avisa cuando la anamnesis está pendiente", () => {
    render(<AlertasMedicas alertas={[]} anamnesisPendiente />);
    expect(screen.getByText("Anamnesis pendiente.")).toBeInTheDocument();
  });

  it("no dibuja nada si no hay alertas ni pendientes", () => {
    const { container } = render(<AlertasMedicas alertas={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("el indicador compacto prioriza la alerta crítica", () => {
    render(<PuntoAlerta criticas={2} pendiente />);
    expect(screen.getByTitle("Alerta médica crítica")).toHaveTextContent("2");
  });
});

describe("Máquina de estados de la cita (copia de domain/citas.go)", () => {
  it("define los 7 estados del SRS con etiqueta", () => {
    expect(ESTADOS_CITA).toHaveLength(7);
    ESTADOS_CITA.forEach((e) => expect(ETIQUETA_ESTADO[e]).toBeTruthy());
  });
  it("los estados finales no tienen salida", () => {
    ["ATENDIDO", "CANCELADO", "NO_ASISTIO"].forEach((e) => expect(TRANSICIONES[e]).toEqual([]));
  });
  it("sólo se llega a ATENDIDO desde EN_ATENCION", () => {
    const origen = Object.keys(TRANSICIONES).filter((e) => TRANSICIONES[e].includes("ATENDIDO"));
    expect(origen).toEqual(["EN_ATENCION"]);
  });
});

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

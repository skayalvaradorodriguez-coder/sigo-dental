// RNF-01 medido en el navegador: desde que se pide la ficha del paciente con 10 años de historia
// (VOL000001, `go run . volumen`) hasta que la pestaña muestra su contenido. Criterio: < 1 500 ms.
import { expect, test } from "@playwright/test";
import { entrarComo } from "./sesion";

const PESTANAS = [
  { id: "odontograma", listo: (p) => p.locator('svg[aria-label="Diente 16"]') },
  { id: "periodontograma", listo: (p) => p.getByText(/Control periodontal 1 \(volumen\)/).first() },
  { id: "evolucion", listo: (p) => p.getByText(/Evolución favorable \(nota 1\)/).first() },
  { id: "procedimientos", listo: (p) => p.getByText(/Registro de volumen 1$/).first() },
];

test("RNF-01: expediente con 10 años de historia en menos de 1,5 s", async ({ page }, info) => {
  test.skip(!["chromium", "firefox", "webkit"].includes(info.project.name), "se mide en escritorio");
  await entrarComo(page, "odontologo@demo.sigo");
  await page.goto("/expediente");
  await page.getByPlaceholder(/Buscar paciente/).fill("VOL000001");
  await page.getByRole("link", { name: /Paciente Volumen/ }).click();
  await page.waitForURL(/\/pacientes\//);
  const url = page.url().split("?")[0];
  const resultados = [];
  for (const t of PESTANAS) {
    const tiempos = [];
    for (let i = 0; i < 5; i++) {
      await page.goto("about:blank");
      const t0 = Date.now();
      await page.goto(`${url}?pestana=${t.id}`);
      await t.listo(page).waitFor();
      tiempos.push(Date.now() - t0);
    }
    tiempos.sort((a, b) => a - b);
    resultados.push({ pestana: t.id, mediana: tiempos[2], maximo: tiempos[4] });
  }
  info.annotations.push({ type: "RNF-01", description: JSON.stringify(resultados) });
  console.log(
    `RNF-01 (${info.project.name}):`,
    resultados.map((r) => `${r.pestana} mediana ${r.mediana} ms, máx ${r.maximo} ms`).join(" · "),
  );
  for (const r of resultados) expect(r.maximo, `${r.pestana} tarda ${r.maximo} ms`).toBeLessThan(1500);
});

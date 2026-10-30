// Recorrido básico en cada navegador y tamaño de pantalla (RNF-07, RNF-08).
import { expect, test } from "@playwright/test";
import { codigoMFA, entrarComo, sinDesbordeHorizontal } from "./sesion";

test("inicio de sesión real con contraseña y código MFA", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Correo electrónico").fill("auditor@demo.sigo");
  await page.getByLabel("Contraseña").fill("Demo-Sigo-2026");
  await page.getByRole("button", { name: "Continuar" }).click();
  for (let intento = 0; intento < 2; intento++) {
    await page.getByLabel("Código de verificación").fill(codigoMFA("auditor@demo.sigo"));
    await page.getByRole("button", { name: "Verificar" }).click();
    const ok = await page
      .getByRole("heading", { name: /Hola,/ })
      .waitFor({ timeout: 4000 })
      .then(
        () => true,
        () => false,
      );
    if (ok) return;
    await page.waitForTimeout(31_000 - (Date.now() % 30_000)); // anti-replay: esperar la siguiente ventana
  }
  throw new Error("MFA no aceptado");
});

test("odontólogo: expediente (odontograma, SOEP) sin desbordes", async ({ page }) => {
  await entrarComo(page, "odontologo@demo.sigo");
  expect(await sinDesbordeHorizontal(page)).toBe(true);
  await page.goto("/expediente");
  await page.getByPlaceholder(/Buscar paciente/).fill("Torres");
  await page.getByRole("link", { name: /Torres Vera/ }).click();
  await expect(page.getByText("Alerta médica crítica")).toBeVisible();
  await page.getByRole("tab", { name: "Odontograma", exact: true }).click();
  await expect(page.locator('svg[aria-label="Diente 16"]')).toBeVisible();
  await page.getByRole("tab", { name: "Evolución (SOEP)" }).click();
  await expect(page.getByText(/Refiere sensibilidad/).first()).toBeVisible();
  expect(await sinDesbordeHorizontal(page)).toBe(true);
});

test("recepción: agenda y caja sin desbordes", async ({ page }) => {
  await entrarComo(page, "recepcion@demo.sigo");
  await page.goto("/agenda");
  await expect(page.getByRole("button", { name: /Nueva cita/ }).first()).toBeVisible();
  expect(await sinDesbordeHorizontal(page)).toBe(true);
  await page.goto("/caja");
  await expect(page.getByText("Facturación electrónica SIMULADA")).toBeVisible();
  expect(await sinDesbordeHorizontal(page)).toBe(true);
});

test("auditor: conciliación de caja y honorarios", async ({ page }) => {
  await entrarComo(page, "auditor@demo.sigo");
  await page.goto("/caja");
  await expect(page.getByRole("tab", { name: "Por revisar" })).toBeVisible();
  await page.goto("/honorarios");
  await expect(page.getByRole("tab", { name: "Reglas" })).toBeVisible();
  expect(await sinDesbordeHorizontal(page)).toBe(true);
});

// Con la API sirviendo la build (STATIC_DIR) se aplica la CSP estricta: ninguna pantalla debe violarla,
// tampoco la impresión (iframe con /imprimir.css), la firma en pantalla ni el odontograma.
test("sin violaciones de la política de seguridad de contenidos (CSP)", async ({ page }) => {
  const violaciones = [];
  page.on("console", (m) => /Content Security Policy|Refused to/i.test(m.text()) && violaciones.push(m.text()));
  page.on("pageerror", (e) => violaciones.push(e.message));
  await entrarComo(page, "odontologo@demo.sigo");
  await page.goto("/expediente");
  await page.getByPlaceholder(/Buscar paciente/).fill("Torres");
  await page.getByRole("link", { name: /Torres Vera/ }).click();
  await expect(page.locator('svg[aria-label="Diente 16"]')).toBeVisible();
  await page.getByRole("tab", { name: "Presupuestos" }).click();
  await page.getByRole("button", { name: /Plan de tratamiento integral/ }).click();
  await page.evaluate(() => (window.print = () => {})); // el diálogo de impresión no aplica en pruebas
  await page.getByRole("button", { name: "Imprimir" }).click();
  await page.waitForTimeout(800);
  const frame = page.frames().find((f) => f !== page.mainFrame());
  if (frame) await expect.poll(() => frame.evaluate(() => getComputedStyle(document.body).marginTop)).toBe("24px");
  await page.reload(); // cierra el detalle del presupuesto (en Firefox tapaba las pestañas)
  await page.getByRole("tab", { name: "Consentimientos" }).click();
  await page.getByRole("button", { name: "Nuevo consentimiento" }).click();
  await page.goto("/inventario");
  await page.goto("/honorarios");
  expect(violaciones, violaciones.join("\n")).toEqual([]);
});

// Pruebas de navegador (Etapa 7): RNF-07 (pantallas) y RNF-08 (navegadores).
// Requisitos: backend corriendo con datos demo y `go run . volumen` (paciente VOL000001), y el
// frontend en SIGO_URL (por defecto http://localhost:5173, o http://localhost:3000 con STATIC_DIR).
// Primera vez: npx playwright install chromium firefox webkit
import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  workers: 1, // los datos demo son compartidos: una prueba a la vez
  reporter: [["list"], ["html", { open: "never", outputFolder: "e2e-informe" }]],
  use: {
    baseURL: process.env.SIGO_URL ?? "http://localhost:5173",
    locale: "es-EC",
    timezoneId: "America/Guayaquil",
    trace: "retain-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1366, height: 768 } } },
    { name: "firefox", use: { ...devices["Desktop Firefox"], viewport: { width: 1366, height: 768 } } },
    { name: "webkit", use: { ...devices["Desktop Safari"], viewport: { width: 1366, height: 768 } } },
    { name: "edge", use: { ...devices["Desktop Edge"], channel: "msedge" } }, // sólo si Edge está instalado
    {
      name: "tablet-10.5",
      use: { ...devices["iPad Pro 11"], browserName: "chromium", viewport: { width: 834, height: 1112 } },
    },
    { name: "fullhd", use: { ...devices["Desktop Chrome"], viewport: { width: 1920, height: 1080 } } },
    { name: "celular", use: { ...devices["Pixel 7"] } },
  ],
});

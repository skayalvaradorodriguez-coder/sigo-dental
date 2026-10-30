// Sesión de prueba: el backend (fuera de producción) emite un token ya verificado con
// `go run . token <correo>`; se coloca en sessionStorage antes de cargar la app. El inicio de sesión
// real con contraseña y código MFA se prueba aparte en humo.spec.js.
import { execFileSync } from "node:child_process";
import path from "node:path";

const backend = path.resolve(import.meta.dirname, "..", "sigo-backend");
const binario = process.env.SIGO_BIN; // opcional: binario compilado (más rápido que go run)

const ejecutar = (args) =>
  execFileSync(binario ?? "go", binario ? args : ["run", ".", ...args], { cwd: backend, encoding: "utf8" }).trim();

export const token = (correo) => ejecutar(["token", correo]).split("\n").pop();
export const codigoMFA = (correo) => ejecutar(["totp", correo]).match(/\b\d{6}\b/)[0];

export const entrarComo = async (page, correo) => {
  const t = token(correo);
  await page.addInitScript((v) => sessionStorage.setItem("sigo_token", v), t);
  await page.goto("/");
  await page.getByRole("heading", { name: /Hola,/ }).waitFor();
  await page
    .getByRole("button", { name: "Entendido" })
    .click({ timeout: 1500 })
    .catch(() => {});
};

// Sin desplazamiento horizontal de la página (RNF-07).
export const sinDesbordeHorizontal = (page) =>
  page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth + 1);

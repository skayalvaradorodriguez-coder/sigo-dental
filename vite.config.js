import process from "node:process";
import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { VitePWA } from "vite-plugin-pwa";

// Colores de la marca (los mismos del menú lateral).
const COLOR_TEMA = "#0b4f5c";
const COLOR_FONDO = "#f4f7f9";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  // El navegador llama a /api en el MISMO origen que la app y Vite lo reenvía al backend en Go.
  const proxy = {
    "/api": { target: env.VITE_PROXY_TARGET || "http://127.0.0.1:3000", changeOrigin: true },
  };
  const allowedHosts = [".trycloudflare.com", ".ngrok-free.app", ".ngrok.app"];

  return {
    server: { host: true, proxy, allowedHosts },
    preview: { host: true, proxy, allowedHosts },
    test: { environment: "jsdom", setupFiles: ["./src/pruebas/setup.js"], css: false },
    plugins: [
      react(),
      tailwindcss(),
      VitePWA({
        // "prompt": la versión nueva espera a que el usuario pulse "Actualizar" (AvisoActualizacion.jsx).
        registerType: "prompt",
        includeAssets: ["favicon.ico", "favicon.svg", "logo.svg", "apple-touch-icon-180x180.png", "robots.txt"],
        devOptions: { enabled: true, type: "module", navigateFallback: "index.html" },
        manifest: {
          id: "/",
          name: "SIGO-Dental",
          short_name: "SIGO-Dental",
          description: "Gestión odontológica: pacientes, agenda por sillón y expediente clínico digital.",
          lang: "es",
          dir: "ltr",
          start_url: "/",
          scope: "/",
          display: "standalone",
          theme_color: COLOR_TEMA,
          background_color: COLOR_FONDO,
          categories: ["medical", "business", "productivity"],
          icons: [
            { src: "pwa-64x64.png", sizes: "64x64", type: "image/png" },
            { src: "pwa-192x192.png", sizes: "192x192", type: "image/png" },
            { src: "pwa-512x512.png", sizes: "512x512", type: "image/png" },
            { src: "maskable-icon-512x512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
          ],
          shortcuts: [
            { name: "Agenda", short_name: "Agenda", url: "/agenda", icons: [{ src: "pwa-192x192.png", sizes: "192x192", type: "image/png" }] },
            { name: "Pacientes", short_name: "Pacientes", url: "/pacientes", icons: [{ src: "pwa-192x192.png", sizes: "192x192", type: "image/png" }] },
          ],
        },
        workbox: {
          // Sólo el "app shell" (HTML, JS, CSS, íconos, fuentes) queda disponible sin conexión.
          globPatterns: ["**/*.{js,css,html,svg,png,ico,woff2}"],
          cleanupOutdatedCaches: true,
          clientsClaim: true,
          navigateFallback: "/index.html",
          navigateFallbackDenylist: [/^\/api\//],
          // IMPORTANTE (datos de salud): NO hay runtimeCaching para /api. Ninguna respuesta con
          // datos de pacientes, citas o auditoría se guarda en el dispositivo; todo va a la red.
        },
      }),
    ],
  };
});

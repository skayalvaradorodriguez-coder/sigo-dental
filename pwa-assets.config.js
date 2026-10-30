// Genera los íconos de la PWA a partir de public/logo.svg:
//   npm run generar-iconos
// Crea pwa-64x64.png, pwa-192x192.png, pwa-512x512.png, maskable-icon-512x512.png,
// apple-touch-icon-180x180.png y favicon.ico en la carpeta public/.
import { defineConfig, minimal2023Preset } from "@vite-pwa/assets-generator/config";

export default defineConfig({
  headLinkOptions: { preset: "2023" },
  preset: {
    ...minimal2023Preset,
    // El logo ya tiene fondo de borde a borde, así que no se agrega relleno extra.
    maskable: { ...minimal2023Preset.maskable, padding: 0, resizeOptions: { background: "#0b4f5c" } },
    apple: { ...minimal2023Preset.apple, padding: 0, resizeOptions: { background: "#0b4f5c" } },
  },
  images: ["public/logo.svg"],
});

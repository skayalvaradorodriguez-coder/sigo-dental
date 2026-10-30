import { useSyncExternalStore } from "react";
import { estadoInstalacion, mostrarDialogoInstalacion, suscribirInstalacion } from "../pwa/instalacion";

// Devuelve { estado, instalar }. estado: "disponible" | "ios" | "instalada" | "no".
export const useInstalarPWA = () => {
  const estado = useSyncExternalStore(suscribirInstalacion, estadoInstalacion, () => "no");
  return { estado, instalar: mostrarDialogoInstalacion };
};

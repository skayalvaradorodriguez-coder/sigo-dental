import { useSyncExternalStore } from "react";

// Preferencia de notación dental del usuario (FDI o UNIVERSAL). Es una comodidad de este navegador,
// no un dato del paciente: se guarda en localStorage (con try/catch por si no está disponible).
const CLAVE = "sigo_notacion";
const EVENTO = "sigo:notacion";

const leer = () => {
  try {
    return localStorage.getItem(CLAVE) === "UNIVERSAL" ? "UNIVERSAL" : "FDI";
  } catch {
    return "FDI";
  }
};

const suscribir = (avisar) => {
  window.addEventListener(EVENTO, avisar);
  window.addEventListener("storage", avisar);
  return () => {
    window.removeEventListener(EVENTO, avisar);
    window.removeEventListener("storage", avisar);
  };
};

export const useNotacion = () => {
  const notacion = useSyncExternalStore(suscribir, leer, () => "FDI");
  const cambiar = (n) => {
    try {
      localStorage.setItem(CLAVE, n);
    } catch {
      // sin almacenamiento: dura sólo esta vista
    }
    window.dispatchEvent(new Event(EVENTO));
  };
  return [notacion, cambiar];
};

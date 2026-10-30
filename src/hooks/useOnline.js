import { useSyncExternalStore } from "react";

// Escucha los eventos online/offline del navegador.
const suscribir = (avisar) => {
  window.addEventListener("online", avisar);
  window.addEventListener("offline", avisar);
  return () => {
    window.removeEventListener("online", avisar);
    window.removeEventListener("offline", avisar);
  };
};

// true si el navegador tiene conexión. Se actualiza solo al perder o recuperar la red.
export const useOnline = () =>
  useSyncExternalStore(
    suscribir,
    () => navigator.onLine,
    () => true,
  );

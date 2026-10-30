import { useCallback, useState } from "react";
import { mensajeDe } from "../services/http";

// Ejecuta una escritura (POST/PUT/PATCH) y expone { ejecutar, enviando, error, reiniciar }.
// ejecutar(...args) devuelve la respuesta o lanza; el mensaje de error queda en "error".
export const useAccion = (accion) => {
  const [enviando, setEnviando] = useState(false);
  const [error, setError] = useState("");

  const ejecutar = useCallback(
    async (...args) => {
      setEnviando(true);
      setError("");
      try {
        return await accion(...args);
      } catch (e) {
        setError(mensajeDe(e));
        throw e;
      } finally {
        setEnviando(false);
      }
    },
    [accion],
  );

  return { ejecutar, enviando, error, reiniciar: () => setError("") };
};

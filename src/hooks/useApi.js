import { useCallback, useEffect, useMemo, useState } from "react";

// Ejecuta una llamada a la API y expone { data, isLoading, error, recargar }.
// "cargar" debe ser estable (useCallback) para no repetir la petición en cada render.
// Mientras llega una respuesta nueva se conserva la anterior (útil al paginar).
export const useApi = (cargar) => {
  const [version, setVersion] = useState(0);
  // Cada petición es un objeto nuevo: sirve para saber si el resultado guardado es el actual.
  const peticion = useMemo(() => ({ cargar, version }), [cargar, version]);
  const [resultado, setResultado] = useState({ peticion: null, data: null, error: "" });

  useEffect(() => {
    let activo = true;
    peticion
      .cargar()
      .then((data) => {
        if (activo) setResultado({ peticion, data, error: "" });
      })
      .catch((err) => {
        if (activo) setResultado((prev) => ({ peticion, data: prev.data, error: err.mensajeCompleto ?? err.message }));
      });
    return () => {
      activo = false;
    };
  }, [peticion]);

  const recargar = useCallback(() => setVersion((v) => v + 1), []);
  return {
    data: resultado.data,
    isLoading: resultado.peticion !== peticion,
    error: resultado.peticion === peticion ? resultado.error : "",
    recargar,
  };
};

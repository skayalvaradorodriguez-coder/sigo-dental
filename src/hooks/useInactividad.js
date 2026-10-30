import { useEffect, useRef, useState } from "react";
import { heartbeat } from "../services/authService";

const EVENTOS = ["pointerdown", "keydown", "wheel", "touchstart"];
const AVISO_SEGUNDOS = 60;
const LATIDO_MS = 60_000;

// Cierre por inactividad en el cliente (RNF-05). El servidor aplica la misma regla por su cuenta
// (sesiones.ultima_actividad); esto sólo mejora la experiencia:
//  - avisa 60 s antes de cerrar,
//  - envía un "heartbeat" (máx. 1 por minuto) mientras el usuario interactúa, para que escribir
//    una nota larga sin guardar no cuente como inactividad en el servidor.
export const useInactividad = (minutos, alVencer) => {
  const ultimaActividad = useRef(0);
  const ultimoLatido = useRef(0);
  const [segundosRestantes, setSegundosRestantes] = useState(null);

  useEffect(() => {
    const limiteMs = minutos * 60_000;
    ultimaActividad.current = Date.now();
    ultimoLatido.current = Date.now();
    const marcar = () => {
      ultimaActividad.current = Date.now();
      if (Date.now() - ultimoLatido.current > LATIDO_MS) {
        ultimoLatido.current = Date.now();
        heartbeat().catch(() => undefined);
      }
    };
    EVENTOS.forEach((e) => window.addEventListener(e, marcar, { passive: true }));
    const reloj = window.setInterval(() => {
      const quedan = Math.ceil((limiteMs - (Date.now() - ultimaActividad.current)) / 1000);
      if (quedan <= 0) {
        window.clearInterval(reloj);
        alVencer();
      } else setSegundosRestantes(quedan <= AVISO_SEGUNDOS ? quedan : null);
    }, 1000);
    return () => {
      EVENTOS.forEach((e) => window.removeEventListener(e, marcar));
      window.clearInterval(reloj);
    };
  }, [minutos, alVencer]);

  const seguirActivo = () => {
    ultimaActividad.current = Date.now();
    ultimoLatido.current = Date.now();
    setSegundosRestantes(null);
    heartbeat().catch(() => undefined);
  };

  return { segundosRestantes, seguirActivo };
};

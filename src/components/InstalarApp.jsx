import { useState } from "react";
import { useInstalarPWA } from "../hooks/useInstalarPWA";

// Opción "Instalar app" del menú de perfil.
// - Chrome / Edge / Android: abre el diálogo nativo.
// - iPhone / iPad (Safari): muestra cómo agregarla a la pantalla de inicio.
// - Ya instalada o navegador sin soporte: no se muestra.
const InstalarApp = ({ alTerminar }) => {
  const { estado, instalar } = useInstalarPWA();
  const [verPasosIOS, setVerPasosIOS] = useState(false);

  if (estado === "instalada" || estado === "no") return null;

  const alPulsar = async () => {
    if (estado === "ios") {
      setVerPasosIOS(true);
      return;
    }
    await instalar();
    alTerminar?.();
  };

  return (
    <>
      <button
        onClick={alPulsar}
        className="flex items-center w-full min-h-11 px-4 text-sm text-brand-700 font-semibold hover:bg-brand-50"
      >
        📲 Instalar app
      </button>

      {verPasosIOS && (
        <div
          className="fixed inset-0 z-50 bg-black/50 flex items-end sm:items-center justify-center"
          role="dialog"
          aria-modal="true"
        >
          <div className="bg-white w-full sm:max-w-sm rounded-t-2xl sm:rounded-2xl p-6 space-y-4">
            <h2 className="text-lg font-bold text-ink">Instalar en iPhone o iPad</h2>
            <ol className="list-decimal list-inside space-y-2 text-sm text-muted">
              <li>
                En Safari, toca el botón <b>Compartir</b> (el cuadrado con la flecha hacia arriba).
              </li>
              <li>
                Elige <b>Agregar a pantalla de inicio</b>.
              </li>
              <li>
                Toca <b>Agregar</b>. SIGO-Dental aparecerá como una app más.
              </li>
            </ol>
            <button
              onClick={() => {
                setVerPasosIOS(false);
                alTerminar?.();
              }}
              className="w-full min-h-11 rounded-lg bg-brand-700 text-white font-semibold hover:bg-brand-800"
            >
              Entendido
            </button>
          </div>
        </div>
      )}
    </>
  );
};

export default InstalarApp;

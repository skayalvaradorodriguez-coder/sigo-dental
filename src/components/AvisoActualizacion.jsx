import { useRegisterSW } from "virtual:pwa-register/react";

const UNA_HORA = 60 * 60 * 1000;

// Registra el service worker y avisa cuando:
// - la app quedó lista para usarse sin conexión (primera visita), o
// - hay una versión nueva esperando (el usuario decide cuándo recargar).
const AvisoActualizacion = () => {
  const {
    offlineReady: [listaOffline, setListaOffline],
    needRefresh: [hayNueva, setHayNueva],
    updateServiceWorker,
  } = useRegisterSW({
    onRegisteredSW(_url, registro) {
      // Mientras la app esté abierta, revisamos si hay versión nueva cada hora.
      if (registro) setInterval(() => registro.update(), UNA_HORA);
    },
  });

  if (!listaOffline && !hayNueva) return null;

  const cerrar = () => {
    setListaOffline(false);
    setHayNueva(false);
  };

  return (
    <div
      role="alert"
      className="fixed z-50 left-4 right-4 sm:left-auto sm:right-6 sm:w-96 bg-ink text-white rounded-xl shadow-xl p-4"
      style={{ bottom: "calc(1rem + env(safe-area-inset-bottom))" }}
    >
      <p className="font-semibold">
        {hayNueva ? "Hay una nueva versión de SIGO-Dental" : "Lista para usar sin conexión"}
      </p>
      <p className="text-sm text-white/75 mt-1">
        {hayNueva
          ? "Actualice para usar la última versión. Guarde antes lo que esté editando."
          : "La aplicación abre sin conexión, pero los datos de pacientes y agenda siempre requieren internet (no se guardan en el equipo)."}
      </p>
      <div className="flex gap-2 mt-3">
        {hayNueva && (
          <button
            onClick={() => updateServiceWorker(true)}
            className="min-h-11 flex-1 rounded-lg bg-brand-200 text-brand-900 font-semibold hover:bg-white"
          >
            Actualizar
          </button>
        )}
        <button
          onClick={cerrar}
          className="min-h-11 flex-1 rounded-lg border border-white/30 font-semibold hover:bg-white/10"
        >
          {hayNueva ? "Más tarde" : "Entendido"}
        </button>
      </div>
    </div>
  );
};

export default AvisoActualizacion;

import { useOnline } from "../hooks/useOnline";

// Franja que aparece solo cuando el navegador pierde la conexión.
const BannerSinConexion = () => {
  const online = useOnline();
  if (online) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      className="bg-warn-50 text-warn-700 border-b border-warn-700/20 text-sm font-semibold text-center px-4 py-2"
    >
      Sin conexión — los datos clínicos no se guardan en el equipo; reconéctese para continuar.
    </div>
  );
};

export default BannerSinConexion;

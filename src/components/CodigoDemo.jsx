// Despliegue de demostración (APP_ENV=demo): muestra el código MFA vigente del usuario ficticio para
// ingresar o firmar sin app autenticadora. En producción la ruta no existe y no se muestra nada.
import { useEffect, useState } from "react";
import * as auth from "../services/authService";

const CodigoDemo = ({ alObtener }) => {
  const [hay, setHay] = useState(false);
  const [dato, setDato] = useState(null);
  useEffect(() => {
    auth.estado().then(
      (r) => setHay(Boolean(r?.demo)),
      () => {},
    );
  }, []);
  if (!hay) return null;
  const pedir = () =>
    auth.codigoDemo().then(
      (d) => {
        setDato(d);
        alObtener?.(d.codigo);
      },
      () => {},
    );
  return (
    <div className="rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900">
      <p>Demostración con datos ficticios: no necesita app autenticadora.</p>
      {dato && (
        <p className="mt-1">
          Código: <strong className="font-mono text-base">{dato.codigo}</strong> (vale {dato.segundos} s; cada código se
          usa una sola vez)
        </p>
      )}
      <button type="button" className="mt-1 font-medium underline" onClick={pedir}>
        {dato ? "Pedir otro código" : "Mostrar código de demostración"}
      </button>
    </div>
  );
};

export default CodigoDemo;

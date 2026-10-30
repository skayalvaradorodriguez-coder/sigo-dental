// Módulo "Caja y facturación" (RF-12, RN-02). Recepción cobra y cierra a ciegas; el Auditor concilia.
import { useCallback } from "react";
import { Alert, PageHeader } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { getProveedorFacturacion } from "../../services/cajaService";
import CajaRecepcion from "./CajaRecepcion";
import Cierres from "./Cierres";

const Caja = () => {
  const { puede } = useAuth();
  const prov = useApi(useCallback(() => getProveedorFacturacion(), []));
  const opera = puede("caja.operar");
  return (
    <>
      <PageHeader
        title={opera ? "Caja" : "Cierres de caja"}
        description={
          opera
            ? "Cobros con pago mixto (efectivo, tarjeta, transferencia y seguro) y cierre ciego al final del turno."
            : "Conciliación de los cierres ciegos: teórico del sistema frente a lo contado por la cajera."
        }
      />
      {prov.data?.simulado && (
        <div className="mb-4">
          <Alert tone="warn" title="Facturación electrónica SIMULADA">
            Los comprobantes se numeran como «SIM-…» y no tienen validez tributaria: falta definir el país y su
            autoridad tributaria para conectar el proveedor real.
          </Alert>
        </div>
      )}
      {opera ? <CajaRecepcion /> : <Cierres />}
    </>
  );
};

export default Caja;

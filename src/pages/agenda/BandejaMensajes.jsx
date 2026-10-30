// Bandeja de confirmaciones y recordatorios 48 h / 24 h (RF-11). Proveedor SIMULADO: nada sale del sistema.
import { useCallback, useState } from "react";
import { Alert, Button, EmptyState, Modal, Spinner } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useAccion } from "../../hooks/useAccion";
import { useApi } from "../../hooks/useApi";
import { getBandeja, procesarNotificaciones } from "../../services/citasService";
import { fmtFechaHora } from "../../utils/fechas";
import FilaNotificacion from "./FilaNotificacion";

const BandejaMensajes = ({ alCerrar }) => {
  const { puede } = useAuth();
  const { data, isLoading, error, recargar } = useApi(useCallback(() => getBandeja(), []));
  const procesar = useAccion(procesarNotificaciones);
  const [resultado, setResultado] = useState(null);

  return (
    <Modal open title="Bandeja de mensajes (RF-11)" onClose={alCerrar}>
      <div className="space-y-3">
        {data?.proveedor.simulado && (
          <Alert tone="warn" title="Proveedor SIMULADO">
            Ningún mensaje sale del sistema. Se muestran el contenido y la programación tal como se enviarían. Para
            envíos reales hay que integrar un proveedor (WhatsApp Business, SMS o correo) con sus credenciales.
          </Alert>
        )}
        {puede("citas.escribir") && (
          <div className="flex items-center justify-between gap-2">
            <p className="text-xs text-muted">Los mensajes vencidos se procesan automáticamente cada 30 s.</p>
            <Button
              variant="secondary"
              className="shrink-0 whitespace-nowrap"
              loading={procesar.enviando}
              onClick={() =>
                procesar
                  .ejecutar()
                  .then((r) => {
                    setResultado(r);
                    recargar();
                  })
                  .catch(() => {})
              }
            >
              Procesar ahora
            </Button>
          </div>
        )}
        {resultado && (
          <Alert tone="ok">
            Procesados: {resultado.enviados} enviados (simulado), {resultado.fallidos} con error.
          </Alert>
        )}
        {(error || procesar.error) && <Alert tone="danger">{error || procesar.error}</Alert>}
        {isLoading && !data && <Spinner />}
        {data && data.data.length === 0 && <EmptyState title="Sin mensajes todavía" />}
        <ul className="max-h-[50vh] divide-y divide-line overflow-y-auto">
          {data?.data.map((n) => (
            <div key={n.id}>
              <p className="pt-2 text-xs text-muted">
                {n.paciente} · cita {fmtFechaHora(n.inicioCita)}
              </p>
              <FilaNotificacion n={n} />
            </div>
          ))}
        </ul>
      </div>
    </Modal>
  );
};

export default BandejaMensajes;

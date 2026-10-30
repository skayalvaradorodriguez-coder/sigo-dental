import { Badge } from "../../components/ui";
import { ETIQUETA_CANAL, ETIQUETA_NOTIFICACION, TONO_NOTIFICACION } from "../../utils/etiquetas";
import { fmtFechaHora } from "../../utils/fechas";

// Un mensaje al paciente (RF-11). Con el proveedor simulado se marca claramente que NO se envió.
const FilaNotificacion = ({ n }) => (
  <li className="py-2 text-sm">
    <div className="flex flex-wrap items-center gap-2">
      <span className="font-medium">{ETIQUETA_NOTIFICACION[n.tipo]}</span>
      <span className="text-muted">
        {ETIQUETA_CANAL[n.canal]} · {fmtFechaHora(n.programadaPara)}
      </span>
      <Badge tone={TONO_NOTIFICACION[n.estado]}>{n.estado.charAt(0) + n.estado.slice(1).toLowerCase()}</Badge>
      {n.simulado && <Badge tone="warn">SIMULADO · no se envió</Badge>}
    </div>
    {n.motivo && <p className="text-xs text-muted">{n.motivo}</p>}
    {n.mensaje && <p className="mt-1 rounded-md bg-canvas p-2 text-xs">{n.mensaje}</p>}
    {n.idProveedor && <p className="mt-0.5 font-mono text-[11px] text-muted">{n.idProveedor}</p>}
  </li>
);

export default FilaNotificacion;

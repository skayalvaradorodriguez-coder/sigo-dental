// Módulo "Tratamientos y presupuestos": bandeja por estado. El trabajo se hace en la ficha del
// paciente (pestaña Presupuestos), donde están el odontograma y las alertas a la vista.
import { FileText } from "lucide-react";
import { useCallback, useState } from "react";
import { Link } from "react-router-dom";
import { Alert, Badge, Card, EmptyState, PageHeader, Skeleton, Tabs } from "../../components/ui";
import { useApi } from "../../hooks/useApi";
import { getPresupuestos } from "../../services/planesService";
import { fmtDinero } from "../../utils/dinero";
import { ETIQUETA_PRESUPUESTO, TONO_PRESUPUESTO } from "../../utils/etiquetas";
import { fmtFechaHora } from "../../utils/fechas";

const FILTROS = [
  { id: "", label: "Todos" },
  { id: "BORRADOR", label: "Borradores" },
  { id: "PRESENTADO", label: "Esperan decisión" },
  { id: "APROBADO_PARCIAL", label: "Aprobados parcialmente" },
  { id: "APROBADO", label: "Aprobados" },
  { id: "RECHAZADO", label: "Rechazados" },
];

const Presupuestos = () => {
  const [estado, setEstado] = useState("PRESENTADO");
  const { data, isLoading, error } = useApi(useCallback(() => getPresupuestos(estado), [estado]));
  return (
    <>
      <PageHeader
        title="Tratamientos y presupuestos"
        description="Planes por fases y sesiones. Al aprobarse (total o parcialmente) cada procedimiento aceptado se convierte en una tarea por agendar."
      />
      <Tabs value={estado} onChange={setEstado} tabs={FILTROS} />
      <Card className="mt-4">
        {isLoading && !data ? (
          <Skeleton className="m-4 h-40" />
        ) : error ? (
          <div className="p-4">
            <Alert tone="danger">{error}</Alert>
          </div>
        ) : data.length === 0 ? (
          <EmptyState icon={<FileText className="size-8" />} title="No hay presupuestos en este estado" />
        ) : (
          <ul className="divide-y divide-line">
            {data.map((p) => (
              <li key={p.id}>
                <Link
                  to={`/pacientes/${p.pacienteId}?pestana=presupuestos&presupuesto=${p.id}`}
                  className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 text-sm hover:bg-canvas"
                >
                  <span className="min-w-48 flex-1">
                    <span className="font-medium">{p.paciente}</span>
                    <span className="block text-xs text-muted">
                      {p.titulo} · {p.odontologo} · actualizado {fmtFechaHora(p.actualizadoEn)}
                    </span>
                  </span>
                  {p.descuentosPendientes > 0 && <Badge tone="warn">Descuento por autorizar</Badge>}
                  <span className="tabular-nums">{fmtDinero(p.totales.total)}</span>
                  <Badge tone={TONO_PRESUPUESTO[p.estado]}>{ETIQUETA_PRESUPUESTO[p.estado]}</Badge>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </>
  );
};

export default Presupuestos;

// Pestaña "Presupuestos" de la ficha: lista, editor, detalle y tareas agendables (RF-07, RF-08).
// ?presupuesto=<id> abre directamente un presupuesto (enlace desde la bandeja del módulo).
import { CalendarPlus, FileText, Plus } from "lucide-react";
import { useCallback, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { Alert, Badge, Button, Card, EmptyState, SectionTitle, Skeleton } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { useNotacion } from "../../hooks/useNotacion";
import { getPresupuestosPaciente, getTareas } from "../../services/planesService";
import { nombreDiente } from "../../utils/dental";
import { fmtDinero } from "../../utils/dinero";
import { ETIQUETA_PRESUPUESTO, ETIQUETA_TAREA, TONO_PRESUPUESTO, TONO_TAREA } from "../../utils/etiquetas";
import { fmtFechaHora } from "../../utils/fechas";
import DetallePresupuesto from "./DetallePresupuesto";
import EditorPresupuesto from "./EditorPresupuesto";

export const TareasPaciente = ({ paciente }) => {
  const { puede } = useAuth();
  const [notacion] = useNotacion();
  const navegar = useNavigate();
  const { data, error } = useApi(useCallback(() => getTareas(paciente.id), [paciente.id]));
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data) return <Skeleton className="h-24" />;
  if (!data.length) return null;
  return (
    <div>
      <SectionTitle>Tareas del tratamiento aprobado</SectionTitle>
      <Card>
        <ul className="divide-y divide-line">
          {data.map((t) => (
            <li key={t.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 text-sm">
              <span className="w-24 text-xs text-muted">
                Fase {t.fase} · S{t.sesion}
              </span>
              <span className="min-w-48 flex-1">
                <span className="font-medium">
                  {t.codigo} · {t.procedimiento}
                </span>
                {t.diente && <span className="text-muted"> · {nombreDiente(t.diente, notacion)}</span>}
                <span className="block text-xs text-muted">{t.presupuesto}</span>
              </span>
              <Badge tone={TONO_TAREA[t.estado]}>{ETIQUETA_TAREA[t.estado]}</Badge>
              {t.estado === "AGENDADA" && <span className="text-xs text-muted">{fmtFechaHora(t.citaInicio)}</span>}
              {t.estado === "REALIZADA" && <span className="text-xs text-muted">{fmtFechaHora(t.realizadoEn)}</span>}
              {t.estado === "PENDIENTE" && puede("citas.escribir") && (
                <Button variant="secondary" onClick={() => navegar(`/agenda?nuevo=${paciente.id}&tarea=${t.id}`)}>
                  <CalendarPlus className="size-4" /> Agendar
                </Button>
              )}
            </li>
          ))}
        </ul>
      </Card>
    </div>
  );
};

const PresupuestosPaciente = ({ paciente }) => {
  const { puede } = useAuth();
  const [params, setParams] = useSearchParams();
  const abierto = params.get("presupuesto");
  const abrir = (id) => {
    const p = new URLSearchParams(params);
    if (id) p.set("presupuesto", id);
    else p.delete("presupuesto");
    setParams(p, { replace: true });
  };
  const [editor, setEditor] = useState(null); // null | { existente?: presupuesto }
  const [version, setVersion] = useState(0);
  const { data, isLoading, error, recargar } = useApi(
    useCallback(() => getPresupuestosPaciente(paciente.id), [paciente.id]),
  );
  const cambio = () => {
    recargar();
    setVersion((v) => v + 1);
  };

  if (editor)
    return (
      <EditorPresupuesto
        paciente={paciente}
        existente={editor.existente}
        alCancelar={() => setEditor(null)}
        alGuardar={(p) => {
          setEditor(null);
          cambio();
          abrir(p.id);
        }}
      />
    );
  if (abierto)
    return (
      <DetallePresupuesto
        key={`${abierto}-${version}`}
        id={abierto}
        paciente={paciente}
        alVolver={() => abrir(null)}
        alEditar={(p) => setEditor({ existente: p })}
        alCambiar={cambio}
      />
    );

  if (isLoading && !data) return <Skeleton className="h-48" />;
  if (error) return <Alert tone="danger">{error}</Alert>;
  return (
    <div className="space-y-5">
      {puede("presupuestos.escribir") && (
        <Button onClick={() => setEditor({})}>
          <Plus className="size-4" /> Nuevo presupuesto
        </Button>
      )}
      {data.length === 0 ? (
        <Card>
          <EmptyState icon={<FileText className="size-8" />} title="Sin presupuestos">
            El odontólogo elabora el plan por fases; recepción registra la decisión del paciente.
          </EmptyState>
        </Card>
      ) : (
        <Card>
          <ul className="divide-y divide-line">
            {data.map((p) => (
              <li key={p.id}>
                <button
                  onClick={() => abrir(p.id)}
                  className="flex w-full flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 text-left text-sm hover:bg-canvas"
                >
                  <span className="min-w-48 flex-1">
                    <span className="font-medium">{p.titulo}</span>
                    <span className="block text-xs text-muted">
                      {p.odontologo} · {fmtFechaHora(p.creadoEn)} · {p.items.length} procedimiento(s)
                    </span>
                  </span>
                  <span className="tabular-nums">{fmtDinero(p.totales.total)}</span>
                  <Badge tone={TONO_PRESUPUESTO[p.estado]}>{ETIQUETA_PRESUPUESTO[p.estado]}</Badge>
                </button>
              </li>
            ))}
          </ul>
        </Card>
      )}
      <TareasPaciente key={version} paciente={paciente} />
    </div>
  );
};

export default PresupuestosPaciente;

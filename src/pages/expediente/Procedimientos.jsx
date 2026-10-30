// Procedimientos realizados (RN-01): se registran firmados y no se editan ni se borran (tampoco en la
// base de datos). Una corrección es una adenda firmada que queda debajo del registro original.
import { Lock, Plus } from "lucide-react";
import { useCallback, useState } from "react";
import { Alert, Button, Card, Checkbox, EmptyState, Select, Skeleton, Textarea } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { useNotacion } from "../../hooks/useNotacion";
import { getProcedimientos } from "../../services/catalogosService";
import { adendaRealizado, getRealizados, registrarRealizado } from "../../services/expedienteService";
import { getUbicaciones } from "../../services/inventarioService";
import { getTareas } from "../../services/planesService";
import {
  etiquetaSuperficie,
  nombreDiente,
  PERMANENTES_INF,
  PERMANENTES_SUP,
  SUPERFICIES,
  TEMPORALES_INF,
  TEMPORALES_SUP,
} from "../../utils/dental";
import { fmtFechaHora } from "../../utils/fechas";
import Adendas from "./Adendas";
import ModalFirma from "./ModalFirma";

const TODOS = [...PERMANENTES_SUP, ...PERMANENTES_INF, ...TEMPORALES_SUP, ...TEMPORALES_INF];

const Formulario = ({ pacienteId, notacion, alContinuar, alCancelar }) => {
  const procedimientos = useApi(useCallback(() => getProcedimientos(), []));
  // Tareas del presupuesto aprobado aún no realizadas (RF-08): al elegir una, se completan sus datos.
  const tareas = useApi(useCallback(() => getTareas(pacienteId).catch(() => []), [pacienteId]));
  const abiertas = (tareas.data ?? []).filter((t) => t.estado !== "REALIZADA");
  // Inventario (RF-14): de dónde se descuentan los insumos de la receta de consumo.
  const ubicaciones = useApi(useCallback(() => getUbicaciones().catch(() => []), []));
  const [f, setF] = useState({
    tareaId: "",
    procedimientoId: "",
    diente: "",
    superficies: [],
    observaciones: "",
    ubicacionId: "",
  });
  const [error, setError] = useState("");
  const d = Number(f.diente);
  return (
    <form
      className="space-y-3"
      noValidate
      onSubmit={(e) => {
        e.preventDefault();
        if (!f.procedimientoId) return setError("Seleccione el procedimiento.");
        alContinuar({
          ...(f.tareaId ? { tareaId: f.tareaId } : {}),
          ...(f.ubicacionId ? { ubicacionId: f.ubicacionId } : {}),
          procedimientoId: f.procedimientoId,
          ...(f.diente ? { diente: d, superficies: f.superficies } : {}),
          observaciones: f.observaciones.trim(),
        });
      }}
    >
      {error && <Alert tone="danger">{error}</Alert>}
      {abiertas.length > 0 && (
        <Select
          label="Tarea del tratamiento aprobado (opcional)"
          value={f.tareaId}
          onChange={(e) => {
            const t = abiertas.find((x) => x.id === e.target.value);
            setF(
              t
                ? {
                    ...f,
                    tareaId: t.id,
                    procedimientoId: t.procedimientoId,
                    diente: t.diente ? String(t.diente) : "",
                    superficies: t.superficies,
                  }
                : { ...f, tareaId: "" },
            );
          }}
        >
          <option value="">Procedimiento fuera del presupuesto</option>
          {abiertas.map((t) => (
            <option key={t.id} value={t.id}>
              Fase {t.fase} · S{t.sesion} · {t.procedimiento}
              {t.diente ? ` · ${nombreDiente(t.diente, notacion)}` : ""}
            </option>
          ))}
        </Select>
      )}
      <div className="grid gap-3 sm:grid-cols-2">
        <Select
          label="Procedimiento"
          disabled={!!f.tareaId}
          value={f.procedimientoId}
          onChange={(e) => setF({ ...f, procedimientoId: e.target.value })}
        >
          <option value="">Seleccione…</option>
          {procedimientos.data?.map((p) => (
            <option key={p.id} value={p.id}>
              {p.codigo} · {p.nombre}
            </option>
          ))}
        </Select>
        <Select
          label="Diente (opcional)"
          value={f.diente}
          onChange={(e) => setF({ ...f, diente: e.target.value, superficies: [] })}
        >
          <option value="">Sin diente específico</option>
          {TODOS.map((x) => (
            <option key={x} value={x}>
              {nombreDiente(x, notacion)}
              {notacion === "UNIVERSAL" ? ` (FDI ${x})` : ""}
            </option>
          ))}
        </Select>
      </div>
      {f.diente && (
        <fieldset>
          <legend className="mb-1 text-sm font-medium">Superficies</legend>
          <div className="flex flex-wrap gap-4">
            {SUPERFICIES.map((s) => (
              <Checkbox
                key={s}
                label={etiquetaSuperficie(d, s)}
                checked={f.superficies.includes(s)}
                onChange={(v) =>
                  setF({ ...f, superficies: v ? [...f.superficies, s] : f.superficies.filter((x) => x !== s) })
                }
              />
            ))}
          </div>
        </fieldset>
      )}
      {ubicaciones.data?.length > 0 && (
        <Select
          label="Descontar insumos de"
          value={f.ubicacionId}
          onChange={(e) => setF({ ...f, ubicacionId: e.target.value })}
        >
          <option value="">Automático (consultorio de la cita o bodega)</option>
          {ubicaciones.data
            .filter((u) => u.activo)
            .map((u) => (
              <option key={u.id} value={u.id}>
                {u.nombre}
              </option>
            ))}
        </Select>
      )}
      <Textarea
        label="Observaciones"
        rows={2}
        value={f.observaciones}
        onChange={(e) => setF({ ...f, observaciones: e.target.value })}
      />
      <div className="flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={alCancelar}>
          Cancelar
        </Button>
        <Button type="submit">Continuar a la firma</Button>
      </div>
    </form>
  );
};

const Procedimientos = ({ paciente }) => {
  const { puede } = useAuth();
  const [notacion] = useNotacion();
  const { data, isLoading, error, recargar } = useApi(useCallback(() => getRealizados(paciente.id), [paciente.id]));
  const [formulario, setFormulario] = useState(false);
  const [firma, setFirma] = useState(null); // { tipo: "registrar", datos } | { tipo: "adenda", id }
  const puedeFirmar = puede("clinica.firmar");

  if (isLoading && !data) return <Skeleton className="h-60" />;
  if (error) return <Alert tone="danger">{error}</Alert>;

  return (
    <div className="space-y-4">
      <Alert tone="info">
        <Lock className="mr-1 inline size-3.5" /> Un procedimiento realizado no se edita ni se borra (RN-01). Las
        correcciones se hacen con adendas firmadas.
      </Alert>
      {puedeFirmar && !paciente.anamnesisPendiente && !formulario && (
        <Button onClick={() => setFormulario(true)}>
          <Plus className="size-4" /> Registrar procedimiento realizado
        </Button>
      )}
      {formulario && (
        <Card className="p-4">
          <Formulario
            pacienteId={paciente.id}
            notacion={notacion}
            alCancelar={() => setFormulario(false)}
            alContinuar={(datos) => setFirma({ tipo: "registrar", datos })}
          />
        </Card>
      )}
      {data.length === 0 ? (
        <Card>
          <EmptyState title="Sin procedimientos realizados" />
        </Card>
      ) : (
        data.map((r) => (
          <Card key={r.id} className="p-4">
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div>
                <p className="font-medium">
                  {r.codigo} · {r.procedimiento}
                  {r.diente && (
                    <span className="font-normal text-muted">
                      {" "}
                      · diente {nombreDiente(r.diente, notacion)}
                      {r.superficies.length > 0 &&
                        ` (${r.superficies.map((s) => etiquetaSuperficie(r.diente, s)).join(", ")})`}
                    </span>
                  )}
                </p>
                <p className="text-xs text-muted">
                  {fmtFechaHora(r.realizadoEn)} · {r.odontologo} · firma{" "}
                  <code className="font-mono">{r.hashFirma.slice(0, 12)}…</code>
                </p>
                {r.observaciones && <p className="mt-1 whitespace-pre-wrap text-sm">{r.observaciones}</p>}
              </div>
              {puedeFirmar && (
                <Button variant="secondary" onClick={() => setFirma({ tipo: "adenda", id: r.id })}>
                  <Plus className="size-4" /> Adenda
                </Button>
              )}
            </div>
            <Adendas adendas={r.adendas} />
          </Card>
        ))
      )}
      {firma && (
        <ModalFirma
          titulo={firma.tipo === "registrar" ? "Firmar procedimiento realizado" : "Adenda al procedimiento"}
          descripcion={
            firma.tipo === "registrar"
              ? "Una vez firmado, el registro no podrá modificarse ni eliminarse."
              : "El registro original no cambia: la corrección se agrega como adenda firmada."
          }
          conTexto={firma.tipo === "adenda"}
          etiquetaTexto="Corrección o aclaración"
          alCerrar={() => setFirma(null)}
          alFirmar={async ({ codigo, texto }) => {
            if (firma.tipo === "registrar") {
              await registrarRealizado(paciente.id, { ...firma.datos, codigo });
              setFormulario(false);
            } else await adendaRealizado(firma.id, texto, codigo);
            setFirma(null);
            recargar();
          }}
        />
      )}
    </div>
  );
};

export default Procedimientos;

// Recetas (propuesta P-01 del análisis): se revisan contra las alergias y la anticoagulación de la
// anamnesis vigente; si hay coincidencias, el profesional debe confirmarlas antes de firmar con su
// código MFA. La receta emitida no se modifica.
import { AlertTriangle, Pill, Plus, Printer, Trash2 } from "lucide-react";
import { useCallback, useState } from "react";
import { Alert, Button, Card, Checkbox, EmptyState, Input, Skeleton, Textarea } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { crearReceta, getRecetas } from "../../services/planesService";
import { fmtFechaHora } from "../../utils/fechas";
import ModalFirma from "./ModalFirma";
import { imprimirReceta } from "../../utils/documentos";

const CAMPOS = [
  ["medicamento", "Medicamento (genérico)"],
  ["presentacion", "Presentación"],
  ["dosis", "Dosis"],
  ["via", "Vía"],
  ["frecuencia", "Frecuencia"],
  ["duracion", "Duración"],
];
const vacio = () => ({
  medicamento: "",
  presentacion: "",
  dosis: "",
  via: "Oral",
  frecuencia: "",
  duracion: "",
  indicaciones: "",
});

const Formulario = ({ paciente, alEmitir, alCancelar }) => {
  const [items, setItems] = useState([vacio()]);
  const [indicaciones, setIndicaciones] = useState("");
  const [alertas, setAlertas] = useState(null); // alertas devueltas por el servidor (409)
  const [confirmado, setConfirmado] = useState(false);
  const [firmar, setFirmar] = useState(false);
  const [error, setError] = useState("");
  const cambiar = (i, k, v) => {
    setItems(items.map((it, j) => (j === i ? { ...it, [k]: v } : it)));
    setAlertas(null);
    setConfirmado(false);
  };
  const cuerpo = (codigo) => ({
    items: items.map((it) => Object.fromEntries(Object.entries(it).map(([k, v]) => [k, v.trim()]))),
    indicaciones: indicaciones.trim(),
    confirmarAlertas: confirmado,
    codigo,
  });
  return (
    <Card className="space-y-4 p-4">
      {error && <Alert tone="danger">{error}</Alert>}
      {items.map((it, i) => (
        <div key={i} className="space-y-2 rounded-lg border border-line p-3">
          <div className="flex items-center justify-between">
            <p className="text-sm font-semibold">Medicamento {i + 1}</p>
            {items.length > 1 && (
              <Button
                variant="ghost"
                aria-label={`Quitar medicamento ${i + 1}`}
                onClick={() => setItems(items.filter((_, j) => j !== i))}
              >
                <Trash2 className="size-4" />
              </Button>
            )}
          </div>
          <div className="grid gap-2 sm:grid-cols-3">
            {CAMPOS.map(([k, l]) => (
              <Input key={k} label={l} value={it[k]} onChange={(e) => cambiar(i, k, e.target.value)} />
            ))}
          </div>
          <Input
            label="Indicaciones (opcional)"
            value={it.indicaciones}
            onChange={(e) => cambiar(i, "indicaciones", e.target.value)}
          />
        </div>
      ))}
      <Button variant="secondary" disabled={items.length >= 10} onClick={() => setItems([...items, vacio()])}>
        <Plus className="size-4" /> Agregar medicamento
      </Button>
      <Textarea
        label="Indicaciones generales (opcional)"
        rows={2}
        value={indicaciones}
        onChange={(e) => setIndicaciones(e.target.value)}
      />
      {alertas && (
        <Alert tone="danger" title="Alerta con la anamnesis del paciente">
          <ul className="list-disc pl-4">
            {alertas.map((a, i) => (
              <li key={i}>
                <strong>{a.medicamento}</strong>: {a.motivo}
              </li>
            ))}
          </ul>
          <div className="mt-2">
            <Checkbox
              label="Revisé las alertas y mantengo la prescripción bajo mi criterio clínico."
              checked={confirmado}
              onChange={setConfirmado}
            />
          </div>
        </Alert>
      )}
      <div className="flex justify-end gap-2">
        <Button variant="secondary" onClick={alCancelar}>
          Cancelar
        </Button>
        <Button
          disabled={alertas && !confirmado}
          onClick={() => {
            setError("");
            const falta = items.some((it) => CAMPOS.some(([k]) => !it[k].trim()));
            if (falta) return setError("Complete todos los datos de cada medicamento.");
            setFirmar(true);
          }}
        >
          Continuar a la firma
        </Button>
      </div>
      {firmar && (
        <ModalFirma
          titulo="Firmar receta"
          descripcion="La receta se emite con su firma electrónica simple y ya no podrá modificarse."
          alCerrar={() => setFirmar(false)}
          alFirmar={async ({ codigo }) => {
            try {
              alEmitir(await crearReceta(paciente.id, cuerpo(codigo)));
            } catch (e) {
              if (e.code === "ALERTA_ALERGIA") {
                setAlertas(e.extra?.alertas ?? []);
                setFirmar(false);
                return;
              }
              throw e;
            }
          }}
        />
      )}
    </Card>
  );
};

const Recetas = ({ paciente }) => {
  const { puede } = useAuth();
  const { data, isLoading, error, recargar } = useApi(useCallback(() => getRecetas(paciente.id), [paciente.id]));
  const [nueva, setNueva] = useState(false);
  const [aviso, setAviso] = useState("");
  if (isLoading && !data) return <Skeleton className="h-48" />;
  if (error) return <Alert tone="danger">{error}</Alert>;
  return (
    <div className="space-y-4">
      {aviso && <Alert tone="ok">{aviso}</Alert>}
      {puede("recetas.escribir") && !paciente.anamnesisPendiente && !nueva && (
        <Button onClick={() => setNueva(true)}>
          <Plus className="size-4" /> Nueva receta
        </Button>
      )}
      {nueva && (
        <Formulario
          paciente={paciente}
          alCancelar={() => setNueva(false)}
          alEmitir={() => {
            setNueva(false);
            setAviso("Receta emitida y firmada. Puede imprimirla desde la lista.");
            recargar();
          }}
        />
      )}
      {data.length === 0 ? (
        <Card>
          <EmptyState icon={<Pill className="size-8" />} title="Sin recetas emitidas" />
        </Card>
      ) : (
        data.map((r) => (
          <Card key={r.id} className="p-4">
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div className="text-sm">
                <p className="text-xs text-muted">
                  {fmtFechaHora(r.emitidaEn)} · {r.odontologo} · firma{" "}
                  <code className="font-mono">{r.hashFirma.slice(0, 12)}…</code>
                  {r.integra === false && <span className="ml-2 font-semibold text-danger-700">ALTERADA</span>}
                </p>
                <ul className="mt-1 space-y-0.5">
                  {r.items.map((it, i) => (
                    <li key={i}>
                      <strong>{it.medicamento}</strong> {it.presentacion} — {it.dosis}, {it.via}, {it.frecuencia},{" "}
                      {it.duracion}
                    </li>
                  ))}
                </ul>
                {r.alertasConfirmadas.length > 0 && (
                  <p className="mt-1 text-xs text-warn-700">
                    <AlertTriangle className="mr-1 inline size-3" />
                    Emitida con alertas confirmadas: {r.alertasConfirmadas.map((a) => a.medicamento).join(", ")}
                  </p>
                )}
              </div>
              <Button variant="secondary" onClick={() => imprimirReceta(r, paciente)}>
                <Printer className="size-4" /> Imprimir
              </Button>
            </div>
          </Card>
        ))
      )}
    </div>
  );
};

export default Recetas;

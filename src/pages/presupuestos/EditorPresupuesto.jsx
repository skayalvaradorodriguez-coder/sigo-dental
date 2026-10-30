// Editor de presupuesto (RF-07): ítems por fase y sesión, con diente y superficies opcionales.
// Los montos se escriben como texto ("45.00") y se suman en centavos; el backend recalcula todo.
import { Plus, Trash2 } from "lucide-react";
import { useCallback, useState } from "react";
import { Alert, Button, Card, Checkbox, Input, Select, Textarea } from "../../components/ui";
import { useApi } from "../../hooks/useApi";
import { useNotacion } from "../../hooks/useNotacion";
import { getProcedimientos } from "../../services/catalogosService";
import { mensajeDe } from "../../services/http";
import { actualizarPresupuesto, crearPresupuesto } from "../../services/planesService";
import {
  etiquetaSuperficie,
  nombreDiente,
  PERMANENTES_INF,
  PERMANENTES_SUP,
  SUPERFICIES,
  TEMPORALES_INF,
  TEMPORALES_SUP,
} from "../../utils/dental";
import { aCentavos, fmtDinero, totalesItems } from "../../utils/dinero";

const DIENTES = [...PERMANENTES_SUP, ...PERMANENTES_INF, ...TEMPORALES_SUP, ...TEMPORALES_INF];

const itemVacio = (anterior) => ({
  fase: anterior?.fase ?? 1,
  faseNombre: anterior?.faseNombre ?? "",
  sesion: anterior?.sesion ?? 1,
  procedimientoId: "",
  diente: "",
  superficies: [],
  honorario: "",
  costoLaboratorio: "",
  descuento: "",
});

const desdeExistente = (it) => ({
  fase: it.fase,
  faseNombre: it.faseNombre,
  sesion: it.sesion,
  procedimientoId: it.procedimientoId,
  diente: it.diente ? String(it.diente) : "",
  superficies: it.superficies ?? [],
  honorario: it.honorario,
  costoLaboratorio: it.costoLaboratorio === "0.00" ? "" : it.costoLaboratorio,
  descuento: it.descuento === "0.00" ? "" : it.descuento,
});

const EditorPresupuesto = ({ paciente, existente, alGuardar, alCancelar }) => {
  const [notacion] = useNotacion();
  const procs = useApi(useCallback(() => getProcedimientos(), []));
  const [titulo, setTitulo] = useState(existente?.titulo ?? "Plan de tratamiento");
  const [observaciones, setObservaciones] = useState(existente?.observaciones ?? "");
  const [items, setItems] = useState(existente ? existente.items.map(desdeExistente) : [itemVacio()]);
  const [error, setError] = useState("");
  const [campos, setCampos] = useState({});
  const [enviando, setEnviando] = useState(false);

  const cambiar = (i, cambios) => setItems(items.map((it, j) => (j === i ? { ...it, ...cambios } : it)));
  const tot = totalesItems(items);
  const montoInvalido = items.some(
    (it) =>
      aCentavos(it.honorario) === null || aCentavos(it.costoLaboratorio) === null || aCentavos(it.descuento) === null,
  );

  const guardar = async (e) => {
    e.preventDefault();
    setError("");
    setCampos({});
    if (montoInvalido) return setError("Revise los montos: use números con hasta 2 decimales (p. ej. 45.00).");
    const cuerpo = {
      titulo: titulo.trim(),
      observaciones: observaciones.trim(),
      items: items.map((it) => ({
        fase: Number(it.fase),
        faseNombre: it.faseNombre.trim(),
        sesion: Number(it.sesion),
        procedimientoId: it.procedimientoId,
        ...(it.diente ? { diente: Number(it.diente), superficies: it.superficies } : {}),
        honorario: it.honorario || "0",
        costoLaboratorio: it.costoLaboratorio || "0",
        descuento: it.descuento || "0",
      })),
    };
    setEnviando(true);
    try {
      const p = existente
        ? await actualizarPresupuesto(existente.id, cuerpo)
        : await crearPresupuesto(paciente.id, cuerpo);
      alGuardar(p);
    } catch (err) {
      setError(mensajeDe(err));
      setCampos(err.campos ?? {});
    } finally {
      setEnviando(false);
    }
  };

  const errItem = (i, campo) => campos[`items[${i}].${campo}`];

  return (
    <form onSubmit={guardar} noValidate className="space-y-4">
      {error && (
        <Alert tone="danger" title="No se pudo guardar">
          {error}
        </Alert>
      )}
      <Card className="space-y-3 p-4">
        <Input label="Título" value={titulo} onChange={(e) => setTitulo(e.target.value)} error={campos.titulo} />
        <Textarea
          label="Observaciones (opcional)"
          rows={2}
          value={observaciones}
          onChange={(e) => setObservaciones(e.target.value)}
        />
      </Card>

      {items.map((it, i) => {
        const d = Number(it.diente);
        return (
          <Card key={i} className="space-y-3 p-4">
            <div className="flex items-center justify-between">
              <p className="text-sm font-semibold">Procedimiento {i + 1}</p>
              {items.length > 1 && (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setItems(items.filter((_, j) => j !== i))}
                  aria-label={`Quitar procedimiento ${i + 1}`}
                >
                  <Trash2 className="size-4" />
                </Button>
              )}
            </div>
            <div className="grid gap-3 sm:grid-cols-4">
              <Input
                label="Fase"
                type="number"
                min={1}
                max={20}
                value={it.fase}
                onChange={(e) => cambiar(i, { fase: e.target.value })}
                error={errItem(i, "fase")}
              />
              <Input
                label="Nombre de la fase"
                placeholder={`Fase ${it.fase}`}
                value={it.faseNombre}
                onChange={(e) => cambiar(i, { faseNombre: e.target.value })}
                error={errItem(i, "faseNombre")}
              />
              <Input
                label="Sesión"
                type="number"
                min={1}
                max={50}
                value={it.sesion}
                onChange={(e) => cambiar(i, { sesion: e.target.value })}
                error={errItem(i, "sesion")}
              />
              <Select
                label="Diente (opcional)"
                value={it.diente}
                onChange={(e) => cambiar(i, { diente: e.target.value, superficies: [] })}
                error={errItem(i, "diente")}
              >
                <option value="">General</option>
                {DIENTES.map((x) => (
                  <option key={x} value={x}>
                    {nombreDiente(x, notacion)}
                  </option>
                ))}
              </Select>
              <Select
                label="Procedimiento"
                className="sm:col-span-4"
                value={it.procedimientoId}
                error={errItem(i, "procedimientoId")}
                onChange={(e) => {
                  const p = procs.data?.find((x) => x.id === e.target.value);
                  cambiar(i, { procedimientoId: e.target.value, honorario: it.honorario || p?.arancel || "" });
                }}
              >
                <option value="">Seleccione…</option>
                {procs.data?.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.codigo} · {p.nombre} (arancel {fmtDinero(p.arancel)})
                  </option>
                ))}
              </Select>
            </div>
            {it.diente && (
              <fieldset>
                <legend className="mb-1 text-sm font-medium">Superficies (opcional)</legend>
                <div className="flex flex-wrap gap-4">
                  {SUPERFICIES.map((s) => (
                    <Checkbox
                      key={s}
                      label={etiquetaSuperficie(d, s)}
                      checked={it.superficies.includes(s)}
                      onChange={(v) =>
                        cambiar(i, { superficies: v ? [...it.superficies, s] : it.superficies.filter((x) => x !== s) })
                      }
                    />
                  ))}
                </div>
              </fieldset>
            )}
            <div className="grid gap-3 sm:grid-cols-3">
              <Input
                label="Honorario"
                inputMode="decimal"
                value={it.honorario}
                onChange={(e) => cambiar(i, { honorario: e.target.value })}
                error={errItem(i, "honorario") ?? (aCentavos(it.honorario) === null ? "Monto inválido" : undefined)}
              />
              <Input
                label="Costo de laboratorio"
                inputMode="decimal"
                placeholder="0.00"
                value={it.costoLaboratorio}
                onChange={(e) => cambiar(i, { costoLaboratorio: e.target.value })}
                error={
                  errItem(i, "costoLaboratorio") ??
                  (aCentavos(it.costoLaboratorio) === null ? "Monto inválido" : undefined)
                }
              />
              <Input
                label="Descuento"
                inputMode="decimal"
                placeholder="0.00"
                value={it.descuento}
                onChange={(e) => cambiar(i, { descuento: e.target.value })}
                error={errItem(i, "descuento") ?? (aCentavos(it.descuento) === null ? "Monto inválido" : undefined)}
                hint="Sobre el tope configurado lo autoriza otro odontólogo."
              />
            </div>
          </Card>
        );
      })}

      <Button
        type="button"
        variant="secondary"
        onClick={() => setItems([...items, itemVacio(items[items.length - 1])])}
        disabled={items.length >= 60}
      >
        <Plus className="size-4" /> Agregar procedimiento
      </Button>

      <Card className="p-4">
        <dl className="grid gap-2 text-sm sm:grid-cols-4">
          {tot.fases.map((f) => (
            <div key={f.fase}>
              <dt className="text-xs text-muted">Fase {f.fase}</dt>
              <dd className="tabular-nums">{fmtDinero(f.total)}</dd>
            </div>
          ))}
        </dl>
        <p className="mt-3 border-t border-line pt-3 text-sm">
          Honorarios {fmtDinero(tot.honorarios)} + laboratorio {fmtDinero(tot.laboratorio)} − descuentos{" "}
          {fmtDinero(tot.descuentos)} = <strong className="tabular-nums">{fmtDinero(tot.total)}</strong>
        </p>
        <p className="mt-1 text-xs text-muted">Vista previa; el servidor recalcula los totales al guardar.</p>
      </Card>

      <div className="flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={alCancelar}>
          Cancelar
        </Button>
        <Button type="submit" loading={enviando}>
          Guardar borrador
        </Button>
      </div>
    </form>
  );
};

export default EditorPresupuesto;

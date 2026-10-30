// Cobro con pago mixto (RF-12). Los montos de los conceptos los fija el servidor (presupuesto
// aprobado o arancel); aquí sólo se elige qué se cobra y cómo se reparte entre los medios de pago.
import { Plus, Trash2 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import SelectorPaciente from "../../components/SelectorPaciente";
import { Alert, Button, Card, Checkbox, Input, Select, Skeleton } from "../../components/ui";
import { useApi } from "../../hooks/useApi";
import { crearCobro, getPorCobrar } from "../../services/cajaService";
import { getConvenios, getProcedimientos } from "../../services/catalogosService";
import { mensajeDe } from "../../services/http";
import { getPaciente } from "../../services/pacientesService";
import { aCentavos, deCentavos, fmtDinero } from "../../utils/dinero";
import { ETIQUETA_MEDIO, MEDIOS_PAGO } from "../../utils/etiquetas";
import { fmtFechaHora } from "../../utils/fechas";

const lineaVacia = (medio = "EFECTIVO", monto = "") => ({ medio, monto, referencia: "", convenioId: "", recibido: "" });

const FormularioCobro = ({ pacienteInicial, alCobrar }) => {
  const [paciente, setPaciente] = useState(null);
  const procs = useApi(useCallback(() => getProcedimientos(), []));
  const convenios = useApi(useCallback(() => getConvenios(), []));
  const pacienteId = paciente?.id;
  const pend = useApi(useCallback(() => (pacienteId ? getPorCobrar(pacienteId) : Promise.resolve([])), [pacienteId]));
  const [quitados, setQuitados] = useState({}); // realizados que NO se cobran ahora
  const [extras, setExtras] = useState([]); // procedimientos del catálogo (p. ej. consulta)
  const [extraSel, setExtraSel] = useState("");
  const [pagos, setPagos] = useState([lineaVacia()]);
  const [error, setError] = useState("");
  const [campos, setCampos] = useState({});
  const [enviando, setEnviando] = useState(false);

  useEffect(() => {
    if (!pacienteInicial) return;
    getPaciente(pacienteInicial)
      .then(setPaciente)
      .catch(() => {});
  }, [pacienteInicial]);

  const elegirPaciente = (p) => {
    setPaciente(p);
    setQuitados({});
    setExtras([]);
    setPagos([lineaVacia()]);
    setError("");
  };

  const seleccionados = (pend.data ?? []).filter((x) => !quitados[x.procedimientoRealizadoId]);
  const total =
    seleccionados.reduce((s, x) => s + aCentavos(x.monto), 0) +
    extras.reduce((s, id) => s + (aCentavos(procs.data?.find((p) => p.id === id)?.arancel) ?? 0), 0);
  const pagado = pagos.reduce((s, p) => s + (aCentavos(p.monto) ?? 0), 0);
  const saldo = total - pagado;
  const cambiar = (i, k, v) => setPagos(pagos.map((p, j) => (j === i ? { ...p, [k]: v } : p)));

  const cobrar = async () => {
    setError("");
    setCampos({});
    if (!paciente) return setError("Seleccione el paciente.");
    if (total <= 0) return setError("Seleccione al menos un concepto a cobrar.");
    if (pagos.some((p) => !aCentavos(p.monto))) return setError("Cada pago debe tener un monto mayor que cero.");
    if (saldo !== 0)
      return setError(
        saldo > 0 ? `Falta asignar ${fmtDinero(saldo)}.` : `Los pagos exceden el total en ${fmtDinero(-saldo)}.`,
      );
    setEnviando(true);
    try {
      const cb = await crearCobro({
        pacienteId: paciente.id,
        conceptos: [
          ...seleccionados.map((x) => ({ procedimientoRealizadoId: x.procedimientoRealizadoId })),
          ...extras.map((id) => ({ procedimientoId: id })),
        ],
        pagos: pagos.map((p) => ({
          medio: p.medio,
          monto: deCentavos(aCentavos(p.monto)),
          referencia: p.referencia.trim(),
          ...(p.medio === "SEGURO" ? { convenioId: p.convenioId } : {}),
        })),
      });
      elegirPaciente(null);
      alCobrar(cb);
    } catch (e) {
      setError(mensajeDe(e));
      setCampos(e.campos ?? {});
      if (e.code === "YA_COBRADO") pend.recargar();
    } finally {
      setEnviando(false);
    }
  };

  return (
    <Card className="space-y-4 p-4">
      <SelectorPaciente valor={paciente} alCambiar={elegirPaciente} />
      {paciente && (
        <>
          <div>
            <p className="mb-1 text-sm font-medium">Procedimientos realizados por cobrar</p>
            {pend.isLoading && !pend.data ? (
              <Skeleton className="h-16" />
            ) : pend.error ? (
              <Alert tone="danger">{pend.error}</Alert>
            ) : pend.data.length === 0 ? (
              <p className="text-sm text-muted">No tiene procedimientos realizados pendientes de cobro.</p>
            ) : (
              <ul className="divide-y divide-line rounded-lg border border-line">
                {pend.data.map((x) => (
                  <li key={x.procedimientoRealizadoId} className="flex flex-wrap items-center gap-3 px-3 py-2 text-sm">
                    <Checkbox
                      label={
                        <span>
                          <span className="font-medium">
                            {x.codigo} · {x.procedimiento}
                          </span>
                          {x.diente && <span className="text-muted"> · diente {x.diente}</span>}
                          <span className="block text-xs text-muted">
                            {fmtFechaHora(x.realizadoEn)} · {x.odontologo} ·{" "}
                            {x.origen === "PRESUPUESTO" ? "precio del presupuesto aprobado" : "arancel"}
                          </span>
                        </span>
                      }
                      checked={!quitados[x.procedimientoRealizadoId]}
                      onChange={(v) => setQuitados({ ...quitados, [x.procedimientoRealizadoId]: !v })}
                    />
                    <span className="ml-auto tabular-nums">{fmtDinero(x.monto)}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className="flex flex-wrap items-end gap-2">
            <Select
              label="Agregar del catálogo (p. ej. consulta)"
              value={extraSel}
              onChange={(e) => setExtraSel(e.target.value)}
            >
              <option value="">Seleccione…</option>
              {procs.data?.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.codigo} · {p.nombre} ({fmtDinero(p.arancel)})
                </option>
              ))}
            </Select>
            <Button
              variant="secondary"
              disabled={!extraSel}
              onClick={() => {
                setExtras([...extras, extraSel]);
                setExtraSel("");
              }}
            >
              <Plus className="size-4" /> Agregar
            </Button>
          </div>
          {extras.length > 0 && (
            <ul className="space-y-1 text-sm">
              {extras.map((id, i) => {
                const p = procs.data?.find((x) => x.id === id);
                return (
                  <li key={i} className="flex items-center gap-2">
                    <span>
                      {p?.codigo} · {p?.nombre}
                    </span>
                    <span className="ml-auto tabular-nums">{fmtDinero(p?.arancel)}</span>
                    <Button
                      variant="ghost"
                      aria-label={`Quitar ${p?.nombre}`}
                      onClick={() => setExtras(extras.filter((_, j) => j !== i))}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </li>
                );
              })}
            </ul>
          )}
          <p className="border-t border-line pt-3 text-right text-lg">
            Total a cobrar: <strong className="tabular-nums">{fmtDinero(total)}</strong>
          </p>

          <fieldset className="space-y-3">
            <legend className="mb-1 text-sm font-medium">Formas de pago</legend>
            {pagos.map((p, i) => {
              const cambio =
                p.medio === "EFECTIVO" && aCentavos(p.recibido) > aCentavos(p.monto)
                  ? aCentavos(p.recibido) - aCentavos(p.monto)
                  : 0;
              return (
                <div
                  key={i}
                  className="grid gap-2 rounded-lg border border-line p-3 sm:grid-cols-[10rem_8rem_1fr_auto]"
                >
                  <Select
                    label="Medio"
                    value={p.medio}
                    onChange={(e) => cambiar(i, "medio", e.target.value)}
                    error={campos[`pagos[${i}].medio`]}
                  >
                    {MEDIOS_PAGO.map((m) => (
                      <option key={m} value={m}>
                        {ETIQUETA_MEDIO[m]}
                      </option>
                    ))}
                  </Select>
                  <Input
                    label="Monto"
                    inputMode="decimal"
                    value={p.monto}
                    onChange={(e) => cambiar(i, "monto", e.target.value)}
                    error={campos[`pagos[${i}].monto`]}
                  />
                  {p.medio === "EFECTIVO" && (
                    <Input
                      label="Recibido (para calcular el cambio)"
                      inputMode="decimal"
                      value={p.recibido}
                      onChange={(e) => cambiar(i, "recibido", e.target.value)}
                      hint={cambio > 0 ? `Cambio: ${fmtDinero(cambio)}` : undefined}
                    />
                  )}
                  {(p.medio === "TARJETA" || p.medio === "TRANSFERENCIA") && (
                    <Input
                      label={p.medio === "TARJETA" ? "N.º de voucher / aprobación" : "N.º de transferencia"}
                      value={p.referencia}
                      onChange={(e) => cambiar(i, "referencia", e.target.value)}
                      error={campos[`pagos[${i}].referencia`]}
                    />
                  )}
                  {p.medio === "SEGURO" && (
                    <Select
                      label="Convenio (cuenta por cobrar)"
                      value={p.convenioId}
                      onChange={(e) => cambiar(i, "convenioId", e.target.value)}
                      error={campos[`pagos[${i}].convenioId`]}
                    >
                      <option value="">Seleccione…</option>
                      {convenios.data?.map((c) => (
                        <option key={c.id} value={c.id}>
                          {c.nombre}
                        </option>
                      ))}
                    </Select>
                  )}
                  <div className="flex items-end gap-1">
                    {saldo > 0 && (
                      <Button
                        variant="ghost"
                        onClick={() => cambiar(i, "monto", deCentavos((aCentavos(p.monto) ?? 0) + saldo))}
                      >
                        + saldo
                      </Button>
                    )}
                    {pagos.length > 1 && (
                      <Button
                        variant="ghost"
                        aria-label="Quitar pago"
                        onClick={() => setPagos(pagos.filter((_, j) => j !== i))}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    )}
                  </div>
                </div>
              );
            })}
            <div className="flex flex-wrap items-center justify-between gap-2">
              <Button
                variant="secondary"
                disabled={pagos.length >= 10}
                onClick={() => setPagos([...pagos, lineaVacia("TARJETA", saldo > 0 ? deCentavos(saldo) : "")])}
              >
                <Plus className="size-4" /> Otro medio de pago
              </Button>
              <p className={`text-sm ${saldo === 0 ? "text-ok-700" : "text-danger-700"}`}>
                {saldo === 0
                  ? "Pagos completos"
                  : saldo > 0
                    ? `Falta ${fmtDinero(saldo)}`
                    : `Sobra ${fmtDinero(-saldo)}`}
              </p>
            </div>
          </fieldset>
          {error && <Alert tone="danger">{error}</Alert>}
          <div className="flex justify-end">
            <Button onClick={cobrar} loading={enviando} disabled={total <= 0}>
              Cobrar {fmtDinero(total)}
            </Button>
          </div>
        </>
      )}
    </Card>
  );
};

export default FormularioCobro;

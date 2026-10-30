// Auditor (caja.revisar): conciliación de cierres ciegos — teórico, declarado y diferencia por medio —
// y cuentas por cobrar a aseguradoras.
import { ArrowLeft, CheckCircle2, ShieldAlert } from "lucide-react";
import { useCallback, useState } from "react";
import { Alert, Badge, Button, Card, EmptyState, Skeleton, Tabs, Textarea } from "../../components/ui";
import { useApi } from "../../hooks/useApi";
import { getCierre, getCierres, getCuentasPorCobrar, revisarCierre } from "../../services/cajaService";
import { mensajeDe } from "../../services/http";
import { fmtDinero } from "../../utils/dinero";
import { ETIQUETA_CIERRE, ETIQUETA_MEDIO } from "../../utils/etiquetas";
import { fmtFechaHora } from "../../utils/fechas";
import ListaCobros from "./ListaCobros";

const Cuadre = ({ c }) =>
  c.cuadra === null || c.cuadra === undefined ? (
    <Badge tone="neutral">{ETIQUETA_CIERRE[c.estado]}</Badge>
  ) : c.cuadra ? (
    <Badge tone="ok">Cuadra</Badge>
  ) : (
    <Badge tone="danger">Con diferencias</Badge>
  );

const DetalleCierre = ({ id, alVolver }) => {
  const { data, error, recargar } = useApi(useCallback(() => getCierre(id), [id]));
  const [nota, setNota] = useState("");
  const [errorRev, setErrorRev] = useState("");
  const [enviando, setEnviando] = useState(false);
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data) return <Skeleton className="h-60" />;
  const c = data.cierre;
  return (
    <div className="space-y-4">
      <Button variant="ghost" onClick={alVolver}>
        <ArrowLeft className="size-4" /> Cierres
      </Button>
      <Card className="p-4">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div>
            <p className="text-lg font-semibold">Caja de {c.cajero}</p>
            <p className="text-sm text-muted">
              Abierta {fmtFechaHora(c.abiertaEn)} · cerrada {fmtFechaHora(c.cerradaEn)} · fondo inicial{" "}
              {fmtDinero(c.fondoInicial)} · {c.cobros} cobro(s), {c.anulados} anulado(s) · seguro {fmtDinero(c.seguro)}
            </p>
            {c.notaCierre && <p className="mt-1 text-sm">Nota de la cajera: {c.notaCierre}</p>}
          </div>
          <Cuadre c={c} />
        </div>
        {c.diferencias.length > 0 && (
          <table className="mt-4 w-full text-sm">
            <thead>
              <tr className="border-b border-line text-left text-xs text-muted">
                <th className="py-1">Medio</th>
                <th className="py-1 text-right">Teórico (sistema)</th>
                <th className="py-1 text-right">Declarado (contado)</th>
                <th className="py-1 text-right">Diferencia</th>
              </tr>
            </thead>
            <tbody>
              {c.diferencias.map((d) => (
                <tr key={d.medio} className="border-b border-line">
                  <td className="py-1.5">{ETIQUETA_MEDIO[d.medio]}</td>
                  <td className="py-1.5 text-right tabular-nums">{fmtDinero(d.teorico)}</td>
                  <td className="py-1.5 text-right tabular-nums">{fmtDinero(d.declarado)}</td>
                  <td
                    className={`py-1.5 text-right tabular-nums font-medium ${d.cuadra ? "text-ok-700" : "text-danger-700"}`}
                  >
                    {d.diferencia.startsWith("-")
                      ? `−${fmtDinero(d.diferencia.slice(1))} (faltante)`
                      : d.diferencia === "0.00"
                        ? "0,00"
                        : `+${fmtDinero(d.diferencia)} (sobrante)`}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <p className="mt-2 text-xs text-muted">
          El efectivo teórico incluye el fondo inicial. Los cobros anulados no cuentan.
        </p>
        {c.estado === "REVISADA" && (
          <p className="mt-3 text-sm">
            <CheckCircle2 className="mr-1 inline size-4 text-ok-700" />
            Revisada por {c.revisadaPor} el {fmtFechaHora(c.revisadaEn)}
            {c.notaRevision && <>: {c.notaRevision}</>}
          </p>
        )}
      </Card>
      {c.estado === "CERRADA" && (
        <Card className="space-y-3 p-4">
          {errorRev && <Alert tone="danger">{errorRev}</Alert>}
          <Textarea
            label={c.cuadra ? "Nota de revisión (opcional)" : "Cómo se resolvió la diferencia (obligatorio)"}
            rows={2}
            value={nota}
            onChange={(e) => setNota(e.target.value)}
          />
          <div className="flex justify-end">
            <Button
              loading={enviando}
              onClick={async () => {
                setEnviando(true);
                setErrorRev("");
                try {
                  await revisarCierre(c.id, nota.trim());
                  recargar();
                } catch (e) {
                  setErrorRev(e.campos?.nota ?? mensajeDe(e));
                } finally {
                  setEnviando(false);
                }
              }}
            >
              <CheckCircle2 className="size-4" /> Marcar como revisada
            </Button>
          </div>
        </Card>
      )}
      <ListaCobros cobros={data.cobros} puedeAnular={false} />
    </div>
  );
};

const CuentasSeguros = () => {
  const { data, error } = useApi(useCallback(() => getCuentasPorCobrar(), []));
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data) return <Skeleton className="h-32" />;
  if (!data.length) return <EmptyState title="No hay pagos con seguro pendientes de cobrar" />;
  return (
    <ul className="divide-y divide-line">
      {data.map((x) => (
        <li key={x.convenioId} className="flex items-center gap-4 px-4 py-3 text-sm">
          <span className="flex-1">
            <span className="font-medium">{x.convenio}</span>
            <span className="block text-xs text-muted">
              {x.aseguradora ?? "Sin aseguradora"} · {x.cobros} cobro(s)
            </span>
          </span>
          <span className="tabular-nums font-medium">{fmtDinero(x.total)}</span>
        </li>
      ))}
    </ul>
  );
};

const ListaCierres = ({ estado, alAbrir }) => {
  const { data, error } = useApi(useCallback(() => getCierres(estado), [estado]));
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data) return <Skeleton className="m-4 h-40" />;
  if (!data.length) return <EmptyState title="No hay cajas en este estado" />;
  return (
    <ul className="divide-y divide-line">
      {data.map((c) => (
        <li key={c.id}>
          <button
            onClick={() => alAbrir(c.id)}
            className="flex w-full flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 text-left text-sm hover:bg-canvas"
          >
            <span className="min-w-48 flex-1">
              <span className="font-medium">{c.cajero}</span>
              <span className="block text-xs text-muted">
                {fmtFechaHora(c.abiertaEn)} → {c.cerradaEn ? fmtFechaHora(c.cerradaEn) : "abierta"} · {c.cobros}{" "}
                cobro(s)
              </span>
            </span>
            {c.diferencias
              .filter((d) => !d.cuadra)
              .map((d) => (
                <span key={d.medio} className="text-xs text-danger-700">
                  <ShieldAlert className="mr-0.5 inline size-3" />
                  {ETIQUETA_MEDIO[d.medio]} {fmtDinero(d.diferencia.replace("-", ""))}{" "}
                  {d.diferencia.startsWith("-") ? "faltante" : "sobrante"}
                </span>
              ))}
            <Cuadre c={c} />
          </button>
        </li>
      ))}
    </ul>
  );
};

const Cierres = () => {
  const [pestana, setPestana] = useState("CERRADA");
  const [abierto, setAbierto] = useState(null);
  if (abierto) return <DetalleCierre id={abierto} alVolver={() => setAbierto(null)} />;
  return (
    <>
      <Tabs
        value={pestana}
        onChange={setPestana}
        tabs={[
          { id: "CERRADA", label: "Por revisar" },
          { id: "REVISADA", label: "Revisadas" },
          { id: "ABIERTA", label: "Abiertas ahora" },
          { id: "seguros", label: "Cuentas por cobrar (seguros)" },
        ]}
      />
      <Card className="mt-4">
        {pestana === "seguros" ? <CuentasSeguros /> : <ListaCierres estado={pestana} alAbrir={setAbierto} />}
      </Card>
    </>
  );
};

export default Cierres;

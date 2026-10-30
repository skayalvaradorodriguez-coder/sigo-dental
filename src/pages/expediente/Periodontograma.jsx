// Periodontograma (RF-04): profundidad de sondaje (PS) y margen gingival (MG) en 6 sitios por diente,
// sangrado, movilidad y furca. NIC = PS + MG (MG positivo = recesión). El NIC lo calcula la base;
// aquí sólo se muestra como vista previa. Cada examen se guarda completo y no se edita.
import { Plus } from "lucide-react";
import { useCallback, useState } from "react";
import { Alert, Badge, Button, Card, EmptyState, Skeleton, Textarea } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { useNotacion } from "../../hooks/useNotacion";
import { crearPeriodontograma, getPeriodontogramas } from "../../services/expedienteService";
import { mensajeDe } from "../../services/http";
import { cx } from "../../utils/cx";
import {
  esSuperior,
  nic,
  nombreDiente,
  PERMANENTES_INF,
  PERMANENTES_SUP,
  SITIOS_L,
  SITIOS_V,
} from "../../utils/dental";
import { fmtFechaHora } from "../../utils/fechas";

const vacio = () => ({ ausente: false, movilidad: "", furca: "", sitios: {} });
const num = (v) => (v === "" || v === undefined ? null : Number(v));

const Celda = ({ valor, onChange, min, max, etiqueta }) => (
  <input
    aria-label={etiqueta}
    inputMode="numeric"
    value={valor ?? ""}
    onChange={(e) => {
      const v = e.target.value.replace(/[^\d-]/g, "");
      if (v === "" || v === "-" || (Number(v) >= min && Number(v) <= max)) onChange(v);
    }}
    className="h-7 w-full min-w-0 rounded border border-line text-center text-xs tabular-nums"
  />
);

const Captura = ({ notacion, alGuardar, alCancelar }) => {
  const [arcada, setArcada] = useState("SUP");
  const [datos, setDatos] = useState({});
  const [obs, setObs] = useState("");
  const [error, setError] = useState("");
  const [enviando, setEnviando] = useState(false);
  const dientes = arcada === "SUP" ? PERMANENTES_SUP : PERMANENTES_INF;
  const d = (x) => datos[x] ?? vacio();
  const poner = (x, cambio) => setDatos({ ...datos, [x]: { ...d(x), ...cambio } });
  const ponerSitio = (x, s, campo, v) =>
    poner(x, { sitios: { ...d(x).sitios, [s]: { ...d(x).sitios[s], [campo]: v } } });

  const armar = () => {
    const out = [];
    for (const [diente, v] of Object.entries(datos)) {
      const x = Number(diente);
      const tieneDatos =
        Object.values(v.sitios).some((s) => s.ps !== undefined && s.ps !== "") || v.movilidad !== "" || v.furca !== "";
      if (v.ausente) {
        out.push({ diente: x, ausente: true });
        continue;
      }
      if (!tieneDatos) continue; // diente sin medir: no se incluye
      const sitios = [...SITIOS_V, ...SITIOS_L].map((s) => ({
        sitio: s,
        profundidad: num(v.sitios[s]?.ps),
        margen: num(v.sitios[s]?.mg) ?? 0,
        sangrado: !!v.sitios[s]?.sang,
        supuracion: false,
      }));
      if (sitios.some((s) => s.profundidad === null))
        throw new Error(`Diente ${nombreDiente(x, notacion)}: complete la profundidad de los 6 sitios.`);
      out.push({ diente: x, ausente: false, movilidad: num(v.movilidad), furca: num(v.furca), sitios });
    }
    if (out.length === 0) throw new Error("Registre al menos un diente.");
    return out;
  };

  const enviar = async () => {
    setError("");
    let dientesOut;
    try {
      dientesOut = armar();
    } catch (e) {
      return setError(e.message);
    }
    setEnviando(true);
    try {
      await alGuardar({ observaciones: obs.trim(), dientes: dientesOut });
    } catch (e) {
      setError(mensajeDe(e));
    } finally {
      setEnviando(false);
    }
  };

  const filaSitios = (sitios, titulo) => (
    <>
      <tr>
        <th className="whitespace-nowrap px-1 text-left font-medium text-muted" colSpan={dientes.length + 1}>
          {titulo}
        </th>
      </tr>
      {[
        ["ps", "PS (mm)", 0, 15],
        ["mg", "MG (mm)", -10, 15],
      ].map(([campo, et, min, max]) => (
        <tr key={campo}>
          <td className="whitespace-nowrap px-1 text-muted">{et}</td>
          {dientes.map((x) => (
            <td key={x} className="px-0.5">
              <div className="grid grid-cols-3 gap-px">
                {sitios.map((s) => (
                  <Celda
                    key={s}
                    etiqueta={`${campo} ${x} ${s}`}
                    valor={d(x).sitios[s]?.[campo]}
                    min={min}
                    max={max}
                    onChange={(v) => ponerSitio(x, s, campo, v)}
                  />
                ))}
              </div>
            </td>
          ))}
        </tr>
      ))}
      <tr>
        <td className="whitespace-nowrap px-1 text-muted">NIC</td>
        {dientes.map((x) => (
          <td key={x} className="px-0.5">
            <div className="grid grid-cols-3 gap-px text-center text-xs tabular-nums">
              {sitios.map((s) => {
                const ps = num(d(x).sitios[s]?.ps);
                return (
                  <span
                    key={s}
                    className={cx(
                      ps !== null && nic(ps, num(d(x).sitios[s]?.mg) ?? 0) >= 5 && "font-bold text-danger-700",
                    )}
                  >
                    {ps === null ? "·" : nic(ps, num(d(x).sitios[s]?.mg) ?? 0)}
                  </span>
                );
              })}
            </div>
          </td>
        ))}
      </tr>
      <tr>
        <td className="whitespace-nowrap px-1 text-muted">Sangrado</td>
        {dientes.map((x) => (
          <td key={x} className="px-0.5">
            <div className="grid grid-cols-3 gap-px">
              {sitios.map((s) => (
                <input
                  key={s}
                  type="checkbox"
                  aria-label={`sangrado ${x} ${s}`}
                  checked={!!d(x).sitios[s]?.sang}
                  onChange={(e) => ponerSitio(x, s, "sang", e.target.checked)}
                  className="mx-auto size-3.5 accent-danger-600"
                />
              ))}
            </div>
          </td>
        ))}
      </tr>
    </>
  );

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        <div role="group" aria-label="Arcada" className="flex rounded-lg border border-line p-0.5">
          {[
            ["SUP", "Arcada superior"],
            ["INF", "Arcada inferior"],
          ].map(([v, t]) => (
            <button
              key={v}
              onClick={() => setArcada(v)}
              aria-pressed={arcada === v}
              className={cx(
                "min-h-9 rounded-md px-3 text-sm",
                arcada === v ? "bg-brand-700 text-white" : "hover:bg-canvas",
              )}
            >
              {t}
            </button>
          ))}
        </div>
        <p className="text-xs text-muted">
          Sitios por diente: mesial · centro · distal. MG positivo = recesión (margen apical a la LAC); negativo =
          margen coronal. Los dientes sin datos no se incluyen.
        </p>
      </div>
      {error && <Alert tone="danger">{error}</Alert>}
      <div className="overflow-x-auto rounded-lg border border-line">
        <table className="w-full min-w-[1100px] text-xs">
          <thead>
            <tr className="border-b border-line bg-canvas">
              <th className="w-20" />
              {dientes.map((x) => (
                <th key={x} className="py-1 font-semibold">
                  {nombreDiente(x, notacion)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            <tr>
              <td className="px-1 text-muted">Ausente</td>
              {dientes.map((x) => (
                <td key={x} className="text-center">
                  <input
                    type="checkbox"
                    aria-label={`ausente ${x}`}
                    checked={d(x).ausente}
                    onChange={(e) => poner(x, { ausente: e.target.checked })}
                    className="size-3.5"
                  />
                </td>
              ))}
            </tr>
            {[
              ["movilidad", "Movilidad", 3],
              ["furca", "Furca", 3],
            ].map(([campo, et, max]) => (
              <tr key={campo}>
                <td className="px-1 text-muted">{et}</td>
                {dientes.map((x) => (
                  <td key={x} className="px-0.5">
                    <select
                      aria-label={`${campo} ${x}`}
                      disabled={d(x).ausente}
                      value={d(x)[campo]}
                      onChange={(e) => poner(x, { [campo]: e.target.value })}
                      className="h-7 w-full rounded border border-line text-xs"
                    >
                      <option value="">—</option>
                      {Array.from({ length: max + 1 }, (_, i) => (
                        <option key={i} value={i}>
                          {i}
                        </option>
                      ))}
                    </select>
                  </td>
                ))}
              </tr>
            ))}
            {filaSitios(SITIOS_V, "Vestibular")}
            {filaSitios(SITIOS_L, esSuperior(dientes[0]) ? "Palatino" : "Lingual")}
          </tbody>
        </table>
      </div>
      <Textarea label="Observaciones" rows={2} value={obs} onChange={(e) => setObs(e.target.value)} />
      <div className="flex justify-end gap-2">
        <Button variant="secondary" onClick={alCancelar}>
          Cancelar
        </Button>
        <Button loading={enviando} onClick={enviar}>
          Guardar examen
        </Button>
      </div>
    </div>
  );
};

const Examen = ({ p, notacion }) => {
  const [abierto, setAbierto] = useState(false);
  return (
    <Card className="p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p className="font-medium">{fmtFechaHora(p.registradoEn)}</p>
          <p className="text-xs text-muted">{p.registradoPor}</p>
        </div>
        <div className="flex flex-wrap gap-2 text-xs">
          <Badge>
            {p.dientes.length} dientes · {p.resumen.sitios} sitios
          </Badge>
          <Badge tone={p.resumen.porcentajeSangrado > 25 ? "warn" : "neutral"}>
            Sangrado {p.resumen.porcentajeSangrado} %
          </Badge>
          <Badge tone={p.resumen.bolsasDe4oMas > 0 ? "warn" : "neutral"}>
            Bolsas ≥ 4 mm: {p.resumen.bolsasDe4oMas}
          </Badge>
          <Badge tone={p.resumen.nicMaximo >= 5 ? "danger" : "neutral"}>NIC máx. {p.resumen.nicMaximo} mm</Badge>
        </div>
      </div>
      {p.observaciones && <p className="mt-2 text-sm">{p.observaciones}</p>}
      <button onClick={() => setAbierto(!abierto)} className="mt-2 text-sm text-brand-700 hover:underline">
        {abierto ? "Ocultar detalle" : "Ver detalle"}
      </button>
      {abierto && (
        <div className="mt-2 overflow-x-auto">
          <table className="w-full text-xs">
            <thead className="text-left text-muted">
              <tr>
                <th className="px-2 py-1">Diente</th>
                <th className="px-2 py-1">Mov.</th>
                <th className="px-2 py-1">Furca</th>
                <th className="px-2 py-1">PS (MV V DV / ML L DL)</th>
                <th className="px-2 py-1">NIC</th>
                <th className="px-2 py-1">Sangrado</th>
              </tr>
            </thead>
            <tbody>
              {p.dientes.map((d) => (
                <tr key={d.diente} className="border-t border-line">
                  <td className="px-2 py-1 font-medium">{nombreDiente(d.diente, notacion)}</td>
                  {d.ausente ? (
                    <td colSpan={5} className="px-2 py-1 text-muted">
                      Ausente
                    </td>
                  ) : (
                    <>
                      <td className="px-2 py-1">{d.movilidad ?? "—"}</td>
                      <td className="px-2 py-1">{d.furca ?? "—"}</td>
                      <td className="px-2 py-1 tabular-nums">
                        {d.sitios.map((s) => (
                          <span
                            key={s.sitio}
                            className={cx("mr-1.5", s.profundidad >= 4 && "font-bold text-danger-700")}
                          >
                            {s.profundidad}
                          </span>
                        ))}
                      </td>
                      <td className="px-2 py-1 tabular-nums">{d.sitios.map((s) => s.nic).join(" ")}</td>
                      <td className="px-2 py-1">{d.sitios.filter((s) => s.sangrado).length} / 6</td>
                    </>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
};

const Periodontograma = ({ paciente }) => {
  const { puede } = useAuth();
  const [notacion] = useNotacion();
  const { data, isLoading, error, recargar } = useApi(
    useCallback(() => getPeriodontogramas(paciente.id), [paciente.id]),
  );
  const [capturando, setCapturando] = useState(false);
  const puedeEscribir = puede("clinica.escribir") && !paciente.anamnesisPendiente;

  if (isLoading && !data) return <Skeleton className="h-60" />;
  if (error) return <Alert tone="danger">{error}</Alert>;

  return (
    <div className="space-y-4">
      {puedeEscribir && !capturando && (
        <Button onClick={() => setCapturando(true)}>
          <Plus className="size-4" /> Nuevo periodontograma
        </Button>
      )}
      {capturando && (
        <Card className="p-4">
          <Captura
            notacion={notacion}
            alCancelar={() => setCapturando(false)}
            alGuardar={async (examen) => {
              await crearPeriodontograma(paciente.id, examen);
              setCapturando(false);
              recargar();
            }}
          />
        </Card>
      )}
      {data.length === 0 && !capturando ? (
        <Card>
          <EmptyState title="Sin periodontogramas registrados" />
        </Card>
      ) : (
        data.map((p) => <Examen key={p.id} p={p} notacion={notacion} />)
      )}
    </div>
  );
};

export default Periodontograma;

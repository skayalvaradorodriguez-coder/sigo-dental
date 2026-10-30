// Módulo "Honorarios" (RF-13). Base: lo cobrado por cada procedimiento realizado. Tres tipos de regla:
// % de lo cobrado, monto fijo y % del neto (cobrado − laboratorio − materiales del inventario).
// El odontólogo ve sus honorarios; el Auditor configura reglas y cierra liquidaciones.
import { ArrowLeft, CheckCircle2, Plus, Printer } from "lucide-react";
import { useCallback, useState } from "react";
import { Alert, Button, Card, EmptyState, Input, Modal, PageHeader, Select, Skeleton, Tabs } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { getOdontologos, getProcedimientos } from "../../services/catalogosService";
import { mensajeDe } from "../../services/http";
import {
  crearLiquidacion,
  crearRegla,
  getCalculo,
  getLiquidacion,
  getLiquidaciones,
  getReglas,
} from "../../services/inventarioService";
import { fmtDinero } from "../../utils/dinero";
import { ETIQUETA_REGLA, TIPOS_REGLA } from "../../utils/etiquetas";
import { fechaISO, fmtFechaHora } from "../../utils/fechas";
import { escapar, imprimir } from "../../utils/imprimir";

const inicioMes = () => {
  const d = new Date();
  return fechaISO(new Date(d.getFullYear(), d.getMonth(), 1));
};

const TablaLineas = ({ lineas }) => (
  <div className="overflow-x-auto">
    <table className="w-full min-w-[44rem] text-sm">
      <thead>
        <tr className="border-b border-line text-left text-xs text-muted">
          <th className="px-3 py-2">Cobrado</th>
          <th className="px-3 py-2">Procedimiento</th>
          <th className="px-3 py-2 text-right">Cobrado</th>
          <th className="px-3 py-2 text-right">Laboratorio</th>
          <th className="px-3 py-2 text-right">Materiales</th>
          <th className="px-3 py-2">Regla</th>
          <th className="px-3 py-2 text-right">Honorario</th>
        </tr>
      </thead>
      <tbody>
        {lineas.map((l) => (
          <tr key={l.cobroConceptoId} className="border-b border-line">
            <td className="px-3 py-2 text-xs text-muted">
              {fmtFechaHora(l.cobradoEn)}
              <br />
              recibo N.º {l.recibo}
            </td>
            <td className="px-3 py-2">{l.descripcion}</td>
            <td className="px-3 py-2 text-right tabular-nums">{fmtDinero(l.monto)}</td>
            <td className="px-3 py-2 text-right tabular-nums">{fmtDinero(l.laboratorio)}</td>
            <td className="px-3 py-2 text-right tabular-nums">{fmtDinero(l.materiales)}</td>
            <td className={`px-3 py-2 text-xs ${l.reglaId ? "" : "text-danger-700"}`}>{l.regla}</td>
            <td className="px-3 py-2 text-right font-medium tabular-nums">{fmtDinero(l.honorario)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  </div>
);

const imprimirLiquidacion = (x) =>
  imprimir(
    `Liquidación de honorarios ${x.odontologo}`,
    `<h1>Liquidación de honorarios</h1><p class="muted">${escapar(x.odontologo)} · del ${escapar(x.desde)} al ${escapar(x.hasta)} · cerrada ${escapar(fmtFechaHora(x.creadoEn))} por ${escapar(x.creadoPor)}</p>` +
      `<table><thead><tr><th>Recibo</th><th>Procedimiento</th><th class="num">Cobrado</th><th class="num">Lab.</th><th class="num">Materiales</th><th>Regla</th><th class="num">Honorario</th></tr></thead><tbody>` +
      x.lineas
        .map(
          (l) =>
            `<tr><td>${l.recibo}</td><td>${escapar(l.descripcion)}</td><td class="num">${fmtDinero(l.monto)}</td><td class="num">${fmtDinero(l.laboratorio)}</td><td class="num">${fmtDinero(l.materiales)}</td><td>${escapar(l.regla)}</td><td class="num">${fmtDinero(l.honorario)}</td></tr>`,
        )
        .join("") +
      `</tbody></table><h2>Total: ${fmtDinero(x.total)}</h2><p class="pie">Base: lo cobrado (cobros no anulados). Materiales: costo de los insumos descargados del inventario.</p>`,
  );

const Periodo = ({ desde, hasta, setDesde, setHasta, children }) => (
  <div className="flex flex-wrap items-end gap-3">
    <Input label="Desde" type="date" value={desde} onChange={(e) => setDesde(e.target.value)} />
    <Input label="Hasta" type="date" value={hasta} onChange={(e) => setHasta(e.target.value)} />
    {children}
  </div>
);

const Calculo = ({ odontologoId, desde, hasta, alLiquidar }) => {
  const { data, error } = useApi(
    useCallback(() => getCalculo({ odontologoId, desde, hasta }), [odontologoId, desde, hasta]),
  );
  const [errLiq, setErrLiq] = useState("");
  const [enviando, setEnviando] = useState(false);
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data) return <Skeleton className="h-40" />;
  return (
    <Card className="space-y-3 p-4">
      {data.lineas.length === 0 ? (
        <EmptyState title="No hay procedimientos cobrados pendientes de liquidar en el periodo" />
      ) : (
        <>
          <TablaLineas lineas={data.lineas} />
          <p className="text-right text-lg">
            Total pendiente: <strong className="tabular-nums">{fmtDinero(data.total)}</strong>
          </p>
          {data.sinRegla > 0 && (
            <Alert tone="warn">{data.sinRegla} procedimiento(s) sin regla vigente: configure una regla general.</Alert>
          )}
        </>
      )}
      {errLiq && <Alert tone="danger">{errLiq}</Alert>}
      {alLiquidar && data.lineas.length > 0 && (
        <div className="flex justify-end">
          <Button
            loading={enviando}
            disabled={data.sinRegla > 0}
            onClick={async () => {
              setEnviando(true);
              setErrLiq("");
              try {
                alLiquidar(await crearLiquidacion({ odontologoId, desde, hasta }));
              } catch (e) {
                setErrLiq(mensajeDe(e));
              } finally {
                setEnviando(false);
              }
            }}
          >
            <CheckCircle2 className="size-4" /> Cerrar liquidación
          </Button>
        </div>
      )}
    </Card>
  );
};

const DetalleLiquidacion = ({ id, alVolver }) => {
  const { data, error } = useApi(useCallback(() => getLiquidacion(id), [id]));
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data) return <Skeleton className="h-40" />;
  return (
    <div className="space-y-3">
      <div className="flex justify-between">
        <Button variant="ghost" onClick={alVolver}>
          <ArrowLeft className="size-4" /> Liquidaciones
        </Button>
        <Button variant="secondary" onClick={() => imprimirLiquidacion(data)}>
          <Printer className="size-4" /> Imprimir
        </Button>
      </div>
      <Card className="space-y-3 p-4">
        <p className="font-medium">
          {data.odontologo} · del {data.desde} al {data.hasta}
        </p>
        <p className="text-xs text-muted">
          Cerrada {fmtFechaHora(data.creadoEn)} por {data.creadoPor}
        </p>
        <TablaLineas lineas={data.lineas} />
        <p className="text-right text-lg">
          Total: <strong className="tabular-nums">{fmtDinero(data.total)}</strong>
        </p>
      </Card>
    </div>
  );
};

const Liquidaciones = ({ odontologoId }) => {
  const { data, error } = useApi(useCallback(() => getLiquidaciones(odontologoId), [odontologoId]));
  const [abierta, setAbierta] = useState(null);
  if (abierta) return <DetalleLiquidacion id={abierta} alVolver={() => setAbierta(null)} />;
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data) return <Skeleton className="h-32" />;
  if (!data.length) return <EmptyState title="Sin liquidaciones cerradas" />;
  return (
    <Card>
      <ul className="divide-y divide-line">
        {data.map((x) => (
          <li key={x.id}>
            <button
              onClick={() => setAbierta(x.id)}
              className="flex w-full items-center gap-4 px-4 py-3 text-left text-sm hover:bg-canvas"
            >
              <span className="flex-1">
                <span className="font-medium">{x.odontologo}</span>
                <span className="block text-xs text-muted">
                  {x.desde} → {x.hasta} · {x.cantidadLineas} procedimiento(s) · cerrada {fmtFechaHora(x.creadoEn)}
                </span>
              </span>
              <span className="font-medium tabular-nums">{fmtDinero(x.total)}</span>
            </button>
          </li>
        ))}
      </ul>
    </Card>
  );
};

const NuevaRegla = ({ alCerrar, alGuardar }) => {
  const odos = useApi(useCallback(() => getOdontologos(), []));
  const procs = useApi(useCallback(() => getProcedimientos(), []));
  const [f, setF] = useState({
    odontologoId: "",
    procedimientoId: "",
    tipo: "PORCENTAJE",
    valor: "",
    vigenteDesde: fechaISO(new Date()),
  });
  const [error, setError] = useState("");
  const [campos, setCampos] = useState({});
  return (
    <Modal open title="Nueva regla de honorarios" onClose={alCerrar}>
      <div className="space-y-3">
        {error && <Alert tone="danger">{error}</Alert>}
        <Select
          label="Odontólogo"
          value={f.odontologoId}
          onChange={(e) => setF({ ...f, odontologoId: e.target.value })}
        >
          <option value="">Todos</option>
          {odos.data?.map((o) => (
            <option key={o.id} value={o.id}>
              {o.nombre}
            </option>
          ))}
        </Select>
        <Select
          label="Procedimiento"
          value={f.procedimientoId}
          onChange={(e) => setF({ ...f, procedimientoId: e.target.value })}
        >
          <option value="">Todos</option>
          {procs.data?.map((p) => (
            <option key={p.id} value={p.id}>
              {p.codigo} · {p.nombre}
            </option>
          ))}
        </Select>
        <Select label="Tipo" value={f.tipo} onChange={(e) => setF({ ...f, tipo: e.target.value })}>
          {TIPOS_REGLA.map((t) => (
            <option key={t} value={t}>
              {ETIQUETA_REGLA[t]}
            </option>
          ))}
        </Select>
        <div className="grid gap-3 sm:grid-cols-2">
          <Input
            label={f.tipo === "MONTO_FIJO" ? "Monto por procedimiento" : "Porcentaje (0 a 100)"}
            inputMode="decimal"
            value={f.valor}
            onChange={(e) => setF({ ...f, valor: e.target.value })}
            error={campos.valor}
          />
          <Input
            label="Vigente desde"
            type="date"
            value={f.vigenteDesde}
            onChange={(e) => setF({ ...f, vigenteDesde: e.target.value })}
            error={campos.vigenteDesde}
          />
        </div>
        <p className="text-xs text-muted">
          Se aplica la regla más específica (odontólogo + procedimiento, luego odontólogo, luego procedimiento, luego
          general) vigente a la fecha del cobro. Una regla no se edita: para cambiarla, cree otra con una fecha de
          vigencia nueva.
        </p>
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={alCerrar}>
            Cancelar
          </Button>
          <Button
            onClick={async () => {
              setError("");
              setCampos({});
              try {
                await crearRegla({ ...f, valor: f.valor.replace(",", ".") });
                alGuardar();
              } catch (e) {
                setError(mensajeDe(e));
                setCampos(e.campos ?? {});
              }
            }}
          >
            Guardar regla
          </Button>
        </div>
      </div>
    </Modal>
  );
};

const Reglas = ({ configura }) => {
  const { data, error, recargar } = useApi(useCallback(() => getReglas(), []));
  const [nueva, setNueva] = useState(false);
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data) return <Skeleton className="h-32" />;
  return (
    <div className="space-y-3">
      {configura && (
        <Button variant="secondary" onClick={() => setNueva(true)}>
          <Plus className="size-4" /> Nueva regla
        </Button>
      )}
      <Card>
        {data.length === 0 ? (
          <EmptyState title="Sin reglas" />
        ) : (
          <ul className="divide-y divide-line">
            {[...data].reverse().map((r) => (
              <li key={r.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 text-sm">
                <span className="min-w-48 flex-1">
                  <span className="font-medium">{r.descripcion}</span>
                  <span className="block text-xs text-muted">
                    {r.odontologo ?? "Todos los odontólogos"} · {r.procedimiento ?? "todos los procedimientos"} ·
                    vigente desde {r.vigenteDesde}
                  </span>
                </span>
                <span className="text-xs text-muted">
                  {r.creadoPor} · {fmtFechaHora(r.creadoEn)}
                </span>
              </li>
            ))}
          </ul>
        )}
      </Card>
      {nueva && (
        <NuevaRegla
          alCerrar={() => setNueva(false)}
          alGuardar={() => {
            setNueva(false);
            recargar();
          }}
        />
      )}
    </div>
  );
};

const Honorarios = () => {
  const { puede, usuario } = useAuth();
  const todos = puede("honorarios.todos");
  const [pestana, setPestana] = useState(todos ? "liquidar" : "mios");
  const [desde, setDesde] = useState(inicioMes());
  const [hasta, setHasta] = useState(fechaISO(new Date()));
  const [odontologo, setOdontologo] = useState("");
  const [aviso, setAviso] = useState("");
  const [version, setVersion] = useState(0);
  const odos = useApi(useCallback(() => (todos ? getOdontologos() : Promise.resolve([])), [todos]));
  const pestanas = todos
    ? [
        { id: "liquidar", label: "Liquidar" },
        { id: "liquidaciones", label: "Liquidaciones" },
        { id: "reglas", label: "Reglas" },
      ]
    : [
        { id: "mios", label: "Mis honorarios" },
        { id: "liquidaciones", label: "Mis liquidaciones" },
        { id: "reglas", label: "Reglas que me aplican" },
      ];
  return (
    <>
      <PageHeader
        title="Honorarios"
        description="Comisiones sobre lo cobrado: % fijo, monto fijo por procedimiento o % del neto tras descontar laboratorio y materiales."
      />
      <Tabs value={pestana} onChange={setPestana} tabs={pestanas} />
      <div className="mt-4 space-y-4">
        {aviso && <Alert tone="ok">{aviso}</Alert>}
        {(pestana === "mios" || pestana === "liquidar") && (
          <>
            <Periodo desde={desde} hasta={hasta} setDesde={setDesde} setHasta={setHasta}>
              {todos && (
                <Select label="Odontólogo" value={odontologo} onChange={(e) => setOdontologo(e.target.value)}>
                  <option value="">Seleccione…</option>
                  {odos.data?.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.nombre}
                    </option>
                  ))}
                </Select>
              )}
            </Periodo>
            {desde && hasta && (todos ? odontologo : true) ? (
              <Calculo
                key={version}
                odontologoId={todos ? odontologo : usuario.id}
                desde={desde}
                hasta={hasta}
                alLiquidar={
                  todos
                    ? (x) => {
                        setAviso(`Liquidación cerrada: ${x.odontologo}, total ${fmtDinero(x.total)}.`);
                        setVersion((v) => v + 1);
                      }
                    : null
                }
              />
            ) : (
              <p className="text-sm text-muted">Elija el odontólogo y el periodo.</p>
            )}
          </>
        )}
        {pestana === "liquidaciones" && <Liquidaciones key={version} odontologoId={todos ? "" : usuario.id} />}
        {pestana === "reglas" && <Reglas configura={puede("honorarios.configurar")} />}
      </div>
    </>
  );
};

export default Honorarios;

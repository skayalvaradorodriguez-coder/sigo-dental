// Módulo "Inventario" (RF-14). El stock es la suma de los movimientos (kardex de sólo inserción);
// al registrar un procedimiento realizado se descargan automáticamente los insumos de su receta.
import { AlertTriangle, Pencil, Plus, Trash2 } from "lucide-react";
import { useCallback, useState } from "react";
import {
  Alert,
  Badge,
  Button,
  Card,
  EmptyState,
  Input,
  Modal,
  PageHeader,
  Select,
  Skeleton,
  Tabs,
} from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { mensajeDe } from "../../services/http";
import {
  actualizarInsumo,
  crearInsumo,
  getInsumos,
  getMovimientos,
  getRecetasConsumo,
  getStock,
  getUbicaciones,
  guardarRecetaConsumo,
  registrarMovimiento,
} from "../../services/inventarioService";
import { fmtCantidad, fmtDinero } from "../../utils/dinero";
import { ETIQUETA_MOVIMIENTO } from "../../utils/etiquetas";
import { fmtFechaHora } from "../../utils/fechas";

const Stock = ({ ubicaciones }) => {
  const [ubic, setUbic] = useState("");
  const { data, error } = useApi(useCallback(() => getStock(ubic), [ubic]));
  if (error) return <Alert tone="danger">{error}</Alert>;
  const alertas = (data ?? []).filter((x) => x.negativo || x.bajo);
  return (
    <div className="space-y-4">
      <Select label="Ubicación" value={ubic} onChange={(e) => setUbic(e.target.value)} className="max-w-xs">
        <option value="">Todas</option>
        {ubicaciones.map((u) => (
          <option key={u.id} value={u.id}>
            {u.nombre}
          </option>
        ))}
      </Select>
      {alertas.length > 0 && (
        <Alert tone="warn" title="Revise el inventario">
          <ul className="list-disc pl-4">
            {[...new Map(alertas.map((x) => [x.insumoId + (x.negativo ? x.ubicacionId : ""), x])).values()].map((x) => (
              <li key={x.insumoId + x.ubicacionId}>
                {x.negativo
                  ? `${x.insumo}: saldo negativo en ${x.ubicacion} (${fmtCantidad(x.cantidad)} ${x.unidad}). Registre la entrada o un ajuste.`
                  : `${x.insumo}: stock total bajo el mínimo (${fmtCantidad(x.stockMinimo)} ${x.unidad}).`}
              </li>
            ))}
          </ul>
        </Alert>
      )}
      <Card>
        {!data ? (
          <Skeleton className="m-4 h-40" />
        ) : data.length === 0 ? (
          <EmptyState title="Sin stock registrado" />
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-line text-left text-xs text-muted">
                <th className="px-4 py-2">Insumo</th>
                <th className="px-4 py-2">Ubicación</th>
                <th className="px-4 py-2 text-right">Saldo</th>
                <th className="px-4 py-2" />
              </tr>
            </thead>
            <tbody>
              {data.map((x) => (
                <tr key={x.insumoId + x.ubicacionId} className="border-b border-line">
                  <td className="px-4 py-2">
                    <span className="font-mono text-xs text-muted">{x.codigo}</span> {x.insumo}
                  </td>
                  <td className="px-4 py-2">{x.ubicacion}</td>
                  <td
                    className={`px-4 py-2 text-right tabular-nums ${x.negativo ? "font-semibold text-danger-700" : ""}`}
                  >
                    {fmtCantidad(x.cantidad)} {x.unidad}
                  </td>
                  <td className="px-4 py-2">
                    {x.negativo ? (
                      <Badge tone="danger">Negativo</Badge>
                    ) : (
                      x.bajo && <Badge tone="warn">Bajo mínimo</Badge>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  );
};

const Kardex = ({ insumos }) => {
  const [insumo, setInsumo] = useState("");
  const { data, error } = useApi(useCallback(() => getMovimientos({ insumoId: insumo }), [insumo]));
  if (error) return <Alert tone="danger">{error}</Alert>;
  return (
    <div className="space-y-4">
      <Select label="Insumo" value={insumo} onChange={(e) => setInsumo(e.target.value)} className="max-w-sm">
        <option value="">Todos (últimos 300 movimientos)</option>
        {insumos.map((i) => (
          <option key={i.id} value={i.id}>
            {i.codigo} · {i.nombre}
          </option>
        ))}
      </Select>
      <Card>
        {!data ? (
          <Skeleton className="m-4 h-40" />
        ) : data.length === 0 ? (
          <EmptyState title="Sin movimientos" />
        ) : (
          <ul className="divide-y divide-line">
            {data.map((m) => (
              <li key={m.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-2 text-sm">
                <span className="w-36 text-xs text-muted">{fmtFechaHora(m.registradoEn)}</span>
                <span className="min-w-48 flex-1">
                  <span className="font-medium">{m.insumo}</span> · {m.ubicacion}
                  <span className="block text-xs text-muted">
                    {ETIQUETA_MOVIMIENTO[m.tipo]}
                    {m.procedimiento && ` · ${m.procedimiento}`}
                    {m.motivo && ` · ${m.motivo}`} · {m.registradoPor}
                  </span>
                </span>
                <span className={`tabular-nums ${Number(m.cantidad) < 0 ? "text-danger-700" : "text-ok-700"}`}>
                  {Number(m.cantidad) > 0 ? "+" : ""}
                  {fmtCantidad(m.cantidad)} {m.unidad}
                </span>
                <span className="w-24 text-right text-xs text-muted">a {Number(m.costoUnitario).toFixed(4)}</span>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
};

const FormularioMovimiento = ({ insumos, ubicaciones }) => {
  const vacio = {
    tipo: "ENTRADA",
    insumoId: "",
    ubicacionId: ubicaciones[0]?.id ?? "",
    origenId: "",
    cantidad: "",
    costoUnitario: "",
    motivo: "",
  };
  const [f, setF] = useState(vacio);
  const [error, setError] = useState("");
  const [campos, setCampos] = useState({});
  const [ok, setOk] = useState("");
  const [enviando, setEnviando] = useState(false);
  const ins = insumos.find((i) => i.id === f.insumoId);
  return (
    <Card className="max-w-2xl space-y-3 p-4">
      {ok && <Alert tone="ok">{ok}</Alert>}
      {error && <Alert tone="danger">{error}</Alert>}
      <div className="grid gap-3 sm:grid-cols-2">
        <Select label="Tipo" value={f.tipo} onChange={(e) => setF({ ...f, tipo: e.target.value })}>
          <option value="ENTRADA">Entrada (compra o donación)</option>
          <option value="AJUSTE">Ajuste (conteo físico, vencimiento, rotura)</option>
          <option value="TRANSFERENCIA">Transferencia entre ubicaciones</option>
        </Select>
        <Select
          label="Insumo"
          value={f.insumoId}
          onChange={(e) => setF({ ...f, insumoId: e.target.value })}
          error={campos.insumoId}
        >
          <option value="">Seleccione…</option>
          {insumos.map((i) => (
            <option key={i.id} value={i.id}>
              {i.codigo} · {i.nombre} ({i.unidad})
            </option>
          ))}
        </Select>
        {f.tipo === "TRANSFERENCIA" && (
          <Select
            label="Desde"
            value={f.origenId}
            onChange={(e) => setF({ ...f, origenId: e.target.value })}
            error={campos.origenId}
          >
            <option value="">Seleccione…</option>
            {ubicaciones.map((u) => (
              <option key={u.id} value={u.id}>
                {u.nombre}
              </option>
            ))}
          </Select>
        )}
        <Select
          label={f.tipo === "TRANSFERENCIA" ? "Hacia" : "Ubicación"}
          value={f.ubicacionId}
          onChange={(e) => setF({ ...f, ubicacionId: e.target.value })}
          error={campos.ubicacionId}
        >
          {ubicaciones.map((u) => (
            <option key={u.id} value={u.id}>
              {u.nombre}
            </option>
          ))}
        </Select>
        <Input
          label={`Cantidad${ins ? ` (${ins.unidad})` : ""}`}
          inputMode="decimal"
          value={f.cantidad}
          onChange={(e) => setF({ ...f, cantidad: e.target.value })}
          error={campos.cantidad}
          hint={f.tipo === "AJUSTE" ? "Negativa para descontar (p. ej. -3), positiva para sumar." : undefined}
        />
        {f.tipo === "ENTRADA" && (
          <Input
            label="Costo unitario (opcional)"
            inputMode="decimal"
            placeholder={ins?.costoUnitario}
            value={f.costoUnitario}
            onChange={(e) => setF({ ...f, costoUnitario: e.target.value })}
            error={campos.costoUnitario}
            hint="Si lo indica, pasa a ser el costo del insumo para los descargos siguientes."
          />
        )}
      </div>
      <Input
        label={f.tipo === "AJUSTE" ? "Motivo (obligatorio)" : "Motivo / referencia (opcional)"}
        value={f.motivo}
        onChange={(e) => setF({ ...f, motivo: e.target.value })}
        error={campos.motivo}
      />
      <div className="flex justify-end">
        <Button
          loading={enviando}
          onClick={async () => {
            setEnviando(true);
            setError("");
            setCampos({});
            setOk("");
            try {
              await registrarMovimiento({
                ...f,
                cantidad: f.cantidad.replace(",", "."),
                costoUnitario: f.costoUnitario.replace(",", "."),
              });
              setOk(
                `${{ ENTRADA: "Entrada", AJUSTE: "Ajuste", TRANSFERENCIA: "Transferencia" }[f.tipo]} registrada: ${ins?.nombre}.`,
              );
              setF({ ...vacio, tipo: f.tipo });
            } catch (e) {
              setError(mensajeDe(e));
              setCampos(e.campos ?? {});
            } finally {
              setEnviando(false);
            }
          }}
        >
          Registrar
        </Button>
      </div>
    </Card>
  );
};

const EditorInsumo = ({ inicial, alCerrar, alGuardar }) => {
  const [f, setF] = useState(
    inicial ?? { codigo: "", nombre: "", unidad: "unidad", costoUnitario: "", stockMinimo: "0", activo: true },
  );
  const [error, setError] = useState("");
  const [campos, setCampos] = useState({});
  return (
    <Modal open title={inicial ? `Insumo ${inicial.codigo}` : "Nuevo insumo"} onClose={alCerrar}>
      <div className="space-y-3">
        {error && <Alert tone="danger">{error}</Alert>}
        <div className="grid gap-3 sm:grid-cols-2">
          <Input
            label="Código"
            value={f.codigo}
            disabled={!!inicial}
            onChange={(e) => setF({ ...f, codigo: e.target.value.toUpperCase() })}
            error={campos.codigo}
          />
          <Input
            label="Unidad"
            value={f.unidad}
            onChange={(e) => setF({ ...f, unidad: e.target.value })}
            error={campos.unidad}
          />
        </div>
        <Input
          label="Nombre"
          value={f.nombre}
          onChange={(e) => setF({ ...f, nombre: e.target.value })}
          error={campos.nombre}
        />
        <div className="grid gap-3 sm:grid-cols-2">
          <Input
            label="Costo unitario"
            inputMode="decimal"
            value={f.costoUnitario}
            onChange={(e) => setF({ ...f, costoUnitario: e.target.value })}
            error={campos.costoUnitario}
          />
          <Input
            label="Stock mínimo"
            inputMode="decimal"
            value={f.stockMinimo}
            onChange={(e) => setF({ ...f, stockMinimo: e.target.value })}
            error={campos.stockMinimo}
          />
        </div>
        {inicial && (
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={f.activo}
              onChange={(e) => setF({ ...f, activo: e.target.checked })}
              className="accent-brand-700"
            />
            Activo (un insumo inactivo no se descarga ni se elige)
          </label>
        )}
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={alCerrar}>
            Cancelar
          </Button>
          <Button
            onClick={async () => {
              setError("");
              setCampos({});
              try {
                if (inicial) await actualizarInsumo(inicial.id, f);
                else await crearInsumo(f);
                alGuardar();
              } catch (e) {
                setError(mensajeDe(e));
                setCampos(e.campos ?? {});
              }
            }}
          >
            Guardar
          </Button>
        </div>
      </div>
    </Modal>
  );
};

const Insumos = ({ insumos, configura, alCambiar }) => {
  const [editor, setEditor] = useState(null);
  return (
    <div className="space-y-3">
      {configura && (
        <Button variant="secondary" onClick={() => setEditor({})}>
          <Plus className="size-4" /> Nuevo insumo
        </Button>
      )}
      <Card>
        <ul className="divide-y divide-line">
          {insumos.map((i) => (
            <li key={i.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-2 text-sm">
              <span className="w-20 font-mono text-xs text-muted">{i.codigo}</span>
              <span className="min-w-48 flex-1">
                {i.nombre} {!i.activo && <Badge tone="neutral">Inactivo</Badge>}
                <span className="block text-xs text-muted">
                  Costo {Number(i.costoUnitario).toFixed(4)} por {i.unidad} · mínimo {fmtCantidad(i.stockMinimo)}
                </span>
              </span>
              <span className="tabular-nums">
                {fmtCantidad(i.stockTotal)} {i.unidad}
              </span>
              {configura && (
                <Button variant="ghost" aria-label={`Editar ${i.nombre}`} onClick={() => setEditor({ inicial: i })}>
                  <Pencil className="size-4" />
                </Button>
              )}
            </li>
          ))}
        </ul>
      </Card>
      {editor && (
        <EditorInsumo
          inicial={editor.inicial}
          alCerrar={() => setEditor(null)}
          alGuardar={() => {
            setEditor(null);
            alCambiar();
          }}
        />
      )}
    </div>
  );
};

const EditorReceta = ({ receta, insumos, alCerrar, alGuardar }) => {
  const [lineas, setLineas] = useState(
    receta.lineas.map((l) => ({ insumoId: l.insumoId, cantidad: String(Number(l.cantidad)) })),
  );
  const [error, setError] = useState("");
  return (
    <Modal open title={`Receta de consumo: ${receta.codigo} · ${receta.procedimiento}`} onClose={alCerrar}>
      <div className="space-y-3">
        {error && <Alert tone="danger">{error}</Alert>}
        <p className="text-sm text-muted">
          Insumos que se descuentan cada vez que se registra este procedimiento. Los cambios no alteran los descargos
          anteriores.
        </p>
        {lineas.map((l, i) => (
          <div key={i} className="grid grid-cols-[1fr_7rem_auto] items-end gap-2">
            <Select
              label="Insumo"
              value={l.insumoId}
              onChange={(e) => setLineas(lineas.map((x, j) => (j === i ? { ...x, insumoId: e.target.value } : x)))}
            >
              <option value="">Seleccione…</option>
              {insumos
                .filter((x) => x.activo)
                .map((x) => (
                  <option key={x.id} value={x.id}>
                    {x.nombre} ({x.unidad})
                  </option>
                ))}
            </Select>
            <Input
              label="Cantidad"
              inputMode="decimal"
              value={l.cantidad}
              onChange={(e) => setLineas(lineas.map((x, j) => (j === i ? { ...x, cantidad: e.target.value } : x)))}
            />
            <Button
              variant="ghost"
              aria-label="Quitar insumo"
              onClick={() => setLineas(lineas.filter((_, j) => j !== i))}
            >
              <Trash2 className="size-4" />
            </Button>
          </div>
        ))}
        <Button variant="secondary" onClick={() => setLineas([...lineas, { insumoId: "", cantidad: "1" }])}>
          <Plus className="size-4" /> Agregar insumo
        </Button>
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={alCerrar}>
            Cancelar
          </Button>
          <Button
            onClick={async () => {
              setError("");
              try {
                await guardarRecetaConsumo(
                  receta.procedimientoId,
                  lineas.map((l) => ({ ...l, cantidad: l.cantidad.replace(",", ".") })),
                );
                alGuardar();
              } catch (e) {
                setError(mensajeDe(e));
              }
            }}
          >
            Guardar receta
          </Button>
        </div>
      </div>
    </Modal>
  );
};

const Recetas = ({ insumos, configura }) => {
  const { data, error, recargar } = useApi(useCallback(() => getRecetasConsumo(), []));
  const [editando, setEditando] = useState(null);
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data) return <Skeleton className="h-40" />;
  return (
    <>
      <div className="grid gap-3 md:grid-cols-2">
        {data.map((r) => (
          <Card key={r.procedimientoId} className="p-4">
            <div className="flex items-start justify-between gap-2">
              <p className="font-medium">
                {r.codigo} · {r.procedimiento}
              </p>
              {configura && (
                <Button
                  variant="ghost"
                  aria-label={`Editar receta de ${r.procedimiento}`}
                  onClick={() => setEditando(r)}
                >
                  <Pencil className="size-4" />
                </Button>
              )}
            </div>
            {r.lineas.length === 0 ? (
              <p className="text-sm text-muted">Sin receta: no se descuentan insumos.</p>
            ) : (
              <ul className="mt-1 text-sm">
                {r.lineas.map((l) => (
                  <li key={l.insumoId} className="flex justify-between">
                    <span>
                      {fmtCantidad(l.cantidad)} {l.unidad} · {l.insumo}
                    </span>
                    <span className="tabular-nums text-muted">{fmtDinero(l.costo)}</span>
                  </li>
                ))}
              </ul>
            )}
            <p className="mt-2 text-xs text-muted">Costo estándar de materiales: {fmtDinero(r.costoEstandar)}</p>
          </Card>
        ))}
      </div>
      {editando && (
        <EditorReceta
          receta={editando}
          insumos={insumos}
          alCerrar={() => setEditando(null)}
          alGuardar={() => {
            setEditando(null);
            recargar();
          }}
        />
      )}
    </>
  );
};

const Inventario = () => {
  const { puede } = useAuth();
  const ubic = useApi(useCallback(() => getUbicaciones(), []));
  const configura = puede("inventario.configurar");
  const ins = useApi(useCallback(() => getInsumos(configura), [configura]));
  const pestanas = [
    { id: "stock", label: "Stock" },
    { id: "kardex", label: "Movimientos" },
    ...(puede("inventario.escribir") ? [{ id: "registrar", label: "Registrar movimiento" }] : []),
    { id: "insumos", label: "Insumos" },
    { id: "recetas", label: "Recetas de consumo" },
  ];
  const [pestana, setPestana] = useState("stock");
  if (ubic.error || ins.error) return <Alert tone="danger">{ubic.error || ins.error}</Alert>;
  return (
    <>
      <PageHeader
        title="Inventario"
        description="Stock por ubicación calculado desde el kardex. Al registrar un procedimiento realizado se descuentan sus insumos automáticamente."
      />
      <Tabs value={pestana} onChange={setPestana} tabs={pestanas} />
      <div className="mt-4">
        {!ubic.data || !ins.data ? (
          <Skeleton className="h-40" />
        ) : (
          <>
            {pestana === "stock" && <Stock ubicaciones={ubic.data} />}
            {pestana === "kardex" && <Kardex insumos={ins.data} />}
            {pestana === "registrar" && (
              <FormularioMovimiento insumos={ins.data.filter((i) => i.activo)} ubicaciones={ubic.data} />
            )}
            {pestana === "insumos" && (
              <Insumos insumos={ins.data} configura={puede("inventario.configurar")} alCambiar={ins.recargar} />
            )}
            {pestana === "recetas" && <Recetas insumos={ins.data} configura={puede("inventario.configurar")} />}
          </>
        )}
      </div>
      {ins.data?.some((i) => Number(i.stockTotal) < Number(i.stockMinimo)) && pestana !== "stock" && (
        <p className="mt-4 text-sm text-warn-700">
          <AlertTriangle className="mr-1 inline size-4" /> Hay insumos bajo el stock mínimo: revise la pestaña Stock.
        </p>
      )}
    </>
  );
};

export default Inventario;

// Catálogos (Etapa 2): sedes → consultorios → sillones, procedimientos y convenios. Nada se borra: se desactiva.
import { Pencil, Plus } from "lucide-react";
import { useCallback, useState } from "react";
import {
  Alert,
  Badge,
  Button,
  Card,
  Checkbox,
  EmptyState,
  Input,
  Modal,
  PageHeader,
  Skeleton,
  Tabs,
} from "../../components/ui";
import { useAccion } from "../../hooks/useAccion";
import { useApi } from "../../hooks/useApi";
import * as api from "../../services/catalogosService";

const TITULO = {
  sedes: "sede",
  consultorios: "consultorio",
  sillones: "sillón",
  procedimientos: "procedimiento",
  convenios: "convenio",
};

const guardar = ({ tipo, fila, datos }) =>
  fila ? api.actualizarCatalogo(tipo, fila.id, datos) : api.crearCatalogo(tipo, datos);

const ModalEdicion = ({ edicion, alCerrar }) => {
  const { tipo, fila } = edicion;
  const [f, setF] = useState({
    nombre: fila?.nombre ?? "",
    direccion: fila?.direccion ?? "",
    aseguradora: fila?.aseguradora ?? "",
    codigo: fila?.codigo ?? "",
    duracionMin: fila?.duracionMin ?? 30,
    arancel: fila?.arancel ?? "0.00",
    activo: fila ? fila.activo : true,
  });
  const [errorLocal, setErrorLocal] = useState("");
  const { ejecutar, enviando, error } = useAccion(guardar);

  const enviar = async (e) => {
    e.preventDefault();
    if (f.nombre.trim().length < 1) return setErrorLocal("Ingrese un nombre.");
    if (tipo === "procedimientos" && !/^\d{1,10}(\.\d{1,2})?$/.test(f.arancel))
      return setErrorLocal("Arancel: número con hasta 2 decimales.");
    setErrorLocal("");
    const datos = { nombre: f.nombre.trim() };
    if (fila) datos.activo = f.activo;
    if (tipo === "sedes") datos.direccion = f.direccion.trim();
    if (tipo === "convenios") datos.aseguradora = f.aseguradora.trim();
    if (tipo === "procedimientos") {
      datos.duracionMin = Number(f.duracionMin);
      datos.arancel = f.arancel.trim();
      if (!fila) datos.codigo = f.codigo.trim().toUpperCase();
    }
    if (!fila && tipo === "consultorios") datos.sedeId = edicion.sedeId;
    if (!fila && tipo === "sillones") datos.consultorioId = edicion.consultorioId;
    try {
      await ejecutar({ tipo, fila, datos });
      alCerrar(true);
    } catch {
      // el mensaje queda en "error"
    }
  };

  return (
    <Modal open title={`${fila ? "Editar" : "Nuevo"} ${TITULO[tipo]}`} onClose={() => alCerrar(false)}>
      <form onSubmit={enviar} noValidate className="space-y-4">
        {(errorLocal || error) && <Alert tone="danger">{errorLocal || error}</Alert>}
        {tipo === "procedimientos" && !fila && (
          <Input
            label="Código"
            placeholder="PRO-02"
            value={f.codigo}
            onChange={(e) => setF({ ...f, codigo: e.target.value })}
          />
        )}
        <Input label="Nombre" value={f.nombre} onChange={(e) => setF({ ...f, nombre: e.target.value })} autoFocus />
        {tipo === "sedes" && (
          <Input label="Dirección" value={f.direccion} onChange={(e) => setF({ ...f, direccion: e.target.value })} />
        )}
        {tipo === "convenios" && (
          <Input
            label="Aseguradora"
            value={f.aseguradora}
            onChange={(e) => setF({ ...f, aseguradora: e.target.value })}
          />
        )}
        {tipo === "procedimientos" && (
          <div className="grid grid-cols-2 gap-3">
            <Input
              label="Duración sugerida (min)"
              type="number"
              min={5}
              max={480}
              step={5}
              value={f.duracionMin}
              onChange={(e) => setF({ ...f, duracionMin: e.target.value })}
            />
            <Input
              label="Arancel de referencia"
              inputMode="decimal"
              value={f.arancel}
              onChange={(e) => setF({ ...f, arancel: e.target.value })}
            />
          </div>
        )}
        {fila && (
          <Checkbox
            label="Activo (si se desactiva deja de ofrecerse, pero se conserva el historial)"
            checked={f.activo}
            onChange={(v) => setF({ ...f, activo: v })}
          />
        )}
        <div className="flex justify-end gap-2 border-t border-line pt-4">
          <Button type="button" variant="secondary" onClick={() => alCerrar(false)}>
            Cancelar
          </Button>
          <Button type="submit" loading={enviando}>
            Guardar
          </Button>
        </div>
      </form>
    </Modal>
  );
};

const Estado = ({ activo }) => (activo ? null : <Badge>Inactivo</Badge>);
const BotonEditar = ({ onClick, etiqueta }) => (
  <button onClick={onClick} className="rounded p-1.5 text-muted hover:bg-canvas hover:text-ink" aria-label={etiqueta}>
    <Pencil className="size-3.5" />
  </button>
);

const Catalogos = () => {
  const [pestana, setPestana] = useState("sillones");
  const [edicion, setEdicion] = useState(null);
  const estructura = useApi(useCallback(() => api.getEstructura(true), []));
  const procedimientos = useApi(useCallback(() => api.getProcedimientos(true), []));
  const convenios = useApi(useCallback(() => api.getConvenios(true), []));

  const cerrar = (cambio) => {
    if (cambio) {
      estructura.recargar();
      procedimientos.recargar();
      convenios.recargar();
    }
    setEdicion(null);
  };

  return (
    <>
      <PageHeader
        title="Catálogos"
        description="Sedes, consultorios y sillones de la agenda; procedimientos y convenios. Nada se borra: se desactiva."
      />
      <Tabs
        value={pestana}
        onChange={setPestana}
        tabs={[
          { id: "sillones", label: "Sedes y sillones" },
          { id: "procedimientos", label: "Procedimientos" },
          { id: "convenios", label: "Seguros y convenios" },
        ]}
      />
      <div className="mt-4">
        {pestana === "sillones" &&
          (estructura.isLoading && !estructura.data ? (
            <Skeleton className="h-40" />
          ) : (
            <div className="space-y-4">
              {estructura.error && <Alert tone="danger">{estructura.error}</Alert>}
              <Button variant="secondary" onClick={() => setEdicion({ tipo: "sedes" })}>
                <Plus className="size-4" /> Nueva sede
              </Button>
              {estructura.data?.length === 0 && (
                <Card>
                  <EmptyState title="Sin sedes registradas" />
                </Card>
              )}
              {estructura.data?.map((s) => (
                <Card key={s.id} className="p-4">
                  <div className="flex items-center gap-2">
                    <h2 className="font-semibold">{s.nombre}</h2> <Estado activo={s.activo} />
                    <BotonEditar
                      etiqueta={`Editar ${s.nombre}`}
                      onClick={() => setEdicion({ tipo: "sedes", fila: s })}
                    />
                    <Button
                      variant="ghost"
                      className="ml-auto"
                      onClick={() => setEdicion({ tipo: "consultorios", sedeId: s.id })}
                    >
                      <Plus className="size-4" /> Consultorio
                    </Button>
                  </div>
                  {s.direccion && <p className="text-sm text-muted">{s.direccion}</p>}
                  <div className="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                    {(s.consultorios ?? []).map((c) => (
                      <div key={c.id} className="rounded-lg border border-line p-3">
                        <div className="flex items-center gap-2">
                          <p className="font-medium">{c.nombre}</p> <Estado activo={c.activo} />
                          <BotonEditar
                            etiqueta={`Editar ${c.nombre}`}
                            onClick={() => setEdicion({ tipo: "consultorios", sedeId: s.id, fila: c })}
                          />
                        </div>
                        <ul className="mt-2 space-y-1">
                          {(c.sillones ?? []).map((x) => (
                            <li key={x.id} className="flex items-center gap-2 text-sm">
                              <span className={x.activo ? "" : "text-muted line-through"}>{x.nombre}</span>
                              <BotonEditar
                                etiqueta={`Editar ${x.nombre}`}
                                onClick={() => setEdicion({ tipo: "sillones", consultorioId: c.id, fila: x })}
                              />
                            </li>
                          ))}
                          {!c.sillones?.length && <li className="text-sm text-muted">Sin sillones</li>}
                        </ul>
                        <Button
                          variant="ghost"
                          className="mt-1 px-2"
                          onClick={() => setEdicion({ tipo: "sillones", consultorioId: c.id })}
                        >
                          <Plus className="size-4" /> Sillón
                        </Button>
                      </div>
                    ))}
                  </div>
                </Card>
              ))}
            </div>
          ))}

        {pestana === "procedimientos" && (
          <Card>
            <div className="flex justify-end border-b border-line p-3">
              <Button onClick={() => setEdicion({ tipo: "procedimientos" })}>
                <Plus className="size-4" /> Nuevo procedimiento
              </Button>
            </div>
            {procedimientos.isLoading && !procedimientos.data ? (
              <Skeleton className="m-4 h-32" />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="border-b border-line text-left text-xs text-muted">
                    <tr>
                      <th className="px-4 py-2 font-medium">Código</th>
                      <th className="px-4 py-2 font-medium">Nombre</th>
                      <th className="px-4 py-2 font-medium">Duración</th>
                      <th className="px-4 py-2 text-right font-medium">Arancel ref.</th>
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    {procedimientos.data?.map((p) => (
                      <tr key={p.id} className="border-b border-line last:border-0">
                        <td className="px-4 py-2 font-mono text-xs">{p.codigo}</td>
                        <td className="px-4 py-2">
                          {p.nombre} <Estado activo={p.activo} />
                        </td>
                        <td className="px-4 py-2">{p.duracionMin} min</td>
                        <td className="px-4 py-2 text-right tabular-nums">{p.arancel}</td>
                        <td className="px-2">
                          <BotonEditar
                            etiqueta={`Editar ${p.nombre}`}
                            onClick={() => setEdicion({ tipo: "procedimientos", fila: p })}
                          />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
        )}

        {pestana === "convenios" && (
          <Card>
            <div className="flex justify-end border-b border-line p-3">
              <Button onClick={() => setEdicion({ tipo: "convenios" })}>
                <Plus className="size-4" /> Nuevo convenio
              </Button>
            </div>
            {convenios.isLoading && !convenios.data ? (
              <Skeleton className="m-4 h-32" />
            ) : convenios.data?.length === 0 ? (
              <EmptyState title="Sin convenios" />
            ) : (
              <ul className="divide-y divide-line">
                {convenios.data?.map((p) => (
                  <li key={p.id} className="flex items-center gap-2 px-4 py-3 text-sm">
                    <span className="font-medium">{p.nombre}</span> <span className="text-muted">{p.aseguradora}</span>{" "}
                    <Estado activo={p.activo} />
                    <span className="ml-auto">
                      <BotonEditar
                        etiqueta={`Editar ${p.nombre}`}
                        onClick={() => setEdicion({ tipo: "convenios", fila: p })}
                      />
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        )}
      </div>
      {edicion && <ModalEdicion edicion={edicion} alCerrar={cerrar} />}
    </>
  );
};

export default Catalogos;

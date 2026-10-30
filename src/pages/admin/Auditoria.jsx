// Bitácora de auditoría (RNF-05): sólo inserción, encadenada por hash SHA-256, con verificación.
import { ChevronDown, ChevronRight, ScrollText, ShieldCheck } from "lucide-react";
import { Fragment, useCallback, useState } from "react";
import { Alert, Badge, Button, Card, EmptyState, PageHeader, Select, Skeleton } from "../../components/ui";
import { useAccion } from "../../hooks/useAccion";
import { useApi } from "../../hooks/useApi";
import { getAuditoria, verificarAuditoria } from "../../services/sistemaService";
import { ETIQUETA_ACCION } from "../../utils/etiquetas";
import { fmtFechaHoraSeg } from "../../utils/fechas";

const PELIGRO = new Set(["LOGIN_FALLIDO", "CUENTA_BLOQUEADA", "MFA_FALLIDO"]);
const LIMITE = 25;

const Auditoria = () => {
  const [filtros, setFiltros] = useState({ accion: "", desde: "", hasta: "" });
  const [pagina, setPagina] = useState(1);
  const [abierto, setAbierto] = useState(null);
  const [verificacion, setVerificacion] = useState(null);
  const verificar = useAccion(verificarAuditoria);

  const cargar = useCallback(
    () =>
      getAuditoria({
        page: pagina,
        limit: LIMITE,
        accion: filtros.accion,
        desde: filtros.desde && new Date(`${filtros.desde}T00:00:00`).toISOString(),
        hasta: filtros.hasta && new Date(`${filtros.hasta}T23:59:59.999`).toISOString(),
      }),
    [pagina, filtros],
  );
  const { data, isLoading, error } = useApi(cargar);
  const paginas = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1;
  const poner = (k, v) => {
    setFiltros({ ...filtros, [k]: v });
    setPagina(1);
  };

  return (
    <>
      <PageHeader
        title="Auditoría"
        description="Bitácora de sólo inserción: quién hizo qué, cuándo (ISO-8601) y desde qué IP. Cada registro encadena el hash del anterior."
        actions={
          <Button
            variant="secondary"
            loading={verificar.enviando}
            onClick={() =>
              verificar
                .ejecutar()
                .then(setVerificacion)
                .catch(() => {})
            }
          >
            <ShieldCheck className="size-4" /> Verificar integridad
          </Button>
        }
      />
      {(verificacion || verificar.error) && (
        <div className="mb-4">
          {verificar.error ? (
            <Alert tone="danger">{verificar.error}</Alert>
          ) : verificacion.valida ? (
            <Alert tone="ok" title="Cadena íntegra">
              Se verificaron {verificacion.verificados} registros sin alteraciones.
            </Alert>
          ) : (
            <Alert tone="danger" title="Se detectó una alteración">
              La cadena se rompe en el registro #{verificacion.rotaEnId}. Escale el incidente al responsable de
              seguridad.
            </Alert>
          )}
        </div>
      )}
      <Card>
        <div className="grid gap-3 border-b border-line p-4 sm:grid-cols-3">
          <Select label="Acción" value={filtros.accion} onChange={(e) => poner("accion", e.target.value)}>
            <option value="">Todas</option>
            {Object.entries(ETIQUETA_ACCION).map(([k, v]) => (
              <option key={k} value={k}>
                {v}
              </option>
            ))}
          </Select>
          {[
            ["desde", "Desde"],
            ["hasta", "Hasta"],
          ].map(([k, t]) => (
            <div key={k}>
              <label htmlFor={`f-${k}`} className="mb-1 block text-sm font-medium">
                {t}
              </label>
              <input
                id={`f-${k}`}
                type="date"
                value={filtros[k]}
                onChange={(e) => poner(k, e.target.value)}
                className="min-h-11 w-full rounded-lg border border-line px-3"
              />
            </div>
          ))}
        </div>
        {isLoading && !data ? (
          <div className="space-y-2 p-4">
            {[0, 1, 2, 3, 4].map((i) => (
              <Skeleton key={i} className="h-9" />
            ))}
          </div>
        ) : error ? (
          <div className="p-4">
            <Alert tone="danger">{error}</Alert>
          </div>
        ) : data.data.length === 0 ? (
          <EmptyState icon={<ScrollText className="size-8" />} title="Sin registros para estos filtros" />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px] text-sm">
              <thead className="border-b border-line text-left text-xs text-muted">
                <tr>
                  <th className="w-8 px-2 py-2.5" />
                  <th className="px-3 py-2.5 font-medium">Fecha y hora</th>
                  <th className="px-3 py-2.5 font-medium">Acción</th>
                  <th className="px-3 py-2.5 font-medium">Usuario</th>
                  <th className="px-3 py-2.5 font-medium">Entidad</th>
                  <th className="px-3 py-2.5 font-medium">IP</th>
                </tr>
              </thead>
              <tbody>
                {data.data.map((r) => (
                  <Fragment key={r.id}>
                    <tr className="border-b border-line hover:bg-canvas/60">
                      <td className="px-2 py-2">
                        <button
                          aria-label="Ver detalle"
                          aria-expanded={abierto === r.id}
                          onClick={() => setAbierto(abierto === r.id ? null : r.id)}
                          className="rounded p-1 text-muted hover:bg-line/50"
                        >
                          {abierto === r.id ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
                        </button>
                      </td>
                      <td className="whitespace-nowrap px-3 py-2" title={r.ocurridoEn}>
                        {fmtFechaHoraSeg(r.ocurridoEn)}
                      </td>
                      <td className="px-3 py-2">
                        <Badge tone={PELIGRO.has(r.accion) ? "danger" : "neutral"}>
                          {ETIQUETA_ACCION[r.accion] ?? r.accion}
                        </Badge>
                      </td>
                      <td className="px-3 py-2">{r.usuario ?? <span className="text-muted">—</span>}</td>
                      <td className="px-3 py-2 text-muted">
                        {r.entidad}
                        {r.entidadId && ` · ${r.entidadId.slice(0, 18)}`}
                      </td>
                      <td className="px-3 py-2 font-mono text-xs text-muted">{r.ip ?? "—"}</td>
                    </tr>
                    {abierto === r.id && (
                      <tr className="border-b border-line bg-canvas/60">
                        <td />
                        <td colSpan={5} className="px-3 py-3 text-xs">
                          <dl className="grid gap-x-6 gap-y-1 sm:grid-cols-[max-content_1fr]">
                            <dt className="text-muted">Registro</dt>
                            <dd>
                              #{r.id} · {r.ocurridoEn}
                            </dd>
                            <dt className="text-muted">Correo</dt>
                            <dd>{r.email ?? "—"}</dd>
                            <dt className="text-muted">Navegador</dt>
                            <dd className="break-all">{r.userAgent ?? "—"}</dd>
                            <dt className="text-muted">Detalle</dt>
                            <dd>
                              <pre className="whitespace-pre-wrap break-all font-mono">
                                {r.detalle ? JSON.stringify(r.detalle, null, 2) : "—"}
                              </pre>
                            </dd>
                          </dl>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {data && data.total > 0 && (
          <div className="flex items-center justify-between border-t border-line px-4 py-3 text-sm">
            <p className="text-muted">
              {data.total} registros · página {pagina} de {paginas}
            </p>
            <div className="flex gap-2">
              <Button variant="secondary" disabled={pagina <= 1} onClick={() => setPagina(pagina - 1)}>
                Anterior
              </Button>
              <Button variant="secondary" disabled={pagina >= paginas} onClick={() => setPagina(pagina + 1)}>
                Siguiente
              </Button>
            </div>
          </div>
        )}
      </Card>
    </>
  );
};

export default Auditoria;

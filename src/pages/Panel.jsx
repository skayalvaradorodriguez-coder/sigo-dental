// Panel de inicio: sólo cifras reales y sólo los widgets que el rol puede ver (GET /api/panel).
import { ArrowRight, ShieldAlert } from "lucide-react";
import { useCallback, useEffect } from "react";
import { Link } from "react-router-dom";
import { ETAPA_ACTUAL, MODULOS } from "../components/modulos";
import { Alert, Badge, Card, Skeleton } from "../components/ui";
import { useAuth } from "../context/useAuth";
import { useApi } from "../hooks/useApi";
import { getPanel } from "../services/sistemaService";
import { ETIQUETA_ACCION, ETIQUETA_ROL } from "../utils/etiquetas";
import { fmtFechaHora, fmtRelativo } from "../utils/fechas";

const Cifra = ({ etiqueta, valor, tono }) => (
  <div className="rounded-lg border border-line bg-white p-4">
    <p className="text-xs font-medium text-muted">{etiqueta}</p>
    <p
      className={`mt-1 text-2xl font-semibold ${
        tono === "danger" && valor > 0 ? "text-danger-700" : tono === "warn" && valor > 0 ? "text-warn-700" : ""
      }`}
    >
      {valor}
    </p>
  </div>
);

const Titulo = ({ id, children }) => (
  <h2 id={id} className="text-sm font-semibold uppercase tracking-wider text-muted">
    {children}
  </h2>
);

const Panel = () => {
  const { usuario, puede } = useAuth();
  const { data, error, recargar } = useApi(useCallback(() => getPanel(), []));

  // Se refresca cada minuto mientras el panel está abierto.
  useEffect(() => {
    const t = setInterval(recargar, 60_000);
    return () => clearInterval(t);
  }, [recargar]);

  const nombre = usuario.nombre.split(" ").find((p) => !/^(dr|dra)\.?$/i.test(p)) ?? usuario.nombre;
  const mios = MODULOS.filter((m) => m.ruta !== "/" && puede(...m.permisos));
  const listos = mios.filter((m) => m.etapa <= ETAPA_ACTUAL);
  const proximos = mios.filter((m) => m.etapa > ETAPA_ACTUAL);
  const w = data?.widgets;
  const sum = (o, ...ks) => ks.reduce((a, k) => a + (o?.[k] ?? 0), 0);

  return (
    <div className="space-y-6">
      <header>
        <p className="text-sm text-muted">{usuario.etiquetaRol}</p>
        <h1 className="text-2xl font-semibold tracking-tight">Hola, {nombre}</h1>
        <p className="mt-1 text-sm text-muted">
          Último acceso {fmtRelativo(w?.yo.ultimoAcceso ?? usuario.ultimoAcceso)}
          {w && w.yo.sesionesAbiertas > 1 && <> · {w.yo.sesionesAbiertas} sesiones abiertas con su usuario</>}
        </p>
      </header>

      {error && (
        <Alert tone="danger" title="No se pudo cargar el panel">
          {error}
        </Alert>
      )}

      {puede("usuarios.gestionar") && (
        <section aria-labelledby="t-usr" className="space-y-3">
          <Titulo id="t-usr">Usuarios y accesos</Titulo>
          {!w?.usuarios ? (
            <div className="grid gap-3 sm:grid-cols-3">
              {[0, 1, 2].map((i) => (
                <Skeleton key={i} className="h-20" />
              ))}
            </div>
          ) : (
            <>
              <div className="grid gap-3 sm:grid-cols-3">
                <Cifra etiqueta="Usuarios activos" valor={w.usuarios.porRol.reduce((a, r) => a + r.activos, 0)} />
                <Cifra
                  etiqueta="Activos sin MFA configurado"
                  valor={w.usuarios.porRol.reduce((a, r) => a + r.sinMfa, 0)}
                  tono="warn"
                />
                <Cifra etiqueta="Sesiones abiertas ahora" valor={w.usuarios.sesionesAbiertas} />
              </div>
              <Card className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="border-b border-line text-left text-xs text-muted">
                    <tr>
                      {["Rol", "Activos", "Inactivos", "Sin MFA", "Bloqueados"].map((t) => (
                        <th key={t} className="px-4 py-2 font-medium">
                          {t}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {w.usuarios.porRol.map((r) => (
                      <tr key={r.rol} className="border-b border-line last:border-0">
                        <td className="px-4 py-2">{ETIQUETA_ROL[r.rol]}</td>
                        <td className="px-4 py-2">{r.activos}</td>
                        <td className="px-4 py-2 text-muted">{r.inactivos}</td>
                        <td className="px-4 py-2">{r.sinMfa > 0 ? <Badge tone="warn">{r.sinMfa}</Badge> : 0}</td>
                        <td className="px-4 py-2">
                          {r.bloqueados > 0 ? <Badge tone="danger">{r.bloqueados}</Badge> : 0}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </Card>
            </>
          )}
        </section>
      )}

      {w?.seguridad && (
        <section aria-labelledby="t-seg" className="space-y-3">
          <Titulo id="t-seg">Seguridad · últimas 24 horas</Titulo>
          <div className="grid gap-3 sm:grid-cols-4">
            <Cifra etiqueta="Intentos fallidos" valor={sum(w.seguridad.ultimas24h, "LOGIN_FALLIDO")} tono="warn" />
            <Cifra
              etiqueta="Bloqueos de cuenta"
              valor={sum(w.seguridad.ultimas24h, "CUENTA_BLOQUEADA")}
              tono="danger"
            />
            <Cifra etiqueta="Códigos MFA inválidos" valor={sum(w.seguridad.ultimas24h, "MFA_FALLIDO")} tono="warn" />
            <Cifra etiqueta="Sesiones expiradas" valor={sum(w.seguridad.ultimas24h, "SESION_EXPIRADA")} />
          </div>
          <Card>
            <div className="flex items-center justify-between border-b border-line px-4 py-3">
              <p className="text-sm font-medium">Eventos de seguridad recientes</p>
              <Link to="/auditoria" className="inline-flex items-center gap-1 text-sm text-brand-700 hover:underline">
                Ver bitácora <ArrowRight className="size-3.5" />
              </Link>
            </div>
            {w.seguridad.recientes.length === 0 ? (
              <p className="px-4 py-6 text-center text-sm text-muted">Sin eventos de seguridad registrados.</p>
            ) : (
              <ul className="divide-y divide-line">
                {w.seguridad.recientes.map((e) => (
                  <li key={e.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-2.5 text-sm">
                    <ShieldAlert className="size-4 text-warn-700" aria-hidden />
                    <span className="font-medium">{ETIQUETA_ACCION[e.accion] ?? e.accion}</span>
                    <span className="text-muted">{e.usuario ?? e.detalle?.email ?? "Usuario no identificado"}</span>
                    <span className="ml-auto text-xs text-muted">
                      {fmtFechaHora(e.ocurridoEn)} · {e.ip ?? "s/IP"}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </section>
      )}

      <section aria-labelledby="t-mod" className="space-y-3">
        <Titulo id="t-mod">Sus módulos</Titulo>
        {listos.length > 0 && (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {listos.map((m) => (
              <Link
                key={m.ruta}
                to={m.ruta}
                className="group flex items-center gap-3 rounded-xl border border-line bg-white p-4 hover:border-brand-200"
              >
                <m.icono className="size-5 text-brand-600" aria-hidden />
                <span className="flex-1 font-medium">{m.etiqueta}</span>
                <ArrowRight className="size-4 text-muted group-hover:text-brand-700" aria-hidden />
              </Link>
            ))}
          </div>
        )}
        {proximos.length > 0 && (
          <Card>
            <p className="border-b border-line px-4 py-3 text-sm font-medium">En desarrollo para su perfil</p>
            <ul className="divide-y divide-line">
              {proximos.map((m) => (
                <li key={m.ruta} className="flex items-start gap-3 px-4 py-3">
                  <m.icono className="mt-0.5 size-4 text-muted" aria-hidden />
                  <div className="flex-1">
                    <p className="text-sm font-medium">
                      {m.etiqueta} <span className="font-normal text-muted">· {m.requisitos}</span>
                    </p>
                    <p className="text-xs text-muted">{m.resumen}</p>
                  </div>
                  <Badge tone="brand">Etapa {m.etapa}</Badge>
                </li>
              ))}
            </ul>
          </Card>
        )}
      </section>
    </div>
  );
};

export default Panel;

// Agenda por sillón y profesional (RF-09), vista día/semana. Un sillón nunca admite dos citas
// superpuestas (RN-03): lo garantiza la restricción EXCLUDE de PostgreSQL, no esta pantalla.
import { ChevronLeft, ChevronRight, Inbox, Plus, ShieldAlert } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Alert, Button, Card, Checkbox, EmptyState, PageHeader, Select, Skeleton } from "../../components/ui";
import { cx } from "../../utils/cx";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { getOdontologos, getSillones } from "../../services/catalogosService";
import { getCitas } from "../../services/citasService";
import { ETIQUETA_ESTADO } from "../../utils/etiquetas";
import {
  fmtDiaCorto,
  fmtDiaLargo,
  fmtDiaMes,
  horaMin,
  inicioDelDia,
  inicioDeSemana,
  mismoDia,
  sumarDias,
} from "../../utils/fechas";
import BandejaMensajes from "./BandejaMensajes";
import {
  ALTO_CUADRICULA,
  ESTILO_ESTADO,
  HORA_FIN,
  HORA_INICIO,
  INACTIVOS,
  minutosDesdeInicio,
  proximoCuarto,
  PX_POR_MIN,
} from "./comun";
import DetalleCita from "./DetalleCita";
import NuevaCita from "./NuevaCita";

// Reparte en carriles las citas que se solapan visualmente (distintos sillones en la vista semanal).
const repartirCarriles = (citas) => {
  const ordenadas = [...citas].sort((a, b) => a.inicio.localeCompare(b.inicio));
  const fines = [];
  const carril = new Map();
  for (const c of ordenadas) {
    const ini = new Date(c.inicio).getTime();
    let i = fines.findIndex((fin) => fin <= ini);
    if (i === -1) i = fines.push(0) - 1;
    fines[i] = new Date(c.fin).getTime();
    carril.set(c.id, i);
  }
  return { carril, carriles: Math.max(1, fines.length) };
};

const Bloque = ({ c, alAbrir, mostrarSillon, carril = 0, carriles = 1 }) => {
  const ini = new Date(c.inicio);
  const fin = new Date(c.fin);
  const top = Math.max(0, minutosDesdeInicio(ini)) * PX_POR_MIN;
  const alto = Math.max(24, ((fin - ini) / 60000) * PX_POR_MIN - 2);
  return (
    <button
      onClick={(ev) => {
        ev.stopPropagation();
        alAbrir();
      }}
      style={{
        top,
        height: alto,
        left: `calc(${(carril / carriles) * 100}% + 2px)`,
        width: `calc(${100 / carriles}% - 4px)`,
      }}
      className={cx(
        "absolute overflow-hidden rounded-md border-l-4 px-1.5 py-1 text-left text-xs shadow-sm transition hover:z-10 hover:shadow-md",
        ESTILO_ESTADO[c.estado],
      )}
      title={`${c.paciente} · ${ETIQUETA_ESTADO[c.estado]}`}
    >
      <div className="flex items-center gap-1 font-semibold">
        {c.alertasCriticas.length > 0 && (
          <ShieldAlert className="size-3.5 shrink-0 text-danger-600" aria-label="Alerta médica crítica" />
        )}
        <span className="truncate">{c.paciente}</span>
      </div>
      <div className="truncate opacity-80">
        {horaMin(ini)}–{horaMin(fin)} {c.procedimiento ?? ""}
      </div>
      {alto > 52 && <div className="truncate opacity-70">{mostrarSillon ? c.sillon : c.odontologo}</div>}
    </button>
  );
};

const RielHoras = () => (
  <div className="relative w-12 shrink-0" style={{ height: ALTO_CUADRICULA }}>
    {Array.from({ length: HORA_FIN - HORA_INICIO }, (_, i) => (
      <span
        key={i}
        className="absolute right-2 -translate-y-1/2 text-[11px] text-muted"
        style={{ top: i * 60 * PX_POR_MIN }}
      >
        {i === 0 ? "" : `${String(HORA_INICIO + i).padStart(2, "0")}:00`}
      </span>
    ))}
  </div>
);

const Columna = ({ children, alPulsar, esHoy }) => {
  const arribaAhora = minutosDesdeInicio(new Date()) * PX_POR_MIN;
  return (
    <div
      className={cx("relative flex-1 border-l border-line", alPulsar && "cursor-cell")}
      style={{ height: ALTO_CUADRICULA, minWidth: 120 }}
      onClick={(e) => {
        if (!alPulsar) return;
        const y = e.clientY - e.currentTarget.getBoundingClientRect().top;
        alPulsar(Math.floor(y / PX_POR_MIN / 15) * 15);
      }}
    >
      {Array.from({ length: (HORA_FIN - HORA_INICIO) * 2 }, (_, i) => (
        <div
          key={i}
          className={cx("absolute inset-x-0 border-t", i % 2 ? "border-dashed border-line/50" : "border-line")}
          style={{ top: i * 30 * PX_POR_MIN }}
        />
      ))}
      {esHoy && arribaAhora > 0 && arribaAhora < ALTO_CUADRICULA && (
        <div
          className="pointer-events-none absolute inset-x-0 z-20 border-t-2 border-danger-600"
          style={{ top: arribaAhora }}
        >
          <span className="absolute -left-1 -top-1.5 size-3 rounded-full bg-danger-600" />
        </div>
      )}
      {children}
    </div>
  );
};

const Agenda = () => {
  const { puede } = useAuth();
  const [params, setParams] = useSearchParams();
  const [vista, setVista] = useState("dia");
  const [cursor, setCursor] = useState(() => inicioDelDia(new Date()));
  const [filtroSillon, setFiltroSillon] = useState("");
  const [filtroOdontologo, setFiltroOdontologo] = useState("");
  const [verInactivas, setVerInactivas] = useState(false);
  const [abiertaId, setAbiertaId] = useState(null);
  const [nueva, setNueva] = useState(null); // { fecha, sillonId?, pacienteId? }
  const [bandeja, setBandeja] = useState(false);
  const puedeEscribir = puede("citas.escribir");

  const sillones = useApi(useCallback(() => getSillones(), []));
  const odontologos = useApi(useCallback(() => getOdontologos(), []));

  // "Agendar" desde la ficha del paciente: /agenda?nuevo=<id>
  useEffect(() => {
    const pid = params.get("nuevo");
    if (pid && puedeEscribir) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- abre el diálogo una sola vez al llegar con ?nuevo
      setNueva({ fecha: proximoCuarto(), pacienteId: pid });
      params.delete("nuevo");
      setParams(params, { replace: true });
    }
  }, [params, setParams, puedeEscribir]);

  const rango = useMemo(() => {
    const desde = vista === "dia" ? cursor : inicioDeSemana(cursor);
    return { desde, hasta: sumarDias(desde, vista === "dia" ? 1 : 7) };
  }, [vista, cursor]);

  const citas = useApi(
    useCallback(
      () =>
        getCitas({
          desde: rango.desde.toISOString(),
          hasta: rango.hasta.toISOString(),
          odontologoId: filtroOdontologo,
          sillonId: vista === "semana" ? filtroSillon : "",
        }),
      [rango, filtroOdontologo, filtroSillon, vista],
    ),
  );
  const recargarCitas = citas.recargar;
  useEffect(() => {
    const t = setInterval(recargarCitas, 60_000);
    return () => clearInterval(t);
  }, [recargarCitas]);

  const visibles = (citas.data ?? []).filter((c) => verInactivas || !INACTIVOS.includes(c.estado));
  const columnasSillon = (sillones.data ?? []).filter((s) => !filtroSillon || s.id === filtroSillon);
  const dias = Array.from({ length: 7 }, (_, i) => sumarDias(rango.desde, i));
  const mover = (n) => setCursor(sumarDias(cursor, vista === "dia" ? n : 7 * n));
  const titulo =
    vista === "dia"
      ? fmtDiaLargo(cursor)
      : `Semana del ${fmtDiaMes(rango.desde)} al ${fmtDiaMes(sumarDias(rango.desde, 6))}`;

  const abrirNueva = (fecha, sillonId) => {
    if (fecha.getTime() < Date.now() - 15 * 60_000) return; // no se agenda en el pasado
    setNueva({ fecha, sillonId });
  };

  return (
    <>
      <PageHeader
        title="Agenda"
        description="Por sillón y profesional. Un sillón nunca admite dos citas a la misma hora (RN-03)."
        actions={
          <>
            <Button variant="secondary" onClick={() => setBandeja(true)}>
              <Inbox className="size-4" /> Mensajes
            </Button>
            {puedeEscribir && (
              <Button
                onClick={() =>
                  abrirNueva(
                    vista === "dia" && !mismoDia(cursor, new Date())
                      ? new Date(cursor.getTime() + 9 * 3600e3)
                      : proximoCuarto(),
                  )
                }
              >
                <Plus className="size-4" /> Nueva cita
              </Button>
            )}
          </>
        }
      />
      <Card className="mb-4 flex flex-wrap items-end gap-3 p-3">
        <div className="flex items-center gap-1">
          <Button variant="secondary" className="px-2" aria-label="Anterior" onClick={() => mover(-1)}>
            <ChevronLeft className="size-4" />
          </Button>
          <Button variant="secondary" onClick={() => setCursor(inicioDelDia(new Date()))}>
            Hoy
          </Button>
          <Button variant="secondary" className="px-2" aria-label="Siguiente" onClick={() => mover(1)}>
            <ChevronRight className="size-4" />
          </Button>
        </div>
        <p className="min-w-48 flex-1 font-medium first-letter:uppercase">{titulo}</p>
        <div role="group" aria-label="Vista" className="flex rounded-lg border border-line p-0.5">
          {["dia", "semana"].map((v) => (
            <button
              key={v}
              onClick={() => setVista(v)}
              aria-pressed={vista === v}
              className={cx(
                "min-h-9 rounded-md px-3 text-sm",
                vista === v ? "bg-brand-700 text-white" : "text-ink hover:bg-canvas",
              )}
            >
              {v === "dia" ? "Día" : "Semana"}
            </button>
          ))}
        </div>
        <Select label="Sillón" value={filtroSillon} onChange={(e) => setFiltroSillon(e.target.value)} className="w-44">
          <option value="">Todos</option>
          {sillones.data?.map((s) => (
            <option key={s.id} value={s.id}>
              {s.nombre}
            </option>
          ))}
        </Select>
        <Select
          label="Odontólogo"
          value={filtroOdontologo}
          onChange={(e) => setFiltroOdontologo(e.target.value)}
          className="w-52"
        >
          <option value="">Todos</option>
          {odontologos.data?.map((o) => (
            <option key={o.id} value={o.id}>
              {o.nombre}
            </option>
          ))}
        </Select>
        <div className="pb-2">
          <Checkbox label="Ver canceladas" checked={verInactivas} onChange={setVerInactivas} />
        </div>
      </Card>

      {(citas.error || sillones.error) && (
        <div className="mb-4">
          <Alert tone="danger">{citas.error || sillones.error}</Alert>
        </div>
      )}
      {sillones.data && sillones.data.length === 0 && (
        <Card>
          <EmptyState title="No hay sillones activos">El administrador debe registrarlos en Catálogos.</EmptyState>
        </Card>
      )}
      {(!sillones.data || !citas.data) && !citas.error ? (
        <Skeleton className="h-96" />
      ) : (
        columnasSillon.length > 0 && (
          <Card className="overflow-x-auto">
            <div className="sticky top-0 z-10 flex border-b border-line bg-white">
              <div className="w-12 shrink-0" />
              {vista === "dia"
                ? columnasSillon.map((s) => (
                    <div
                      key={s.id}
                      className="flex-1 border-l border-line px-2 py-2 text-center"
                      style={{ minWidth: 120 }}
                    >
                      <p className="text-sm font-medium">{s.nombre}</p>
                      <p className="text-[11px] text-muted">{s.consultorio}</p>
                    </div>
                  ))
                : dias.map((d) => (
                    <button
                      key={d.toISOString()}
                      onClick={() => {
                        setCursor(d);
                        setVista("dia");
                      }}
                      className={cx(
                        "flex-1 border-l border-line px-2 py-2 text-center text-sm hover:bg-canvas",
                        mismoDia(d, new Date()) && "font-semibold text-brand-700",
                      )}
                      style={{ minWidth: 120 }}
                    >
                      <span className="first-letter:uppercase">{fmtDiaCorto(d)}</span>
                    </button>
                  ))}
            </div>
            <div className="flex max-h-[70vh] overflow-y-auto pt-2">
              <RielHoras />
              {vista === "dia"
                ? columnasSillon.map((s) => (
                    <Columna
                      key={s.id}
                      esHoy={mismoDia(cursor, new Date())}
                      alPulsar={
                        puedeEscribir
                          ? (m) => abrirNueva(new Date(cursor.getTime() + (HORA_INICIO * 60 + m) * 60000), s.id)
                          : undefined
                      }
                    >
                      {visibles
                        .filter((c) => c.sillonId === s.id)
                        .map((c) => (
                          <Bloque key={c.id} c={c} alAbrir={() => setAbiertaId(c.id)} />
                        ))}
                    </Columna>
                  ))
                : dias.map((d) => {
                    const delDia = visibles.filter((c) => mismoDia(new Date(c.inicio), d));
                    const { carril, carriles } = repartirCarriles(delDia);
                    return (
                      <Columna
                        key={d.toISOString()}
                        esHoy={mismoDia(d, new Date())}
                        alPulsar={
                          puedeEscribir
                            ? (m) =>
                                abrirNueva(
                                  new Date(d.getTime() + (HORA_INICIO * 60 + m) * 60000),
                                  filtroSillon || undefined,
                                )
                            : undefined
                        }
                      >
                        {delDia.map((c) => (
                          <Bloque
                            key={c.id}
                            c={c}
                            mostrarSillon
                            carril={carril.get(c.id)}
                            carriles={carriles}
                            alAbrir={() => setAbiertaId(c.id)}
                          />
                        ))}
                      </Columna>
                    );
                  })}
            </div>
            {visibles.length === 0 && (
              <p className="border-t border-line px-4 py-3 text-center text-sm text-muted">
                Sin citas en este periodo.{puedeEscribir && " Haga clic en un horario libre para agendar."}
              </p>
            )}
          </Card>
        )
      )}

      <div className="mt-3 flex flex-wrap gap-2 text-xs">
        {Object.keys(ESTILO_ESTADO).map((e) => (
          <span key={e} className={cx("rounded border-l-4 px-2 py-0.5", ESTILO_ESTADO[e])}>
            {ETIQUETA_ESTADO[e]}
          </span>
        ))}
        <span className="inline-flex items-center gap-1 text-danger-700">
          <ShieldAlert className="size-3.5" /> Alerta médica crítica
        </span>
      </div>

      {abiertaId && sillones.data && odontologos.data && (
        <DetalleCita
          id={abiertaId}
          sillones={sillones.data}
          odontologos={odontologos.data}
          alCerrar={(cambio) => {
            setAbiertaId(null);
            if (cambio) citas.recargar();
          }}
          alCambiar={citas.recargar}
        />
      )}
      {nueva && sillones.data && odontologos.data && (
        <NuevaCita
          inicial={nueva}
          sillones={sillones.data}
          odontologos={odontologos.data}
          alCerrar={() => setNueva(null)}
          alCrear={(c) => {
            setNueva(null);
            setCursor(inicioDelDia(new Date(c.inicio)));
            citas.recargar();
            setAbiertaId(c.id);
          }}
        />
      )}
      {bandeja && <BandejaMensajes alCerrar={() => setBandeja(false)} />}
    </>
  );
};

export default Agenda;

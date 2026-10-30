// Odontograma 2D interactivo (RF-03): 32 permanentes o 20 temporales, notación FDI o Universal,
// marcado por superficie. Cada cambio es un evento (agregar / quitar): el historial nunca se pierde.
import { History } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import { Alert, Badge, Button, Card, Input, SectionTitle, Skeleton } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useAccion } from "../../hooks/useAccion";
import { useApi } from "../../hooks/useApi";
import { useNotacion } from "../../hooks/useNotacion";
import { getOdontograma, marcarOdontograma } from "../../services/expedienteService";
import { cx } from "../../utils/cx";
import {
  esSuperior,
  etiquetaSuperficie,
  ladoDerecho,
  NOMBRE_SUPERFICIE,
  nombreDiente,
  PERMANENTES_INF,
  PERMANENTES_SUP,
  TEMPORALES_INF,
  TEMPORALES_SUP,
} from "../../utils/dental";
import { fmtFechaHora } from "../../utils/fechas";

const ROJO = "#c0262d";
const AZUL = "#1d5fbf";
const AZUL_CLARO = "#8fb5ea";

// Regiones del dibujo de cada diente (32×32): borde superior, inferior, izquierdo, derecho y centro.
const REGIONES = {
  arriba: "0,0 32,0 22,10 10,10",
  abajo: "10,22 22,22 32,32 0,32",
  izquierda: "0,0 10,10 10,22 0,32",
  derecha: "32,0 32,32 22,22 22,10",
};

// Qué superficie es cada región según la arcada y el lado (la mesial mira a la línea media).
const superficieDeRegion = (diente, region) => {
  if (region === "centro") return "O";
  if (region === "arriba") return esSuperior(diente) ? "V" : "L";
  if (region === "abajo") return esSuperior(diente) ? "L" : "V";
  const mesialDerecha = ladoDerecho(diente);
  if (region === "derecha") return mesialDerecha ? "M" : "D";
  return mesialDerecha ? "D" : "M";
};

const colorSuperficie = (marcas) => {
  if (marcas.some((m) => m.hallazgo === "CARIES")) return ROJO;
  if (marcas.some((m) => m.hallazgo === "RESTAURACION")) return AZUL;
  if (marcas.some((m) => m.hallazgo === "SELLANTE")) return AZUL_CLARO;
  return "white";
};

const Diente = ({ diente, marcas, notacion, seleccion, alElegir }) => {
  const deDiente = marcas.filter((m) => !m.superficie).map((m) => m.hallazgo);
  const superior = esSuperior(diente);
  const elegido = seleccion?.diente === diente;
  const region = (nombre, puntos) => {
    const sup = superficieDeRegion(diente, nombre);
    const activa = elegido && seleccion.superficie === sup;
    const props = {
      fill: colorSuperficie(marcas.filter((m) => m.superficie === sup)),
      stroke: activa ? "#0b4f5c" : "#5b6b77",
      strokeWidth: activa ? 2.2 : 0.8,
      className: "cursor-pointer hover:opacity-80",
      onClick: () => alElegir({ diente, superficie: sup }),
    };
    return (
      <g key={nombre}>
        <title>{`${nombreDiente(diente, notacion)} · ${NOMBRE_SUPERFICIE[etiquetaSuperficie(diente, sup)]}`}</title>
        {puntos ? <polygon points={puntos} {...props} /> : <rect x={10} y={10} width={12} height={12} {...props} />}
      </g>
    );
  };
  return (
    <svg viewBox="-4 -16 40 64" className="w-full" role="group" aria-label={`Diente ${nombreDiente(diente, notacion)}`}>
      <text
        x={16}
        y={superior ? -6 : 46}
        textAnchor="middle"
        className={cx("cursor-pointer select-none text-[9px]", elegido ? "fill-brand-700 font-bold" : "fill-ink")}
        onClick={() => alElegir({ diente, superficie: null })}
      >
        {nombreDiente(diente, notacion)}
      </text>
      {Object.entries(REGIONES).map(([n, p]) => region(n, p))}
      {region("centro")}
      {deDiente.includes("PROTESIS_FIJA") && (
        <circle cx={16} cy={16} r={18} fill="none" stroke={AZUL} strokeWidth={2} />
      )}
      {deDiente.includes("ENDODONCIA_REQUERIDA") && (
        <line x1={16} y1={-2} x2={16} y2={34} stroke={ROJO} strokeWidth={2.5} />
      )}
      {deDiente.includes("ENDODONCIA_REALIZADA") && (
        <line x1={16} y1={-2} x2={16} y2={34} stroke={AZUL} strokeWidth={2.5} />
      )}
      {deDiente.includes("IMPLANTE") && (
        <text x={16} y={20} textAnchor="middle" className="text-[9px] font-bold" fill={AZUL}>
          IMP
        </text>
      )}
      {deDiente.includes("PROTESIS_REMOVIBLE") && (
        <g stroke={AZUL} strokeWidth={1.5}>
          <line x1={-2} y1={superior ? 36 : -4} x2={34} y2={superior ? 36 : -4} />
          <line x1={-2} y1={superior ? 39 : -7} x2={34} y2={superior ? 39 : -7} />
        </g>
      )}
      {(deDiente.includes("AUSENTE") || deDiente.includes("EXODONCIA_INDICADA")) && (
        <g stroke={deDiente.includes("AUSENTE") ? AZUL : ROJO} strokeWidth={2.5}>
          <line x1={-1} y1={-1} x2={33} y2={33} />
          <line x1={33} y1={-1} x2={-1} y2={33} />
        </g>
      )}
      {elegido && seleccion.superficie === null && (
        <rect
          x={-3}
          y={-3}
          width={38}
          height={38}
          fill="none"
          stroke="#0b4f5c"
          strokeWidth={1.5}
          strokeDasharray="3 2"
        />
      )}
    </svg>
  );
};

const Arcada = ({ dientes, marcas, notacion, seleccion, alElegir }) => {
  const mitad = dientes.length / 2;
  const lado = (lista) => (
    <div className="grid flex-1 gap-0.5" style={{ gridTemplateColumns: `repeat(${lista.length}, minmax(0, 1fr))` }}>
      {lista.map((d) => (
        <Diente
          key={d}
          diente={d}
          marcas={marcas.filter((m) => m.diente === d)}
          notacion={notacion}
          seleccion={seleccion}
          alElegir={alElegir}
        />
      ))}
    </div>
  );
  // Dos mitades iguales separadas por la línea media.
  return (
    <div className="flex gap-2">
      {lado(dientes.slice(0, mitad))}
      <div className="w-px shrink-0 border-l-2 border-dashed border-line" aria-hidden />
      {lado(dientes.slice(mitad))}
    </div>
  );
};

const Leyenda = ({ catalogo }) => (
  <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted">
    {catalogo.map((h) => (
      <span key={h.codigo} className="inline-flex items-center gap-1.5">
        <span className="inline-block size-3 rounded-sm" style={{ background: h.color === "ROJO" ? ROJO : AZUL }} />
        {h.etiqueta}
      </span>
    ))}
    <span>· Rojo = hallazgo o tratamiento pendiente · Azul = existente o realizado</span>
  </div>
);

const Odontograma = ({ paciente }) => {
  const { puede } = useAuth();
  const [notacion, setNotacion] = useNotacion();
  const [denticion, setDenticion] = useState(paciente.edad < 12 ? "TEMPORAL" : "PERMANENTE");
  const [seleccion, setSeleccion] = useState(null);
  const [nota, setNota] = useState("");
  const [verHistorial, setVerHistorial] = useState(false);
  const consulta = useApi(useCallback(() => getOdontograma(paciente.id), [paciente.id]));
  const [actual, setActual] = useState(null);
  const marcar = useAccion(marcarOdontograma);
  const datos = actual ?? consulta.data;
  const puedeEscribir = puede("clinica.escribir") && !paciente.anamnesisPendiente;
  const porCodigo = useMemo(() => Object.fromEntries((datos?.catalogo ?? []).map((h) => [h.codigo, h])), [datos]);

  if (consulta.isLoading && !datos) return <Skeleton className="h-80" />;
  if (consulta.error) return <Alert tone="danger">{consulta.error}</Alert>;

  const sup = denticion === "PERMANENTE" ? PERMANENTES_SUP : TEMPORALES_SUP;
  const inf = denticion === "PERMANENTE" ? PERMANENTES_INF : TEMPORALES_INF;
  const vigentes = seleccion
    ? datos.estado.filter(
        (m) =>
          m.diente === seleccion.diente && (seleccion.superficie === null || m.superficie === seleccion.superficie),
      )
    : [];
  const opciones = seleccion
    ? datos.catalogo.filter((h) =>
        seleccion.superficie === null
          ? h.nivel === "DIENTE" && !(h.soloPermanentes && denticion === "TEMPORAL")
          : h.nivel === "SUPERFICIE" && (!h.superficie || h.superficie === seleccion.superficie),
      )
    : [];

  const enviar = async (hallazgo, accion, superficie = seleccion.superficie) => {
    try {
      const r = await marcar.ejecutar(paciente.id, {
        diente: seleccion.diente,
        superficie: superficie ?? "",
        hallazgo,
        accion,
        nota: nota.trim(),
      });
      setActual(r);
      setNota("");
    } catch {
      // el mensaje queda en marcar.error
    }
  };

  const etiquetaSel = seleccion
    ? `Diente ${nombreDiente(seleccion.diente, notacion)}${
        seleccion.superficie
          ? ` · ${NOMBRE_SUPERFICIE[etiquetaSuperficie(seleccion.diente, seleccion.superficie)]}`
          : " · todo el diente"
      }`
    : "";

  return (
    <div className="space-y-4">
      <Card className="p-4">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <div role="group" aria-label="Dentición" className="flex rounded-lg border border-line p-0.5">
            {[
              ["PERMANENTE", "Permanente (32)"],
              ["TEMPORAL", "Temporal (20)"],
            ].map(([v, t]) => (
              <button
                key={v}
                onClick={() => {
                  setDenticion(v);
                  setSeleccion(null);
                }}
                aria-pressed={denticion === v}
                className={cx(
                  "min-h-9 rounded-md px-3 text-sm",
                  denticion === v ? "bg-brand-700 text-white" : "hover:bg-canvas",
                )}
              >
                {t}
              </button>
            ))}
          </div>
          <div role="group" aria-label="Notación" className="flex rounded-lg border border-line p-0.5">
            {["FDI", "UNIVERSAL"].map((n) => (
              <button
                key={n}
                onClick={() => setNotacion(n)}
                aria-pressed={notacion === n}
                className={cx(
                  "min-h-9 rounded-md px-3 text-sm",
                  notacion === n ? "bg-brand-700 text-white" : "hover:bg-canvas",
                )}
              >
                {n === "FDI" ? "FDI" : "Universal"}
              </button>
            ))}
          </div>
        </div>
        <div className="overflow-x-auto">
          <div className="min-w-[640px] space-y-3">
            <p className="text-center text-[11px] uppercase tracking-wider text-muted">
              Superior · derecha del paciente ← → izquierda
            </p>
            <Arcada
              dientes={sup}
              marcas={datos.estado}
              notacion={notacion}
              seleccion={seleccion}
              alElegir={setSeleccion}
            />
            <Arcada
              dientes={inf}
              marcas={datos.estado}
              notacion={notacion}
              seleccion={seleccion}
              alElegir={setSeleccion}
            />
            <p className="text-center text-[11px] uppercase tracking-wider text-muted">Inferior</p>
          </div>
        </div>
        <p className="mt-2 text-xs text-muted">
          Haga clic en una cara del diente para marcar caries, restauración o sellante, o en el número para hallazgos de
          todo el diente. Se guarda siempre en FDI; Universal es sólo la forma de mostrar.
        </p>
        <div className="mt-3 border-t border-line pt-3">
          <Leyenda catalogo={datos.catalogo} />
        </div>
      </Card>

      {seleccion && (
        <Card className="p-4">
          <div className="mb-2 flex items-center justify-between">
            <SectionTitle>{etiquetaSel}</SectionTitle>
            <Button variant="ghost" onClick={() => setSeleccion(null)}>
              Cerrar
            </Button>
          </div>
          {marcar.error && (
            <div className="mb-3">
              <Alert tone="danger">{marcar.error}</Alert>
            </div>
          )}
          <p className="mb-1 text-sm font-medium">Registrado</p>
          {vigentes.length === 0 ? (
            <p className="mb-3 text-sm text-muted">Sin hallazgos.</p>
          ) : (
            <ul className="mb-3 space-y-1">
              {vigentes.map((m) => (
                <li key={`${m.superficie}-${m.hallazgo}`} className="flex flex-wrap items-center gap-2 text-sm">
                  <Badge tone={porCodigo[m.hallazgo]?.color === "ROJO" ? "danger" : "brand"}>
                    {porCodigo[m.hallazgo]?.etiqueta}
                  </Badge>
                  {m.superficie && (
                    <span className="text-muted">{NOMBRE_SUPERFICIE[etiquetaSuperficie(m.diente, m.superficie)]}</span>
                  )}
                  <span className="text-xs text-muted">desde {fmtFechaHora(m.desde)}</span>
                  {puedeEscribir && (
                    <Button
                      variant="ghost"
                      className="min-h-8 px-2 text-xs"
                      loading={marcar.enviando}
                      onClick={() => enviar(m.hallazgo, "QUITAR", m.superficie)}
                    >
                      Quitar (queda en el historial)
                    </Button>
                  )}
                </li>
              ))}
            </ul>
          )}
          {puedeEscribir ? (
            <>
              <p className="mb-1 text-sm font-medium">Agregar</p>
              <div className="mb-2 flex flex-wrap gap-2">
                {opciones.map((h) => (
                  <button
                    key={h.codigo}
                    disabled={marcar.enviando || vigentes.some((m) => m.hallazgo === h.codigo)}
                    onClick={() => enviar(h.codigo, "AGREGAR")}
                    className={cx(
                      "min-h-9 rounded-lg border px-3 text-sm disabled:opacity-40",
                      h.color === "ROJO"
                        ? "border-danger-600/40 text-danger-700 hover:bg-danger-50"
                        : "border-brand-200 text-brand-800 hover:bg-brand-50",
                    )}
                  >
                    {h.etiqueta}
                  </button>
                ))}
              </div>
              <Input label="Nota (opcional)" value={nota} maxLength={500} onChange={(e) => setNota(e.target.value)} />
            </>
          ) : (
            <p className="text-sm text-muted">
              {paciente.anamnesisPendiente
                ? "Registre primero la anamnesis: es obligatoria antes de cualquier registro clínico."
                : "Su perfil sólo puede consultar el odontograma."}
            </p>
          )}
        </Card>
      )}

      <Card className="p-4">
        <button onClick={() => setVerHistorial(!verHistorial)} className="flex items-center gap-2 text-sm font-medium">
          <History className="size-4" /> Historial ({datos.historial.length} eventos)
        </button>
        {verHistorial && (
          <ul className="mt-2 max-h-72 divide-y divide-line overflow-y-auto text-sm">
            {datos.historial.map((e) => (
              <li key={e.id} className="flex flex-wrap gap-x-3 py-1.5">
                <span className="text-muted">{fmtFechaHora(e.registradoEn)}</span>
                <span className={e.accion === "QUITAR" ? "text-muted line-through" : ""}>
                  {nombreDiente(e.diente, notacion)}
                  {e.superficie && ` ${etiquetaSuperficie(e.diente, e.superficie)}`} · {porCodigo[e.hallazgo]?.etiqueta}
                </span>
                <Badge tone={e.accion === "AGREGAR" ? "ok" : "neutral"}>
                  {e.accion === "AGREGAR" ? "Agregado" : "Quitado"}
                </Badge>
                <span className="text-muted">{e.registradoPor}</span>
                {e.nota && <span className="text-muted">— {e.nota}</span>}
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
};

export default Odontograma;

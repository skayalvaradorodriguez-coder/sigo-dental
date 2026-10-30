// Detalle de presupuesto: fases y sesiones, autorización de descuentos (cuatro ojos), presentación
// (se congela con un hash) y decisión del paciente total o parcial por ítem (RF-08).
import { ArrowLeft, CheckCircle2, Pencil, Printer, Send, ShieldCheck } from "lucide-react";
import { useCallback, useState } from "react";
import { Alert, Badge, Button, Card, Checkbox, Skeleton, Textarea } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { useNotacion } from "../../hooks/useNotacion";
import { mensajeDe } from "../../services/http";
import {
  autorizarDescuentos,
  decidirPresupuesto,
  getPresupuesto,
  presentarPresupuesto,
} from "../../services/planesService";
import { etiquetaSuperficie, nombreDiente } from "../../utils/dental";
import { fmtDinero } from "../../utils/dinero";
import { ETIQUETA_PRESUPUESTO, TONO_PRESUPUESTO } from "../../utils/etiquetas";
import { fmtFechaHora } from "../../utils/fechas";
import { escapar, imprimir } from "../../utils/imprimir";
import ModalFirma from "../expediente/ModalFirma";

const TONO_DECISION = { PENDIENTE: "neutral", APROBADO: "ok", RECHAZADO: "danger" };
const ETIQUETA_DECISION = { PENDIENTE: "Pendiente", APROBADO: "Aprobado", RECHAZADO: "No aprobado" };

const porFase = (items) => {
  const m = new Map();
  items.forEach((it) => {
    if (!m.has(it.fase)) m.set(it.fase, { fase: it.fase, nombre: it.faseNombre, items: [] });
    m.get(it.fase).items.push(it);
  });
  return [...m.values()];
};

const ubicacion = (it, notacion) =>
  it.diente
    ? `${nombreDiente(it.diente, notacion)}${it.superficies.length ? ` (${it.superficies.map((s) => etiquetaSuperficie(it.diente, s)).join(", ")})` : ""}`
    : "General";

const imprimirPresupuesto = (p, paciente, notacion) => {
  const filas = p.items
    .map(
      (it) =>
        `<tr><td>${it.fase}</td><td>${it.sesion}</td><td>${escapar(it.codigo)} · ${escapar(it.procedimiento)}</td><td>${escapar(ubicacion(it, notacion))}</td>` +
        `<td class="num">${fmtDinero(it.honorario)}</td><td class="num">${fmtDinero(it.costoLaboratorio)}</td><td class="num">${fmtDinero(it.descuento)}</td><td class="num">${fmtDinero(it.subtotal)}</td></tr>`,
    )
    .join("");
  imprimir(
    p.titulo,
    `<h1>${escapar(p.titulo)}</h1><p class="muted">Paciente: ${escapar(paciente.nombres)} ${escapar(paciente.apellidos)} · ${escapar(paciente.numeroDocumento)} · Elaborado por ${escapar(p.odontologo)} · ${escapar(fmtFechaHora(p.creadoEn))}</p>` +
      `<table><thead><tr><th>Fase</th><th>Sesión</th><th>Procedimiento</th><th>Diente</th><th class="num">Honorario</th><th class="num">Laboratorio</th><th class="num">Descuento</th><th class="num">Subtotal</th></tr></thead><tbody>${filas}</tbody></table>` +
      `<h2>Total: ${fmtDinero(p.totales.total)}</h2>` +
      (p.observaciones ? `<p class="pre">${escapar(p.observaciones)}</p>` : "") +
      `<p class="pie">${p.hashPresentado ? `Presentado el ${escapar(fmtFechaHora(p.presentadoEn))} · huella SHA-256 ${escapar(p.hashPresentado)}` : "Borrador: sujeto a cambios."} Montos sin impuestos ni moneda definidos (configuración de la clínica).</p>`,
  );
};

const DetallePresupuesto = ({ id, paciente, alVolver, alEditar, alCambiar }) => {
  const { puede, usuario } = useAuth();
  const [notacion] = useNotacion();
  const { data: p, isLoading, error, recargar } = useApi(useCallback(() => getPresupuesto(id), [id]));
  const [firmando, setFirmando] = useState(false);
  const [aprobados, setAprobados] = useState(null); // { itemId: bool } mientras se registra la decisión
  const [nota, setNota] = useState("");
  const [errorAccion, setErrorAccion] = useState("");
  const [enviando, setEnviando] = useState(false);

  if (isLoading && !p) return <Skeleton className="h-72" />;
  if (error) return <Alert tone="danger">{error}</Alert>;

  const accion = async (fn) => {
    setErrorAccion("");
    setEnviando(true);
    try {
      await fn();
      recargar();
      alCambiar?.();
    } catch (e) {
      setErrorAccion(mensajeDe(e));
    } finally {
      setEnviando(false);
    }
  };

  const esBorrador = p.estado === "BORRADOR";
  const puedeAutorizar =
    esBorrador && p.descuentosPendientes > 0 && puede("clinica.firmar") && usuario?.id !== p.odontologoId;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Button variant="ghost" onClick={alVolver}>
          <ArrowLeft className="size-4" /> Presupuestos
        </Button>
        <Button variant="secondary" onClick={() => imprimirPresupuesto(p, paciente, notacion)}>
          <Printer className="size-4" /> Imprimir
        </Button>
      </div>

      <Card className="p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold">{p.titulo}</h2>
            <p className="text-xs text-muted">
              {p.odontologo} · creado {fmtFechaHora(p.creadoEn)}
              {p.presentadoEn && <> · presentado {fmtFechaHora(p.presentadoEn)}</>}
            </p>
            {p.observaciones && <p className="mt-2 whitespace-pre-wrap text-sm">{p.observaciones}</p>}
          </div>
          <div className="flex flex-col items-end gap-1">
            <Badge tone={TONO_PRESUPUESTO[p.estado]}>{ETIQUETA_PRESUPUESTO[p.estado]}</Badge>
            {p.integro === true && (
              <span className="flex items-center gap-1 text-xs text-ok-700">
                <ShieldCheck className="size-3.5" /> Íntegro desde la presentación
              </span>
            )}
          </div>
        </div>
        {p.integro === false && (
          <div className="mt-3">
            <Alert tone="danger" title="Integridad comprometida">
              Los ítems no coinciden con la huella guardada al presentarlo.
            </Alert>
          </div>
        )}
      </Card>

      {errorAccion && <Alert tone="danger">{errorAccion}</Alert>}
      {esBorrador && p.descuentosPendientes > 0 && (
        <Alert tone="warn" title={`${p.descuentosPendientes} descuento(s) sobre el tope`}>
          Debe autorizarlos otro odontólogo con su código de verificación antes de presentar el presupuesto.
        </Alert>
      )}

      {porFase(p.items).map((f) => {
        const totFase = p.totales.fases.find((x) => x.fase === f.fase);
        return (
          <Card key={f.fase}>
            <div className="flex items-center justify-between border-b border-line px-4 py-2">
              <p className="text-sm font-semibold">
                Fase {f.fase} · {f.nombre}
              </p>
              <p className="text-sm tabular-nums">{fmtDinero(totFase?.total)}</p>
            </div>
            <ul className="divide-y divide-line">
              {f.items.map((it) => (
                <li key={it.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 text-sm">
                  {aprobados && (
                    <Checkbox
                      label={<span className="sr-only">Aprobar {it.procedimiento}</span>}
                      checked={aprobados[it.id]}
                      onChange={(v) => setAprobados({ ...aprobados, [it.id]: v })}
                    />
                  )}
                  <span className="w-20 text-xs text-muted">Sesión {it.sesion}</span>
                  <span className="min-w-48 flex-1">
                    <span className="font-medium">
                      {it.codigo} · {it.procedimiento}
                    </span>
                    <span className="block text-xs text-muted">{ubicacion(it, notacion)}</span>
                  </span>
                  <span className="text-right text-xs text-muted">
                    Honorario {fmtDinero(it.honorario)}
                    {it.costoLaboratorio !== "0.00" && <> · lab. {fmtDinero(it.costoLaboratorio)}</>}
                    {it.descuento !== "0.00" && (
                      <>
                        {" "}
                        · desc. {fmtDinero(it.descuento)}{" "}
                        {it.requiereAutorizacion &&
                          (it.descuentoAutorizadoPor ? (
                            <Badge tone="ok">autorizado</Badge>
                          ) : (
                            <Badge tone="warn">por autorizar</Badge>
                          ))}
                      </>
                    )}
                  </span>
                  <span className="w-24 text-right font-medium tabular-nums">{fmtDinero(it.subtotal)}</span>
                  {!esBorrador && <Badge tone={TONO_DECISION[it.decision]}>{ETIQUETA_DECISION[it.decision]}</Badge>}
                </li>
              ))}
            </ul>
          </Card>
        );
      })}

      <Card className="p-4 text-sm">
        <p>
          Honorarios {fmtDinero(p.totales.honorarios)} + laboratorio {fmtDinero(p.totales.laboratorio)} − descuentos{" "}
          {fmtDinero(p.totales.descuentos)} = <strong className="tabular-nums">{fmtDinero(p.totales.total)}</strong>
        </p>
        {["APROBADO", "APROBADO_PARCIAL", "RECHAZADO"].includes(p.estado) && (
          <p className="mt-1">
            Aprobado por el paciente: <strong className="tabular-nums">{fmtDinero(p.totales.aprobado)}</strong> ·
            registrado {fmtFechaHora(p.decididoEn)}
            {p.decisionNota && <span className="block text-muted">Nota: {p.decisionNota}</span>}
          </p>
        )}
      </Card>

      {aprobados ? (
        <Card className="space-y-3 p-4">
          <p className="text-sm">
            Marque los procedimientos que el paciente acepta. Los aceptados pasan a <strong>tareas por agendar</strong>.
          </p>
          <Textarea label="Nota (opcional)" rows={2} value={nota} onChange={(e) => setNota(e.target.value)} />
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setAprobados(null)}>
              Cancelar
            </Button>
            <Button
              loading={enviando}
              onClick={() =>
                accion(async () => {
                  await decidirPresupuesto(
                    p.id,
                    p.items.map((it) => ({ itemId: it.id, aprobado: !!aprobados[it.id] })),
                    nota.trim(),
                  );
                  setAprobados(null);
                })
              }
            >
              <CheckCircle2 className="size-4" /> Registrar decisión
            </Button>
          </div>
        </Card>
      ) : (
        <div className="flex flex-wrap justify-end gap-2">
          {esBorrador && puede("presupuestos.escribir") && (
            <Button variant="secondary" onClick={() => alEditar(p)}>
              <Pencil className="size-4" /> Editar
            </Button>
          )}
          {puedeAutorizar && (
            <Button variant="secondary" onClick={() => setFirmando(true)}>
              <ShieldCheck className="size-4" /> Autorizar descuentos
            </Button>
          )}
          {esBorrador && puede("presupuestos.escribir") && (
            <Button
              loading={enviando}
              disabled={p.descuentosPendientes > 0 || p.items.length === 0}
              onClick={() => accion(() => presentarPresupuesto(p.id))}
            >
              <Send className="size-4" /> Presentar al paciente
            </Button>
          )}
          {p.estado === "PRESENTADO" && puede("presupuestos.aprobar") && (
            <Button onClick={() => setAprobados(Object.fromEntries(p.items.map((it) => [it.id, true])))}>
              <CheckCircle2 className="size-4" /> Registrar decisión del paciente
            </Button>
          )}
        </div>
      )}

      {firmando && (
        <ModalFirma
          titulo="Autorizar descuentos"
          descripcion={`Autoriza ${p.descuentosPendientes} descuento(s) sobre el tope del presupuesto elaborado por ${p.odontologo}.`}
          alCerrar={() => setFirmando(false)}
          alFirmar={async ({ codigo }) => {
            await autorizarDescuentos(p.id, codigo);
            setFirmando(false);
            recargar();
          }}
        />
      )}
    </div>
  );
};

export default DetallePresupuesto;

// Detalle de la cita: estado (7 estados del SRS), cancelación con motivo, reprogramación, mensajes e historial.
import { MessageSquare } from "lucide-react";
import { useCallback, useState } from "react";
import { Link } from "react-router-dom";
import { AlertasMedicas } from "../../components/AlertasMedicas";
import { Alert, Badge, Button, Input, Modal, Select, Spinner } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { cambiarEstadoCita, getCita, reprogramarCita } from "../../services/citasService";
import { mensajeDe } from "../../services/http";
import { ETIQUETA_ESTADO, TONO_ESTADO, TRANSICIONES } from "../../utils/etiquetas";
import { combinar, fechaISO, fmtFechaHora, horaMin } from "../../utils/fechas";
import FilaNotificacion from "./FilaNotificacion";

const DetalleCita = ({ id, sillones, odontologos, alCerrar, alCambiar }) => {
  const { puede } = useAuth();
  const consulta = useApi(useCallback(() => getCita(id), [id]));
  const [actual, setActual] = useState(null); // respuesta más reciente de una escritura
  const [cancelando, setCancelando] = useState(false);
  const [motivo, setMotivo] = useState("");
  const [reprogramacion, setReprogramacion] = useState(null);
  const [enviando, setEnviando] = useState(null); // estado o "reprogramar" en curso
  const [error, setError] = useState("");

  const c = actual?.id === id ? actual : consulta.data;
  const puedeEscribir = puede("citas.escribir");
  const empezo = c ? new Date() >= new Date(c.inicio) : false;

  const aplicar = async (clave, accion) => {
    setEnviando(clave);
    setError("");
    try {
      setActual(await accion());
      alCambiar();
      return true;
    } catch (e) {
      setError(mensajeDe(e));
      return false;
    } finally {
      setEnviando(null);
    }
  };

  const cambiarEstado = async (estado, nota) => {
    if (await aplicar(estado, () => cambiarEstadoCita(id, estado, nota))) {
      setCancelando(false);
      setMotivo("");
    }
  };

  const guardarReprogramacion = async () => {
    const r = reprogramacion;
    const ok = await aplicar("reprogramar", () =>
      reprogramarCita(id, {
        inicio: combinar(r.fecha, r.hora).toISOString(),
        duracionMin: Number(r.duracionMin),
        sillonId: r.sillonId,
        odontologoId: r.odontologoId,
      }),
    );
    if (ok) setReprogramacion(null);
  };

  return (
    <Modal open title="Detalle de la cita" onClose={() => alCerrar(false)}>
      {consulta.isLoading && !c && <Spinner />}
      {consulta.error && <Alert tone="danger">{consulta.error}</Alert>}
      {c && (
        <div className="space-y-4">
          <div>
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div>
                {puede("pacientes.leer") ? (
                  <Link
                    to={`/pacientes/${c.pacienteId}`}
                    className="text-lg font-semibold text-brand-800 hover:underline"
                  >
                    {c.paciente}
                  </Link>
                ) : (
                  <p className="text-lg font-semibold">{c.paciente}</p>
                )}
                <p className="text-sm text-muted">
                  {fmtFechaHora(c.inicio)} – {horaMin(c.fin)} · {c.sillon} ({c.consultorio})
                </p>
                <p className="text-sm text-muted">
                  {c.odontologo}
                  {c.procedimiento && ` · ${c.procedimiento}`}
                </p>
              </div>
              <Badge tone={TONO_ESTADO[c.estado]}>{ETIQUETA_ESTADO[c.estado]}</Badge>
            </div>
            {c.motivo && (
              <p className="mt-2 text-sm">
                <span className="text-muted">Motivo:</span> {c.motivo}
              </p>
            )}
            {c.motivoCancelacion && <p className="mt-1 text-sm text-danger-700">Cancelada: {c.motivoCancelacion}</p>}
          </div>

          <AlertasMedicas
            alertas={c.alertasCriticas.map((e) => ({ codigo: e, etiqueta: e, severidad: "CRITICA" }))}
            anamnesisPendiente={c.anamnesisPendiente}
            compacto
          />
          {error && <Alert tone="danger">{error}</Alert>}

          {puedeEscribir && TRANSICIONES[c.estado].length > 0 && !reprogramacion && (
            <div className="space-y-2">
              <p className="text-sm font-medium">Cambiar estado</p>
              <div className="flex flex-wrap gap-2">
                {TRANSICIONES[c.estado]
                  .filter((e) => e !== "CANCELADO")
                  .map((e) => (
                    <Button
                      key={e}
                      variant={e === "NO_ASISTIO" ? "secondary" : "primary"}
                      disabled={(e === "NO_ASISTIO" && !empezo) || (enviando && enviando !== e)}
                      title={e === "NO_ASISTIO" && !empezo ? "Disponible desde la hora de la cita" : undefined}
                      loading={enviando === e}
                      onClick={() => cambiarEstado(e)}
                    >
                      {ETIQUETA_ESTADO[e]}
                    </Button>
                  ))}
                {TRANSICIONES[c.estado].includes("CANCELADO") && !cancelando && (
                  <Button variant="secondary" onClick={() => setCancelando(true)}>
                    Cancelar cita
                  </Button>
                )}
                {["PROGRAMADO", "CONFIRMADO"].includes(c.estado) && (
                  <Button
                    variant="ghost"
                    onClick={() => {
                      const ini = new Date(c.inicio);
                      setReprogramacion({
                        fecha: fechaISO(ini),
                        hora: horaMin(ini),
                        duracionMin: (new Date(c.fin) - ini) / 60000,
                        sillonId: c.sillonId,
                        odontologoId: c.odontologoId,
                      });
                    }}
                  >
                    Reprogramar
                  </Button>
                )}
              </div>
              {cancelando && (
                <div className="flex flex-wrap items-end gap-2 rounded-lg border border-line p-3">
                  <Input
                    label="Motivo de la cancelación"
                    className="flex-1"
                    value={motivo}
                    onChange={(e) => setMotivo(e.target.value)}
                    autoFocus
                  />
                  <Button
                    variant="danger"
                    disabled={motivo.trim().length < 3}
                    loading={enviando === "CANCELADO"}
                    onClick={() => cambiarEstado("CANCELADO", motivo.trim())}
                  >
                    Confirmar cancelación
                  </Button>
                  <Button variant="ghost" onClick={() => setCancelando(false)}>
                    Volver
                  </Button>
                </div>
              )}
            </div>
          )}

          {reprogramacion && (
            <div className="space-y-3 rounded-lg border border-brand-200 p-3">
              <p className="text-sm font-medium">Reprogramar (los recordatorios pendientes se rehacen)</p>
              <div className="grid gap-3 sm:grid-cols-2">
                <Input
                  label="Fecha"
                  type="date"
                  value={reprogramacion.fecha}
                  onChange={(e) => setReprogramacion({ ...reprogramacion, fecha: e.target.value })}
                />
                <div className="grid grid-cols-2 gap-2">
                  <Input
                    label="Hora"
                    type="time"
                    step={300}
                    value={reprogramacion.hora}
                    onChange={(e) => setReprogramacion({ ...reprogramacion, hora: e.target.value })}
                  />
                  <Input
                    label="Minutos"
                    type="number"
                    step={5}
                    min={10}
                    value={reprogramacion.duracionMin}
                    onChange={(e) => setReprogramacion({ ...reprogramacion, duracionMin: e.target.value })}
                  />
                </div>
                <Select
                  label="Sillón"
                  value={reprogramacion.sillonId}
                  onChange={(e) => setReprogramacion({ ...reprogramacion, sillonId: e.target.value })}
                >
                  {sillones.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.nombre} · {s.consultorio}
                    </option>
                  ))}
                </Select>
                <Select
                  label="Odontólogo"
                  value={reprogramacion.odontologoId}
                  onChange={(e) => setReprogramacion({ ...reprogramacion, odontologoId: e.target.value })}
                >
                  {odontologos.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.nombre}
                    </option>
                  ))}
                </Select>
              </div>
              <div className="flex justify-end gap-2">
                <Button
                  variant="ghost"
                  onClick={() => {
                    setReprogramacion(null);
                    setError("");
                  }}
                >
                  Volver
                </Button>
                <Button loading={enviando === "reprogramar"} onClick={guardarReprogramacion}>
                  Guardar nuevo horario
                </Button>
              </div>
            </div>
          )}

          <details className="rounded-lg border border-line p-3" open>
            <summary className="flex cursor-pointer items-center gap-2 text-sm font-medium">
              <MessageSquare className="size-4" /> Mensajes al paciente
            </summary>
            <ul className="mt-1 divide-y divide-line">
              {(c.notificaciones ?? []).map((n) => (
                <FilaNotificacion key={n.id} n={n} />
              ))}
            </ul>
          </details>
          <details className="rounded-lg border border-line p-3">
            <summary className="cursor-pointer text-sm font-medium">Historial de estados</summary>
            <ol className="mt-2 space-y-1 text-sm">
              {(c.eventos ?? []).map((e) => (
                <li key={e.id}>
                  <span className="text-muted">{fmtFechaHora(e.ocurridoEn)}</span> ·{" "}
                  {e.estadoAnterior ? `${ETIQUETA_ESTADO[e.estadoAnterior]} → ` : ""}
                  <strong>{ETIQUETA_ESTADO[e.estadoNuevo]}</strong> · {e.usuario}
                  {e.nota && <span className="text-muted"> — {e.nota}</span>}
                </li>
              ))}
            </ol>
            <p className="mt-2 text-xs text-muted">
              Agendada por {c.creadoPor} el {fmtFechaHora(c.creadoEn)}.
            </p>
          </details>
        </div>
      )}
    </Modal>
  );
};

export default DetalleCita;

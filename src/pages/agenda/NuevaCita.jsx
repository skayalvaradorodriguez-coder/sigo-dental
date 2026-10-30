// Nueva cita (RF-09). Si el sillón está ocupado, el backend responde 409 SILLON_OCUPADO con la cita
// en conflicto (extra.conflicto) y aquí se muestra cuál es.
import { useCallback, useEffect, useRef, useState } from "react";
import { Alert, Button, Input, Modal, Select, Textarea } from "../../components/ui";
import { useApi } from "../../hooks/useApi";
import { getProcedimientos } from "../../services/catalogosService";
import { crearCita } from "../../services/citasService";
import { mensajeDe } from "../../services/http";
import { getPaciente } from "../../services/pacientesService";
import SelectorPaciente from "../../components/SelectorPaciente";
import { getTareas } from "../../services/planesService";
import { combinar, fechaISO, fmtFechaHora, horaMin } from "../../utils/fechas";

const NuevaCita = ({ inicial, sillones, odontologos, alCerrar, alCrear }) => {
  const procedimientos = useApi(useCallback(() => getProcedimientos(), []));
  const [paciente, setPaciente] = useState(null);
  const [f, setF] = useState({
    odontologoId: odontologos[0]?.id ?? "",
    sillonId: inicial.sillonId ?? sillones[0]?.id ?? "",
    procedimientoId: "",
    fecha: fechaISO(inicial.fecha),
    hora: horaMin(inicial.fecha),
    duracionMin: 30,
    motivo: "",
    tareaId: inicial.tareaId ?? "",
  });
  const [error, setError] = useState("");
  const [conflicto, setConflicto] = useState(null);
  const [enviando, setEnviando] = useState(false);

  // Llegada desde la ficha del paciente (?nuevo=<id>): se precarga el paciente.
  useEffect(() => {
    if (!inicial.pacienteId) return;
    getPaciente(inicial.pacienteId)
      .then(setPaciente)
      .catch(() => {});
  }, [inicial.pacienteId]);

  // Tareas pendientes del tratamiento aprobado (RF-08): al elegir una, la cita toma su procedimiento.
  const pacienteId = paciente?.id;
  const tareas = useApi(
    useCallback(() => (pacienteId ? getTareas(pacienteId).catch(() => []) : Promise.resolve([])), [pacienteId]),
  );
  const pendientes = (tareas.data ?? []).filter((t) => t.estado === "PENDIENTE");
  const elegirTarea = (id) => {
    const t = pendientes.find((x) => x.id === id);
    setF((prev) => ({
      ...prev,
      tareaId: id,
      ...(t
        ? { procedimientoId: t.procedimientoId, duracionMin: t.duracionMin, motivo: prev.motivo || t.presupuesto }
        : {}),
    }));
  };
  // Al llegar con ?tarea=, se aplica una sola vez en cuanto cargan las tareas del paciente.
  const precargada = useRef(!inicial.tareaId);
  useEffect(() => {
    if (precargada.current) return;
    const t = (tareas.data ?? []).find((x) => x.id === inicial.tareaId && x.estado === "PENDIENTE");
    if (!t) return;
    precargada.current = true;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- precarga única de la tarea indicada en la URL
    setF((prev) => ({ ...prev, tareaId: t.id, procedimientoId: t.procedimientoId, duracionMin: t.duracionMin }));
  }, [tareas.data, inicial.tareaId]);

  const enviar = async (e) => {
    e.preventDefault();
    setError("");
    setConflicto(null);
    if (!paciente) return setError("Seleccione un paciente.");
    if (!f.odontologoId || !f.sillonId) return setError("Seleccione odontólogo y sillón.");
    if (!f.fecha || !f.hora) return setError("Indique fecha y hora.");
    if (Number(f.duracionMin) % 5 !== 0 || Number(f.duracionMin) < 10)
      return setError("La duración debe ser múltiplo de 5 y de al menos 10 minutos.");
    setEnviando(true);
    try {
      const c = await crearCita({
        pacienteId: paciente.id,
        odontologoId: f.odontologoId,
        sillonId: f.sillonId,
        procedimientoId: f.procedimientoId,
        tareaId: f.tareaId,
        inicio: combinar(f.fecha, f.hora).toISOString(),
        duracionMin: Number(f.duracionMin),
        motivo: f.motivo.trim(),
      });
      alCrear(c);
    } catch (err) {
      setError(mensajeDe(err));
      setConflicto(err.code === "SILLON_OCUPADO" ? err.extra?.conflicto : null);
    } finally {
      setEnviando(false);
    }
  };

  return (
    <Modal open title="Nueva cita" onClose={alCerrar}>
      <form onSubmit={enviar} noValidate className="space-y-4">
        {error && (
          <Alert tone="danger" title={conflicto ? "Sillón ocupado" : undefined}>
            {conflicto
              ? `Ya hay una cita de ${conflicto.paciente} de ${fmtFechaHora(conflicto.inicio)} a ${horaMin(conflicto.fin)}. Elija otro horario o sillón.`
              : error}
          </Alert>
        )}
        <SelectorPaciente
          valor={paciente}
          alCambiar={(p) => {
            setPaciente(p);
            setF((prev) => ({ ...prev, tareaId: "" }));
          }}
        />
        {pendientes.length > 0 && (
          <Select
            label="Tarea del tratamiento aprobado (opcional)"
            value={f.tareaId}
            onChange={(e) => elegirTarea(e.target.value)}
          >
            <option value="">Cita sin tarea del presupuesto</option>
            {pendientes.map((t) => (
              <option key={t.id} value={t.id}>
                Fase {t.fase} · S{t.sesion} · {t.procedimiento}
                {t.diente ? ` · diente ${t.diente}` : ""}
              </option>
            ))}
          </Select>
        )}
        <div className="grid gap-3 sm:grid-cols-2">
          <Select
            label="Odontólogo"
            value={f.odontologoId}
            onChange={(e) => setF({ ...f, odontologoId: e.target.value })}
          >
            {odontologos.map((o) => (
              <option key={o.id} value={o.id}>
                {o.nombre}
              </option>
            ))}
          </Select>
          <Select label="Sillón" value={f.sillonId} onChange={(e) => setF({ ...f, sillonId: e.target.value })}>
            {sillones.map((s) => (
              <option key={s.id} value={s.id}>
                {s.nombre} · {s.consultorio}
              </option>
            ))}
          </Select>
          <Select
            label="Procedimiento"
            value={f.procedimientoId}
            onChange={(e) => {
              const p = procedimientos.data?.find((x) => x.id === e.target.value);
              setF({ ...f, procedimientoId: e.target.value, duracionMin: p?.duracionMin ?? f.duracionMin });
            }}
            disabled={!!f.tareaId}
            className="sm:col-span-2"
          >
            <option value="">Sin procedimiento definido</option>
            {procedimientos.data?.map((p) => (
              <option key={p.id} value={p.id}>
                {p.nombre} ({p.duracionMin} min)
              </option>
            ))}
          </Select>
          <Input label="Fecha" type="date" value={f.fecha} onChange={(e) => setF({ ...f, fecha: e.target.value })} />
          <div className="grid grid-cols-2 gap-3">
            <Input
              label="Hora"
              type="time"
              step={300}
              value={f.hora}
              onChange={(e) => setF({ ...f, hora: e.target.value })}
            />
            <Input
              label="Minutos"
              type="number"
              min={10}
              max={480}
              step={5}
              value={f.duracionMin}
              onChange={(e) => setF({ ...f, duracionMin: e.target.value })}
            />
          </div>
        </div>
        <Textarea
          label="Motivo (opcional)"
          rows={2}
          value={f.motivo}
          onChange={(e) => setF({ ...f, motivo: e.target.value })}
        />
        <div className="flex justify-end gap-2 border-t border-line pt-4">
          <Button type="button" variant="secondary" onClick={alCerrar}>
            Cancelar
          </Button>
          <Button type="submit" loading={enviando}>
            Agendar
          </Button>
        </div>
      </form>
    </Modal>
  );
};

export default NuevaCita;

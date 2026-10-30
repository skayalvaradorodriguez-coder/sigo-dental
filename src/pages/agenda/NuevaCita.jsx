// Nueva cita (RF-09). Si el sillón está ocupado, el backend responde 409 SILLON_OCUPADO con la cita
// en conflicto (extra.conflicto) y aquí se muestra cuál es.
import { Search, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { AlertasMedicas } from "../../components/AlertasMedicas";
import { Alert, Button, Input, Modal, Select, Textarea } from "../../components/ui";
import { useApi } from "../../hooks/useApi";
import { getProcedimientos } from "../../services/catalogosService";
import { crearCita } from "../../services/citasService";
import { mensajeDe } from "../../services/http";
import { buscarPacientes, getPaciente } from "../../services/pacientesService";
import { combinar, fechaISO, fmtFechaHora, horaMin } from "../../utils/fechas";

const SelectorPaciente = ({ valor, alCambiar }) => {
  const [texto, setTexto] = useState("");
  const [termino, setTermino] = useState("");
  useEffect(() => {
    const t = setTimeout(() => setTermino(texto.trim()), 300);
    return () => clearTimeout(t);
  }, [texto]);
  const buscar = useCallback(
    () => (termino.length >= 2 ? buscarPacientes({ q: termino, limit: 8 }) : Promise.resolve(null)),
    [termino],
  );
  const { data, isLoading } = useApi(buscar);

  if (valor)
    return (
      <div className="space-y-2">
        <p className="text-sm font-medium">Paciente</p>
        <div className="flex items-center justify-between rounded-lg border border-brand-200 bg-brand-50 px-3 py-2">
          <span className="text-sm font-medium">
            {valor.nombres} {valor.apellidos} <span className="font-normal text-muted">· {valor.numeroDocumento}</span>
          </span>
          <button
            type="button"
            onClick={() => alCambiar(null)}
            className="rounded p-1 text-muted hover:bg-white"
            aria-label="Cambiar paciente"
          >
            <X className="size-4" />
          </button>
        </div>
        <AlertasMedicas alertas={valor.alertas} anamnesisPendiente={valor.anamnesisPendiente} compacto />
      </div>
    );

  return (
    <div>
      <label htmlFor="selector-paciente" className="mb-1 block text-sm font-medium">
        Paciente
      </label>
      <div className="relative">
        <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted" aria-hidden />
        <input
          id="selector-paciente"
          autoFocus
          value={texto}
          onChange={(e) => setTexto(e.target.value)}
          placeholder="Escriba nombre o documento (mín. 2 letras)"
          className="min-h-11 w-full rounded-lg border border-line pl-9 pr-3 text-sm"
        />
      </div>
      {termino.length >= 2 && (
        <ul className="mt-1 max-h-56 overflow-y-auto rounded-lg border border-line">
          {isLoading && <li className="px-3 py-2 text-sm text-muted">Buscando…</li>}
          {data?.data.length === 0 && (
            <li className="px-3 py-2 text-sm text-muted">
              Sin resultados. Registre primero al paciente en “Pacientes”.
            </li>
          )}
          {data?.data.map((p) => (
            <li key={p.id}>
              <button
                type="button"
                onClick={() => alCambiar(p)}
                className="flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm hover:bg-canvas"
              >
                <span>
                  {p.apellidos}, {p.nombres} <span className="text-muted">· {p.numeroDocumento}</span>
                </span>
                {p.alertas?.some((a) => a.severidad === "CRITICA") && (
                  <span className="text-xs font-semibold text-danger-700">Alerta crítica</span>
                )}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};

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
        <SelectorPaciente valor={paciente} alCambiar={setPaciente} />
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

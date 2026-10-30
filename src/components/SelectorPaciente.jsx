// Buscador de pacientes con sus alertas (agenda y caja).
import { Search, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useApi } from "../hooks/useApi";
import { buscarPacientes } from "../services/pacientesService";
import { AlertasMedicas } from "./AlertasMedicas";

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

export default SelectorPaciente;

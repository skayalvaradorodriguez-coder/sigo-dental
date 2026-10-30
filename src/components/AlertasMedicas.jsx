import { AlertTriangle, ClipboardX, ShieldAlert } from "lucide-react";
import { cx } from "../utils/cx";

// Alertas médicas de alta visibilidad (RF-02). Las CRÍTICAS van en un bloque rojo con borde
// grueso e ícono, siempre arriba en la cabecera del paciente.
// alertas: [{ codigo, etiqueta, severidad: "CRITICA" | "ADVERTENCIA" }]
export const AlertasMedicas = ({ alertas = [], anamnesisPendiente, compacto }) => {
  const criticas = alertas.filter((a) => a.severidad === "CRITICA");
  const advertencias = alertas.filter((a) => a.severidad === "ADVERTENCIA");
  if (!criticas.length && !advertencias.length && !anamnesisPendiente) return null;

  return (
    <div className="space-y-2">
      {criticas.length > 0 && (
        <div
          role="alert"
          className={cx(
            "flex items-start gap-3 rounded-lg border-2 border-danger-600 bg-danger-50 text-danger-700",
            compacto ? "p-2" : "p-3",
          )}
        >
          <ShieldAlert className="mt-0.5 size-5 shrink-0" aria-hidden />
          <div>
            <p className="text-xs font-bold uppercase tracking-wider">Alerta médica crítica</p>
            <ul className="mt-0.5 flex flex-wrap gap-x-4 gap-y-1 font-semibold">
              {criticas.map((a) => (
                <li key={a.codigo + a.etiqueta}>{a.etiqueta}</li>
              ))}
            </ul>
          </div>
        </div>
      )}
      {advertencias.length > 0 && (
        <div className="flex items-start gap-3 rounded-lg border border-warn-700/20 bg-warn-50 p-2.5 text-sm text-warn-700">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden />
          <p>
            <span className="font-medium">Antecedentes: </span>
            {advertencias.map((a) => a.etiqueta).join(" · ")}
          </p>
        </div>
      )}
      {anamnesisPendiente && (
        <div className="flex items-start gap-3 rounded-lg border border-warn-700/20 bg-warn-50 p-2.5 text-sm text-warn-700">
          <ClipboardX className="mt-0.5 size-4 shrink-0" aria-hidden />
          <p>
            <span className="font-medium">Anamnesis pendiente.</span> Debe registrarse antes de cualquier procedimiento
            clínico.
          </p>
        </div>
      )}
    </div>
  );
};

// Indicador compacto para tablas y bloques de agenda.
export const PuntoAlerta = ({ criticas, pendiente }) => {
  if (criticas > 0)
    return (
      <span
        title="Alerta médica crítica"
        className="inline-flex items-center gap-1 rounded bg-danger-600 px-1.5 py-0.5 text-[11px] font-bold text-white"
      >
        <ShieldAlert className="size-3" aria-hidden /> {criticas}
      </span>
    );
  if (pendiente)
    return (
      <span
        title="Anamnesis pendiente"
        className="inline-flex items-center gap-1 rounded bg-warn-50 px-1.5 py-0.5 text-[11px] font-medium text-warn-700"
      >
        <ClipboardX className="size-3" aria-hidden /> Anamnesis
      </span>
    );
  return null;
};

import { fmtFechaHora } from "../../utils/fechas";

// Lista de adendas firmadas (correcciones que no alteran el registro original).
const Adendas = ({ adendas }) =>
  adendas.length > 0 && (
    <div className="mt-3 space-y-2 border-t border-line pt-3">
      <p className="text-xs font-semibold uppercase tracking-wider text-muted">Adendas</p>
      {adendas.map((a) => (
        <div key={a.id} className="rounded-lg border-l-4 border-brand-500 bg-brand-50/60 px-3 py-2 text-sm">
          <p className="whitespace-pre-wrap">{a.texto}</p>
          <p className="mt-1 text-xs text-muted">
            {a.autor} · {fmtFechaHora(a.creadoEn)} · firma{" "}
            <code className="font-mono">{a.hashFirma.slice(0, 12)}…</code>
          </p>
        </div>
      ))}
    </div>
  );

export default Adendas;

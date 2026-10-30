import { AlertTriangle, CheckCircle2, Info, Loader2, X, XCircle } from "lucide-react";
import { forwardRef, useEffect, useId, useRef } from "react";
import { cx } from "../utils/cx";
const VARIANTS = {
  primary: "bg-brand-700 text-white hover:bg-brand-800 disabled:bg-brand-700/50",
  secondary: "bg-white text-ink border border-line hover:bg-canvas disabled:text-muted",
  ghost: "text-brand-700 hover:bg-brand-50 disabled:text-muted",
  danger: "bg-danger-600 text-white hover:bg-danger-700 disabled:bg-danger-600/50",
};
export function Button({ variant = "primary", loading, children, className, disabled, ...rest }) {
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cx(
        "inline-flex min-h-10 items-center justify-center gap-2 rounded-lg px-4 text-sm font-medium transition-colors disabled:cursor-not-allowed",
        VARIANTS[variant],
        className,
      )}
    >
      {loading && <Loader2 className="size-4 animate-spin" aria-hidden />}
      {children}
    </button>
  );
}
export const Input = forwardRef(function Input({ label, error, hint, className, id, ...rest }, ref) {
  const auto = useId();
  const inputId = id ?? auto;
  return (
    <div className={className}>
      <label htmlFor={inputId} className="mb-1 block text-sm font-medium text-ink">
        {label}
      </label>
      <input
        ref={ref}
        id={inputId}
        aria-invalid={!!error || undefined}
        aria-describedby={error || hint ? `${inputId}-msg` : undefined}
        className={cx(
          "block min-h-11 w-full rounded-lg border bg-white px-3 text-[15px] text-ink placeholder:text-muted/70",
          error ? "border-danger-600" : "border-line focus:border-brand-500",
        )}
        {...rest}
      />
      {(error || hint) && (
        <p id={`${inputId}-msg`} className={cx("mt-1 text-xs", error ? "text-danger-700" : "text-muted")}>
          {error ?? hint}
        </p>
      )}
    </div>
  );
});
export function Select({ label, error, children, className, ...rest }) {
  const id = useId();
  return (
    <div className={className}>
      <label htmlFor={id} className="mb-1 block text-sm font-medium text-ink">
        {label}
      </label>
      <select
        id={id}
        aria-invalid={!!error || undefined}
        className={cx(
          "block min-h-11 w-full rounded-lg border bg-white px-3 text-[15px]",
          error ? "border-danger-600" : "border-line",
        )}
        {...rest}
      >
        {children}
      </select>
      {error && <p className="mt-1 text-xs text-danger-700">{error}</p>}
    </div>
  );
}
// ------------------------------------------------------------------ Layout bits
export function Card({ children, className }) {
  return <section className={cx("rounded-xl border border-line bg-white", className)}>{children}</section>;
}
export function PageHeader({ title, description, actions }) {
  return (
    <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight text-ink">{title}</h1>
        {description && <p className="mt-1 max-w-2xl text-sm text-muted">{description}</p>}
      </div>
      {actions && <div className="flex gap-2">{actions}</div>}
    </header>
  );
}
const TONES = {
  neutral: "bg-canvas text-muted border-line",
  brand: "bg-brand-50 text-brand-700 border-brand-100",
  ok: "bg-ok-50 text-ok-700 border-ok-700/15",
  warn: "bg-warn-50 text-warn-700 border-warn-700/15",
  danger: "bg-danger-50 text-danger-700 border-danger-600/20",
};
export function Badge({ tone = "neutral", children }) {
  return (
    <span
      className={cx("inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-medium", TONES[tone])}
    >
      {children}
    </span>
  );
}
const ALERT_ICON = { info: Info, ok: CheckCircle2, warn: AlertTriangle, danger: XCircle };
export function Alert({ tone = "info", title, children }) {
  const Icon = ALERT_ICON[tone];
  const style = {
    info: "bg-brand-50 border-brand-100 text-brand-800",
    ok: TONES.ok,
    warn: TONES.warn,
    danger: TONES.danger,
  }[tone];
  return (
    <div
      role={tone === "danger" ? "alert" : "status"}
      className={cx("flex gap-3 rounded-lg border p-3 text-sm", style)}
    >
      <Icon className="mt-0.5 size-4 shrink-0" aria-hidden />
      <div>
        {title && <p className="font-medium">{title}</p>}
        {children && <div className={title ? "mt-0.5 opacity-90" : ""}>{children}</div>}
      </div>
    </div>
  );
}
export function Spinner({ label = "Cargando…" }) {
  return (
    <div role="status" className="flex items-center justify-center gap-2 py-12 text-sm text-muted">
      <Loader2 className="size-5 animate-spin" aria-hidden />
      {label}
    </div>
  );
}
export function Skeleton({ className }) {
  return <div aria-hidden className={cx("animate-pulse rounded-md bg-line/60", className)} />;
}
export function EmptyState({ icon, title, children }) {
  return (
    <div className="flex flex-col items-center px-6 py-14 text-center">
      {icon && <div className="mb-3 text-brand-500">{icon}</div>}
      <p className="font-medium text-ink">{title}</p>
      {children && <div className="mt-1 max-w-md text-sm text-muted">{children}</div>}
    </div>
  );
}
// ------------------------------------------------------------------ Modal
export function Modal({ open, title, onClose, children, footer }) {
  const ref = useRef(null);
  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) d.showModal?.();
    if (!open && d.open) d.close?.();
  }, [open]);
  if (!open) return null;
  return (
    <dialog
      ref={ref}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      aria-labelledby="modal-title"
      className="m-auto w-[min(32rem,calc(100vw-2rem))] rounded-xl border border-line p-0 shadow-xl backdrop:bg-ink/40"
    >
      <div className="flex items-center justify-between border-b border-line px-5 py-4">
        <h2 id="modal-title" className="font-semibold">
          {title}
        </h2>
        <button onClick={onClose} className="rounded p-1 text-muted hover:bg-canvas" aria-label="Cerrar">
          <X className="size-4" />
        </button>
      </div>
      <div className="px-5 py-4">{children}</div>
      {footer && <div className="flex justify-end gap-2 border-t border-line bg-canvas/60 px-5 py-3">{footer}</div>}
    </dialog>
  );
}
// ------------------------------------------------------------------ Etapa 2
export function Textarea({ label, error, hint, className, ...rest }) {
  const id = useId();
  return (
    <div className={className}>
      <label htmlFor={id} className="mb-1 block text-sm font-medium text-ink">
        {label}
      </label>
      <textarea
        id={id}
        rows={3}
        aria-invalid={!!error || undefined}
        className={cx(
          "block w-full rounded-lg border bg-white px-3 py-2 text-[15px]",
          error ? "border-danger-600" : "border-line",
        )}
        {...rest}
      />
      {(error || hint) && (
        <p className={cx("mt-1 text-xs", error ? "text-danger-700" : "text-muted")}>{error ?? hint}</p>
      )}
    </div>
  );
}
/** Pregunta Sí/No que DEBE responderse (null = sin responder), útil en la anamnesis. */
export function YesNo({ label, value, onChange, error, danger }) {
  const name = useId();
  return (
    <fieldset>
      <legend className="mb-1 text-sm font-medium text-ink">{label}</legend>
      <div className="flex gap-2">
        {[
          { v: false, t: "No" },
          { v: true, t: "Sí" },
        ].map((o) => (
          <label
            key={o.t}
            className={cx(
              "flex min-h-10 cursor-pointer items-center gap-2 rounded-lg border px-4 text-sm",
              value === o.v
                ? o.v && danger
                  ? "border-danger-600 bg-danger-50 text-danger-700"
                  : "border-brand-500 bg-brand-50 text-brand-800"
                : "border-line bg-white",
            )}
          >
            <input
              type="radio"
              name={name}
              checked={value === o.v}
              onChange={() => onChange(o.v)}
              className="accent-brand-700"
            />
            {o.t}
          </label>
        ))}
      </div>
      {error && <p className="mt-1 text-xs text-danger-700">{error}</p>}
    </fieldset>
  );
}
export function Checkbox({ label, checked, onChange, hint }) {
  return (
    <label className="flex cursor-pointer items-start gap-2 text-sm">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-0.5 size-4 accent-brand-700"
      />
      <span>
        {label}
        {hint && <span className="block text-xs text-muted">{hint}</span>}
      </span>
    </label>
  );
}
export function Tabs({ tabs, value, onChange }) {
  return (
    <div role="tablist" className="flex gap-1 overflow-x-auto border-b border-line">
      {tabs.map((t) => (
        <button
          key={t.id}
          role="tab"
          aria-selected={value === t.id}
          onClick={() => onChange(t.id)}
          className={cx(
            "-mb-px min-h-10 whitespace-nowrap border-b-2 px-4 text-sm font-medium",
            value === t.id ? "border-brand-700 text-brand-800" : "border-transparent text-muted hover:text-ink",
          )}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}
export function SectionTitle({ children }) {
  return <h3 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted">{children}</h3>;
}

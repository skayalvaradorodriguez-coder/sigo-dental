// Lista de cobros con su comprobante: reintento si la facturación falló, anulación con motivo e impresión.
import { Ban, Printer, RefreshCw } from "lucide-react";
import { useState } from "react";
import { Alert, Badge, Button, Card, EmptyState, Modal, Textarea } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { anularCobro, reintentarComprobante } from "../../services/cajaService";
import { mensajeDe } from "../../services/http";
import { fmtDinero } from "../../utils/dinero";
import { imprimirCobro } from "../../utils/documentos";
import { ETIQUETA_COMPROBANTE, ETIQUETA_MEDIO } from "../../utils/etiquetas";
import { fmtFechaHora } from "../../utils/fechas";

const ultimo = (cb, tipo) => [...cb.comprobantes].reverse().find((x) => x.tipo === tipo);

export const EstadoComprobante = ({ cb }) => {
  const f = ultimo(cb, "FACTURA");
  const nc = ultimo(cb, "NOTA_CREDITO");
  return (
    <span className="flex flex-wrap gap-1">
      {!f && <Badge tone="warn">Sin comprobante</Badge>}
      {f && (
        <Badge tone={f.estado === "AUTORIZADO" ? "ok" : "danger"}>
          {ETIQUETA_COMPROBANTE.FACTURA}{" "}
          {f.estado === "AUTORIZADO" ? f.numero : f.estado === "ERROR" ? "sin respuesta" : "rechazada"}
          {f.simulado && " · SIMULADO"}
        </Badge>
      )}
      {nc && (
        <Badge tone={nc.estado === "AUTORIZADO" ? "neutral" : "danger"}>
          {ETIQUETA_COMPROBANTE.NOTA_CREDITO} {nc.estado === "AUTORIZADO" ? nc.numero : "pendiente"}
        </Badge>
      )}
      {cb.anulacion && <Badge tone="danger">Anulado</Badge>}
    </span>
  );
};

const necesitaReintento = (cb) => {
  const f = ultimo(cb, "FACTURA");
  if (!cb.anulacion) return !f || f.estado !== "AUTORIZADO";
  const nc = ultimo(cb, "NOTA_CREDITO");
  return f?.estado === "AUTORIZADO" && (!nc || nc.estado !== "AUTORIZADO");
};

const ListaCobros = ({ cobros, puedeAnular, alCambiar, mostrarPaciente = true }) => {
  const { puede } = useAuth();
  const [anulando, setAnulando] = useState(null);
  const [motivo, setMotivo] = useState("");
  const [error, setError] = useState("");
  const [enviando, setEnviando] = useState("");

  const accion = async (id, fn) => {
    setError("");
    setEnviando(id);
    try {
      await fn();
      alCambiar?.();
    } catch (e) {
      setError(mensajeDe(e));
    } finally {
      setEnviando("");
    }
  };

  if (!cobros.length)
    return (
      <Card>
        <EmptyState title="Sin cobros" />
      </Card>
    );
  return (
    <>
      {error && <Alert tone="danger">{error}</Alert>}
      <Card>
        <ul className="divide-y divide-line">
          {cobros.map((cb) => (
            <li
              key={cb.id}
              className={`flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 text-sm ${cb.anulacion ? "opacity-70" : ""}`}
            >
              <span className="w-16 font-mono text-xs text-muted">N.º {cb.numero}</span>
              <span className="min-w-48 flex-1">
                {mostrarPaciente && <span className="font-medium">{cb.paciente}</span>}
                <span className="block text-xs text-muted">
                  {fmtFechaHora(cb.registradoEn)} · {cb.conceptos.map((c) => c.descripcion).join(", ")}
                </span>
                <span className="block text-xs text-muted">
                  {cb.pagos.map((p) => `${ETIQUETA_MEDIO[p.medio]} ${fmtDinero(p.monto)}`).join(" + ")}
                </span>
                {cb.anulacion && <span className="block text-xs text-danger-700">Anulado: {cb.anulacion.motivo}</span>}
              </span>
              <span className={`tabular-nums font-medium ${cb.anulacion ? "line-through" : ""}`}>
                {fmtDinero(cb.total)}
              </span>
              <EstadoComprobante cb={cb} />
              <span className="flex gap-1">
                {necesitaReintento(cb) && puede("facturacion.emitir") && (
                  <Button
                    variant="secondary"
                    loading={enviando === cb.id}
                    onClick={() => accion(cb.id, () => reintentarComprobante(cb.id))}
                  >
                    <RefreshCw className="size-4" /> Emitir comprobante
                  </Button>
                )}
                <Button variant="ghost" aria-label={`Imprimir cobro ${cb.numero}`} onClick={() => imprimirCobro(cb)}>
                  <Printer className="size-4" />
                </Button>
                {puedeAnular && !cb.anulacion && (
                  <Button variant="ghost" aria-label={`Anular cobro ${cb.numero}`} onClick={() => setAnulando(cb)}>
                    <Ban className="size-4" />
                  </Button>
                )}
              </span>
            </li>
          ))}
        </ul>
      </Card>
      {anulando && (
        <Modal open title={`Anular cobro N.º ${anulando.numero}`} onClose={() => setAnulando(null)}>
          <div className="space-y-3">
            <p className="text-sm">
              {anulando.paciente} · {fmtDinero(anulando.total)}. El cobro no se borra: queda marcado como anulado y, si
              tenía factura autorizada, se emite una nota de crédito. Devuelva el dinero al paciente.
            </p>
            {error && <Alert tone="danger">{error}</Alert>}
            <Textarea label="Motivo" rows={2} value={motivo} onChange={(e) => setMotivo(e.target.value)} />
            <div className="flex justify-end gap-2">
              <Button variant="secondary" onClick={() => setAnulando(null)}>
                Cancelar
              </Button>
              <Button
                variant="danger"
                loading={enviando === anulando.id}
                onClick={() =>
                  accion(anulando.id, async () => {
                    await anularCobro(anulando.id, motivo.trim());
                    setAnulando(null);
                    setMotivo("");
                  })
                }
              >
                Anular
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </>
  );
};

export default ListaCobros;

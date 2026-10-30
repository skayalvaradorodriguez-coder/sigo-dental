// Caja del recepcionista: apertura, cobros y cierre CIEGO (RN-02). En ningún momento se muestran
// totales de la caja: el teórico lo calcula el servidor al cerrar y sólo lo ve el Auditor.
import { Lock, Printer, Wallet } from "lucide-react";
import { useCallback, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Alert, Button, Card, Input, Modal, Skeleton, Tabs, Textarea } from "../../components/ui";
import { useApi } from "../../hooks/useApi";
import { abrirCaja, cerrarCaja, getMiCaja, getMisCobros } from "../../services/cajaService";
import { mensajeDe } from "../../services/http";
import { aCentavos, deCentavos, fmtDinero } from "../../utils/dinero";
import { imprimirCobro } from "../../utils/documentos";
import { fmtFechaHora } from "../../utils/fechas";
import FormularioCobro from "./FormularioCobro";
import ListaCobros, { EstadoComprobante } from "./ListaCobros";

const Apertura = ({ alAbrir, aviso }) => {
  const [fondo, setFondo] = useState("");
  const [error, setError] = useState("");
  const [enviando, setEnviando] = useState(false);
  return (
    <Card className="max-w-md space-y-3 p-5">
      {aviso && <Alert tone="ok">{aviso}</Alert>}
      <p className="font-medium">Su caja está cerrada</p>
      <p className="text-sm text-muted">Cuente el fondo con el que empieza (billetes y monedas para dar cambio).</p>
      {error && <Alert tone="danger">{error}</Alert>}
      <Input
        label="Fondo inicial"
        inputMode="decimal"
        placeholder="0.00"
        value={fondo}
        onChange={(e) => setFondo(e.target.value)}
      />
      <Button
        loading={enviando}
        onClick={async () => {
          if (aCentavos(fondo || "0") === null) return setError("Monto inválido.");
          setEnviando(true);
          setError("");
          try {
            await abrirCaja(deCentavos(aCentavos(fondo || "0")));
            alAbrir();
          } catch (e) {
            setError(mensajeDe(e));
          } finally {
            setEnviando(false);
          }
        }}
      >
        <Wallet className="size-4" /> Abrir caja
      </Button>
    </Card>
  );
};

const CierreCiego = ({ alCerrar, alCancelar }) => {
  const [f, setF] = useState({ efectivo: "", tarjeta: "", transferencia: "", nota: "" });
  const [confirmar, setConfirmar] = useState(false);
  const [error, setError] = useState("");
  const [enviando, setEnviando] = useState(false);
  const montos = ["efectivo", "tarjeta", "transferencia"];
  const invalido = montos.some((k) => aCentavos(f[k] || "0") === null);
  return (
    <Modal open title="Cerrar caja (cierre ciego)" onClose={alCancelar}>
      <div className="space-y-3">
        <Alert tone="info">
          Cuente lo que hay en la caja <strong>sin consultar el sistema</strong>: efectivo (incluido el fondo inicial),
          vouchers de tarjeta y transferencias recibidas. El sistema no le mostrará cuánto debería haber; la
          conciliación la revisa el Auditor.
        </Alert>
        {error && <Alert tone="danger">{error}</Alert>}
        <div className="grid gap-3 sm:grid-cols-3">
          <Input
            label="Efectivo contado"
            inputMode="decimal"
            placeholder="0.00"
            value={f.efectivo}
            onChange={(e) => setF({ ...f, efectivo: e.target.value })}
          />
          <Input
            label="Total de vouchers"
            inputMode="decimal"
            placeholder="0.00"
            value={f.tarjeta}
            onChange={(e) => setF({ ...f, tarjeta: e.target.value })}
          />
          <Input
            label="Transferencias"
            inputMode="decimal"
            placeholder="0.00"
            value={f.transferencia}
            onChange={(e) => setF({ ...f, transferencia: e.target.value })}
          />
        </div>
        <Textarea
          label="Nota (opcional)"
          rows={2}
          value={f.nota}
          onChange={(e) => setF({ ...f, nota: e.target.value })}
        />
        {confirmar && (
          <Alert tone="warn" title="¿Confirma el conteo?">
            Efectivo {fmtDinero(f.efectivo || "0")} · tarjeta {fmtDinero(f.tarjeta || "0")} · transferencias{" "}
            {fmtDinero(f.transferencia || "0")}. Después de cerrar ya no podrá cobrar ni anular en esta caja.
          </Alert>
        )}
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={alCancelar}>
            Cancelar
          </Button>
          <Button
            loading={enviando}
            disabled={invalido}
            onClick={async () => {
              if (!confirmar) return setConfirmar(true);
              setEnviando(true);
              setError("");
              try {
                const r = await cerrarCaja({
                  efectivo: deCentavos(aCentavos(f.efectivo || "0")),
                  tarjeta: deCentavos(aCentavos(f.tarjeta || "0")),
                  transferencia: deCentavos(aCentavos(f.transferencia || "0")),
                  nota: f.nota.trim(),
                });
                alCerrar(r.mensaje);
              } catch (e) {
                setError(mensajeDe(e));
              } finally {
                setEnviando(false);
              }
            }}
          >
            <Lock className="size-4" /> {confirmar ? "Sí, cerrar caja" : "Cerrar caja"}
          </Button>
        </div>
      </div>
    </Modal>
  );
};

const CajaRecepcion = () => {
  const [params] = useSearchParams();
  const { data, isLoading, error, recargar } = useApi(useCallback(() => getMiCaja(), []));
  const cobros = useApi(useCallback(() => getMisCobros(), []));
  const [pestana, setPestana] = useState("cobrar");
  const [cerrando, setCerrando] = useState(false);
  const [ultimo, setUltimo] = useState(null);
  const [aviso, setAviso] = useState("");

  if (isLoading && !data) return <Skeleton className="h-48" />;
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data.abierta)
    return (
      <Apertura
        aviso={aviso}
        alAbrir={() => {
          setAviso("");
          recargar();
          cobros.recargar();
        }}
      />
    );

  const s = data.sesion;
  const actualizar = () => {
    recargar();
    cobros.recargar();
  };
  return (
    <div className="space-y-4">
      <Card className="flex flex-wrap items-center justify-between gap-3 p-4">
        <div>
          <p className="font-medium">Caja abierta desde {fmtFechaHora(s.abiertaEn)}</p>
          <p className="text-sm text-muted">
            Fondo inicial {fmtDinero(s.fondoInicial)} · {s.cobros} cobro(s) registrados
          </p>
        </div>
        <Button variant="secondary" onClick={() => setCerrando(true)}>
          <Lock className="size-4" /> Cerrar caja
        </Button>
      </Card>
      <Tabs
        value={pestana}
        onChange={setPestana}
        tabs={[
          { id: "cobrar", label: "Cobrar" },
          { id: "cobros", label: "Cobros de esta caja" },
        ]}
      />
      {pestana === "cobrar" && (
        <>
          {ultimo && (
            <Alert tone="ok" title={`Cobro N.º ${ultimo.numero} registrado · ${fmtDinero(ultimo.total)}`}>
              <span className="flex flex-wrap items-center gap-2">
                <EstadoComprobante cb={ultimo} />
                <Button variant="secondary" onClick={() => imprimirCobro(ultimo)}>
                  <Printer className="size-4" /> Imprimir comprobante
                </Button>
              </span>
            </Alert>
          )}
          <FormularioCobro
            pacienteInicial={params.get("paciente")}
            alCobrar={(cb) => {
              setUltimo(cb);
              actualizar();
            }}
          />
        </>
      )}
      {pestana === "cobros" &&
        (cobros.error ? (
          <Alert tone="danger">{cobros.error}</Alert>
        ) : !cobros.data ? (
          <Skeleton className="h-40" />
        ) : (
          <ListaCobros cobros={cobros.data} puedeAnular alCambiar={actualizar} />
        ))}
      {cerrando && (
        <CierreCiego
          alCancelar={() => setCerrando(false)}
          alCerrar={(mensaje) => {
            setCerrando(false);
            setUltimo(null);
            setAviso(mensaje);
            actualizar();
          }}
        />
      )}
    </div>
  );
};

export default CajaRecepcion;

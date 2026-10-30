// Pestaña "Consentimientos" (RF-06). Flujo: plantilla → vista previa del texto EXACTO → el firmante
// (paciente o, si es menor, su tutor) lo lee y firma en pantalla o con un código enviado (OTP).
// El servidor guarda el texto, su SHA-256, la firma y la fecha; después ya nada cambia.
import { ClipboardSignature, KeyRound, PenLine, Plus, Printer, ShieldCheck, ShieldX } from "lucide-react";
import { useCallback, useRef, useState } from "react";
import { Alert, Badge, Button, Card, Checkbox, EmptyState, Input, Modal, Select, Skeleton } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { getProcedimientos } from "../../services/catalogosService";
import { mensajeDe } from "../../services/http";
import {
  crearConsentimiento,
  getConsentimiento,
  getConsentimientosPaciente,
  getPlantillas,
  verificarOTP,
  vistaPreviaConsentimiento,
} from "../../services/planesService";
import { ETIQUETA_CANAL, ETIQUETA_METODO_FIRMA } from "../../utils/etiquetas";
import { fmtFechaHora, fmtFechaHoraSeg } from "../../utils/fechas";
import LienzoFirma from "./LienzoFirma";
import { imprimirConsentimiento } from "../../utils/documentos";

export const DetalleConsentimiento = ({ id, alCerrar }) => {
  const { data: c, error } = useApi(useCallback(() => getConsentimiento(id), [id]));
  return (
    <Modal open title={c?.titulo ?? "Consentimiento"} onClose={alCerrar}>
      {error && <Alert tone="danger">{error}</Alert>}
      {!c && !error && <Skeleton className="h-60" />}
      {c && (
        <div className="space-y-3">
          {c.integro === false && (
            <Alert tone="danger" title="Integridad comprometida">
              El documento o la firma no coinciden con sus huellas.
            </Alert>
          )}
          <div className="max-h-72 overflow-y-auto whitespace-pre-wrap rounded-lg border border-line bg-canvas/50 p-3 text-sm">
            {c.texto}
          </div>
          {c.firmaImagen && (
            <img src={c.firmaImagen} alt="Firma del firmante" className="max-h-28 border-b border-ink/40" />
          )}
          <p className="text-sm">
            {c.firmanteNombre} · {c.firmanteDocumento} ({c.firmanteRelacion === "TUTOR" ? "tutor legal" : "paciente"})
          </p>
          <p className="text-xs text-muted">
            {ETIQUETA_METODO_FIRMA[c.metodo]}
            {c.metodo === "OTP" && ` a ${c.otpDestinatario}${c.otpSimulado ? " (proveedor SIMULADO)" : ""}`} ·{" "}
            {c.firmadoEn ? `firmado ${fmtFechaHoraSeg(c.firmadoEn)}` : "pendiente de firma"}
            <br />
            Documento SHA-256 <code className="font-mono">{c.hashDocumento.slice(0, 16)}…</code>
          </p>
          {c.estado === "FIRMADO" && (
            <div className="flex justify-end">
              <Button variant="secondary" onClick={() => imprimirConsentimiento(c)}>
                <Printer className="size-4" /> Imprimir
              </Button>
            </div>
          )}
        </div>
      )}
    </Modal>
  );
};

// Ingreso del código OTP (también para retomar un consentimiento pendiente).
const IngresoOTP = ({ consentimiento, codigoSimulado, alFirmar }) => {
  const [codigo, setCodigo] = useState("");
  const [error, setError] = useState("");
  const [enviando, setEnviando] = useState(false);
  const enviar = async (e) => {
    e.preventDefault();
    if (!/^\d{6}$/.test(codigo)) return setError("El código tiene 6 dígitos.");
    setEnviando(true);
    setError("");
    try {
      alFirmar(await verificarOTP(consentimiento.id, codigo));
    } catch (err) {
      setError(mensajeDe(err));
      setCodigo("");
    } finally {
      setEnviando(false);
    }
  };
  return (
    <form onSubmit={enviar} className="space-y-3" noValidate>
      <p className="text-sm">
        Se envió un código por {ETIQUETA_CANAL[consentimiento.otpCanal] ?? consentimiento.otpCanal} a{" "}
        <strong>{consentimiento.otpDestinatario}</strong>. Vence {fmtFechaHora(consentimiento.otpExpira)}.
      </p>
      {codigoSimulado && (
        <Alert tone="warn" title="Proveedor de mensajería SIMULADO">
          No se envió ningún SMS real. Código para la demostración: <code className="font-mono">{codigoSimulado}</code>
        </Alert>
      )}
      {error && <Alert tone="danger">{error}</Alert>}
      <Input
        label="Código que recibió el firmante"
        inputMode="numeric"
        autoComplete="one-time-code"
        maxLength={6}
        value={codigo}
        onChange={(e) => setCodigo(e.target.value.replace(/\D/g, ""))}
        className="[&_input]:text-center [&_input]:font-mono [&_input]:text-lg [&_input]:tracking-[0.3em]"
        autoFocus
      />
      <div className="flex justify-end">
        <Button type="submit" loading={enviando}>
          <KeyRound className="size-4" /> Firmar con el código
        </Button>
      </div>
    </form>
  );
};

const NuevoConsentimiento = ({ paciente, alTerminar, alCancelar }) => {
  const plantillas = useApi(useCallback(() => getPlantillas(), []));
  const procs = useApi(useCallback(() => getProcedimientos(), []));
  const [plantillaId, setPlantillaId] = useState("");
  const [procedimientoId, setProcedimientoId] = useState("");
  const [vista, setVista] = useState(null);
  const [metodo, setMetodo] = useState("MANUSCRITA");
  const [leido, setLeido] = useState(false);
  const [pendiente, setPendiente] = useState(null); // consentimiento creado esperando el OTP
  const [error, setError] = useState("");
  const [enviando, setEnviando] = useState(false);
  const lienzo = useRef(null);

  const plantilla = plantillas.data?.find((p) => p.id === plantillaId);

  const mostrar = async () => {
    setError("");
    if (!plantillaId) return setError("Seleccione la plantilla.");
    setEnviando(true);
    try {
      setVista(await vistaPreviaConsentimiento(paciente.id, { plantillaId, procedimientoId }));
      setLeido(false);
    } catch (e) {
      setError(mensajeDe(e));
    } finally {
      setEnviando(false);
    }
  };

  const firmar = async () => {
    setError("");
    if (!leido) return setError("Confirme que el firmante leyó el documento.");
    if (metodo === "MANUSCRITA" && lienzo.current?.vacio()) return setError("El firmante debe firmar en el recuadro.");
    setEnviando(true);
    try {
      const c = await crearConsentimiento(paciente.id, {
        plantillaId,
        procedimientoId,
        metodo,
        hashDocumento: vista.hashDocumento,
        ...(metodo === "MANUSCRITA" ? { firmaImagen: lienzo.current.aDataURL() } : {}),
      });
      if (c.estado === "FIRMADO") alTerminar(c);
      else setPendiente(c);
    } catch (e) {
      setError(mensajeDe(e));
      if (e.code === "DOCUMENTO_CAMBIO") setVista(null);
    } finally {
      setEnviando(false);
    }
  };

  if (pendiente)
    return (
      <Card className="space-y-3 p-4">
        <p className="font-medium">{vista.titulo}</p>
        <IngresoOTP consentimiento={pendiente} codigoSimulado={pendiente.otpSimuladoCodigo} alFirmar={alTerminar} />
      </Card>
    );

  return (
    <Card className="space-y-4 p-4">
      {error && <Alert tone="danger">{error}</Alert>}
      <div className="grid gap-3 sm:grid-cols-2">
        <Select
          label="Plantilla"
          value={plantillaId}
          onChange={(e) => {
            setPlantillaId(e.target.value);
            setVista(null);
          }}
        >
          <option value="">Seleccione…</option>
          {plantillas.data?.map((p) => (
            <option key={p.id} value={p.id}>
              {p.codigo} · {p.titulo} (v{p.version})
            </option>
          ))}
        </Select>
        <Select
          label="Procedimiento"
          value={procedimientoId}
          onChange={(e) => {
            setProcedimientoId(e.target.value);
            setVista(null);
          }}
        >
          <option value="">
            {plantilla?.procedimiento ? `El de la plantilla (${plantilla.procedimiento})` : "Sin especificar"}
          </option>
          {procs.data?.map((p) => (
            <option key={p.id} value={p.id}>
              {p.codigo} · {p.nombre}
            </option>
          ))}
        </Select>
      </div>
      {!vista ? (
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={alCancelar}>
            Cancelar
          </Button>
          <Button onClick={mostrar} loading={enviando}>
            Mostrar documento al firmante
          </Button>
        </div>
      ) : (
        <>
          <div>
            <p className="mb-1 text-sm font-medium">
              {vista.titulo} <span className="font-normal text-muted">(versión {vista.version})</span>
            </p>
            <div
              className="max-h-80 overflow-y-auto whitespace-pre-wrap rounded-lg border border-line bg-canvas/50 p-4 text-[15px] leading-relaxed"
              tabIndex={0}
              aria-label="Texto del consentimiento"
            >
              {vista.texto}
            </div>
            <p className="mt-1 text-xs text-muted">
              Firma: <strong>{vista.firmanteNombre}</strong> · {vista.firmanteDocumento} (
              {vista.firmanteRelacion === "TUTOR" ? "tutor legal: el paciente es menor de edad" : "paciente"})
            </p>
          </div>
          <Checkbox
            label="El firmante leyó el documento y se respondieron sus preguntas."
            checked={leido}
            onChange={(v) => {
              setLeido(v);
              setError("");
            }}
          />
          <fieldset>
            <legend className="mb-1 text-sm font-medium">Forma de firma</legend>
            <div className="flex flex-wrap gap-2">
              {["MANUSCRITA", "OTP"].map((m) => (
                <label
                  key={m}
                  className={`flex min-h-10 cursor-pointer items-center gap-2 rounded-lg border px-4 text-sm ${metodo === m ? "border-brand-500 bg-brand-50 text-brand-800" : "border-line"}`}
                >
                  <input
                    type="radio"
                    name="metodo-firma"
                    checked={metodo === m}
                    onChange={() => setMetodo(m)}
                    className="accent-brand-700"
                  />
                  {ETIQUETA_METODO_FIRMA[m]}
                </label>
              ))}
            </div>
          </fieldset>
          {metodo === "MANUSCRITA" ? (
            <LienzoFirma ref={lienzo} />
          ) : (
            <p className="text-sm text-muted">
              Se enviará un código de 6 dígitos al teléfono o correo del firmante (vence en 10 minutos, 5 intentos).
            </p>
          )}
          <div className="flex justify-end gap-2 border-t border-line pt-3">
            <Button variant="secondary" onClick={alCancelar}>
              Cancelar
            </Button>
            <Button onClick={firmar} loading={enviando}>
              {metodo === "MANUSCRITA" ? (
                <>
                  <PenLine className="size-4" /> Firmar
                </>
              ) : (
                <>
                  <KeyRound className="size-4" /> Enviar código
                </>
              )}
            </Button>
          </div>
        </>
      )}
    </Card>
  );
};

const ConsentimientosPaciente = ({ paciente }) => {
  const { puede } = useAuth();
  const { data, isLoading, error, recargar } = useApi(
    useCallback(() => getConsentimientosPaciente(paciente.id), [paciente.id]),
  );
  const [nuevo, setNuevo] = useState(false);
  const [detalle, setDetalle] = useState(null);
  const [otp, setOtp] = useState(null);
  const [aviso, setAviso] = useState("");
  const gestiona = puede("consentimientos.gestionar");

  if (isLoading && !data) return <Skeleton className="h-48" />;
  if (error) return <Alert tone="danger">{error}</Alert>;
  return (
    <div className="space-y-4">
      {aviso && <Alert tone="ok">{aviso}</Alert>}
      {gestiona && !nuevo && (
        <Button
          onClick={() => {
            setNuevo(true);
            setAviso("");
          }}
        >
          <Plus className="size-4" /> Nuevo consentimiento
        </Button>
      )}
      {nuevo && (
        <NuevoConsentimiento
          paciente={paciente}
          alCancelar={() => setNuevo(false)}
          alTerminar={(c) => {
            setNuevo(false);
            setAviso(`Consentimiento firmado: ${c.titulo}.`);
            recargar();
          }}
        />
      )}
      {data.length === 0 ? (
        <Card>
          <EmptyState icon={<ClipboardSignature className="size-8" />} title="Sin consentimientos" />
        </Card>
      ) : (
        <Card>
          <ul className="divide-y divide-line">
            {data.map((c) => (
              <li key={c.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 text-sm">
                <button className="min-w-48 flex-1 text-left" onClick={() => setDetalle(c.id)}>
                  <span className="font-medium text-brand-800 hover:underline">
                    {c.titulo} <span className="font-normal text-muted">v{c.version}</span>
                  </span>
                  <span className="block text-xs text-muted">
                    {c.procedimiento ?? "Sin procedimiento"} · {c.firmanteNombre} · {ETIQUETA_METODO_FIRMA[c.metodo]}
                  </span>
                </button>
                {c.estado === "FIRMADO" ? (
                  <>
                    <span className="text-xs text-muted">{fmtFechaHora(c.firmadoEn)}</span>
                    {c.integro === false ? (
                      <Badge tone="danger">
                        <ShieldX className="size-3" /> Alterado
                      </Badge>
                    ) : (
                      <Badge tone="ok">
                        <ShieldCheck className="size-3" /> Firmado
                      </Badge>
                    )}
                  </>
                ) : new Date(c.otpExpira) > new Date() && c.otpIntentos < 5 && gestiona ? (
                  <Button variant="secondary" onClick={() => setOtp(c)}>
                    <KeyRound className="size-4" /> Ingresar código
                  </Button>
                ) : (
                  <Badge tone="neutral">Código vencido o bloqueado</Badge>
                )}
              </li>
            ))}
          </ul>
        </Card>
      )}
      {detalle && <DetalleConsentimiento id={detalle} alCerrar={() => setDetalle(null)} />}
      {otp && (
        <Modal open title="Firmar con el código" onClose={() => setOtp(null)}>
          <IngresoOTP
            consentimiento={otp}
            alFirmar={(c) => {
              setOtp(null);
              setAviso(`Consentimiento firmado: ${c.titulo}.`);
              recargar();
            }}
          />
        </Modal>
      )}
    </div>
  );
};

export default ConsentimientosPaciente;

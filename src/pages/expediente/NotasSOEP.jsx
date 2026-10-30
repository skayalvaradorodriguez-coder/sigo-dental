// Evolución clínica en formato SOEP (RF-05): bitácora cronológica. Un borrador lo edita sólo su autor
// y sólo hasta el bloqueo (24 h por defecto, configurable). Firmada o bloqueada ⇒ sólo adendas.
import { Lock, PenLine, Plus, ShieldAlert, ShieldCheck } from "lucide-react";
import { useCallback, useState } from "react";
import { Alert, Badge, Button, Card, EmptyState, Skeleton, Textarea } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { actualizarNota, adendaNota, crearNota, firmarNota, getNotas } from "../../services/expedienteService";
import { mensajeDe } from "../../services/http";
import { fmtFaltan, fmtFechaHora } from "../../utils/fechas";
import Adendas from "./Adendas";
import ModalFirma from "./ModalFirma";

const APARTADOS = [
  ["subjetivo", "S — Subjetivo", "Lo que refiere el paciente"],
  ["objetivo", "O — Objetivo", "Hallazgos del examen"],
  ["evaluacion", "E — Evaluación", "Diagnóstico o impresión clínica"],
  ["plan", "P — Plan", "Tratamiento, indicaciones y próximos pasos"],
];

const ESTADO = {
  BORRADOR: { tono: "warn", texto: "Borrador" },
  FIRMADA: { tono: "ok", texto: "Firmada" },
  BLOQUEADA_SIN_FIRMA: { tono: "danger", texto: "Bloqueada sin firma" },
};

const Editor = ({ inicial, alGuardar, alCancelar }) => {
  const [f, setF] = useState(inicial ?? { subjetivo: "", objetivo: "", evaluacion: "", plan: "" });
  const [error, setError] = useState("");
  const [enviando, setEnviando] = useState(false);
  const enviar = async (e) => {
    e.preventDefault();
    if (!Object.values(f).some((v) => v.trim())) return setError("Complete al menos un apartado.");
    setEnviando(true);
    setError("");
    try {
      await alGuardar(f);
    } catch (err) {
      setError(mensajeDe(err));
    } finally {
      setEnviando(false);
    }
  };
  return (
    <form onSubmit={enviar} className="space-y-3" noValidate>
      {error && <Alert tone="danger">{error}</Alert>}
      <div className="grid gap-3 lg:grid-cols-2">
        {APARTADOS.map(([k, t, h]) => (
          <Textarea
            key={k}
            label={t}
            placeholder={h}
            rows={3}
            value={f[k]}
            onChange={(e) => setF({ ...f, [k]: e.target.value })}
          />
        ))}
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={alCancelar}>
          Cancelar
        </Button>
        <Button type="submit" loading={enviando}>
          Guardar borrador
        </Button>
      </div>
    </form>
  );
};

const Nota = ({ n, puedeFirmar, alEditar, alFirmar, alAdenda }) => {
  const e = ESTADO[n.estado];
  return (
    <Card className="p-4">
      <div className="mb-3 flex flex-wrap items-start justify-between gap-2">
        <div>
          <p className="font-medium">{fmtFechaHora(n.creadoEn)}</p>
          <p className="text-xs text-muted">Autor: {n.autor}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone={e.tono}>{e.texto}</Badge>
          {n.estado === "BORRADOR" && (
            <span className="text-xs text-muted">
              <Lock className="mr-1 inline size-3" />
              editable hasta {fmtFechaHora(n.bloqueoEn)} ({fmtFaltan(n.bloqueoEn)})
            </span>
          )}
        </div>
      </div>
      <dl className="grid gap-3 lg:grid-cols-2">
        {APARTADOS.map(([k, t]) => (
          <div key={k}>
            <dt className="text-xs font-semibold uppercase tracking-wider text-muted">{t}</dt>
            <dd className="whitespace-pre-wrap text-sm">{n[k] || <span className="text-muted">—</span>}</dd>
          </div>
        ))}
      </dl>
      {n.estado === "FIRMADA" && (
        <div
          className={`mt-3 flex flex-wrap items-center gap-2 rounded-lg px-3 py-2 text-sm ${n.integra ? "bg-ok-50 text-ok-700" : "bg-danger-50 text-danger-700"}`}
        >
          {n.integra ? <ShieldCheck className="size-4" /> : <ShieldAlert className="size-4" />}
          <span>
            Firmada por <strong>{n.firmadaPor}</strong>
            {n.registroProfesional && ` (${n.registroProfesional})`} el {fmtFechaHora(n.firmadaEn)}
          </span>
          <code className="font-mono text-xs">{n.hashFirma.slice(0, 16)}…</code>
          {!n.integra && <strong>· El contenido no coincide con la firma: posible alteración</strong>}
        </div>
      )}
      <Adendas adendas={n.adendas} />
      <div className="mt-3 flex flex-wrap justify-end gap-2">
        {n.editable && (
          <Button variant="secondary" onClick={alEditar}>
            Editar borrador
          </Button>
        )}
        {puedeFirmar && n.estado !== "FIRMADA" && (
          <Button onClick={alFirmar}>
            <PenLine className="size-4" /> Firmar
          </Button>
        )}
        {puedeFirmar && n.estado !== "BORRADOR" && (
          <Button variant="secondary" onClick={alAdenda}>
            <Plus className="size-4" /> Adenda
          </Button>
        )}
      </div>
    </Card>
  );
};

const NotasSOEP = ({ paciente }) => {
  const { puede } = useAuth();
  const { data, isLoading, error, recargar } = useApi(useCallback(() => getNotas(paciente.id), [paciente.id]));
  const [nueva, setNueva] = useState(false);
  const [editando, setEditando] = useState(null);
  const [firma, setFirma] = useState(null); // { nota, tipo: "firmar" | "adenda" }
  const puedeEscribir = puede("clinica.escribir") && !paciente.anamnesisPendiente;
  const puedeFirmar = puede("clinica.firmar");

  if (isLoading && !data) return <Skeleton className="h-60" />;
  if (error) return <Alert tone="danger">{error}</Alert>;

  return (
    <div className="space-y-4">
      {paciente.anamnesisPendiente && (
        <Alert tone="warn">Registre primero la anamnesis: es obligatoria antes de cualquier registro clínico.</Alert>
      )}
      {puedeEscribir && !nueva && (
        <Button onClick={() => setNueva(true)}>
          <Plus className="size-4" /> Nueva nota SOEP
        </Button>
      )}
      {nueva && (
        <Card className="p-4">
          <Editor
            alCancelar={() => setNueva(false)}
            alGuardar={async (f) => {
              await crearNota(paciente.id, f);
              setNueva(false);
              recargar();
            }}
          />
        </Card>
      )}
      {data.length === 0 && !nueva && (
        <Card>
          <EmptyState title="Sin notas de evolución" />
        </Card>
      )}
      {data.map((n) =>
        editando === n.id ? (
          <Card key={n.id} className="p-4">
            <Editor
              inicial={{ subjetivo: n.subjetivo, objetivo: n.objetivo, evaluacion: n.evaluacion, plan: n.plan }}
              alCancelar={() => setEditando(null)}
              alGuardar={async (f) => {
                await actualizarNota(n.id, f);
                setEditando(null);
                recargar();
              }}
            />
          </Card>
        ) : (
          <Nota
            key={n.id}
            n={n}
            puedeFirmar={puedeFirmar}
            alEditar={() => setEditando(n.id)}
            alFirmar={() => setFirma({ nota: n, tipo: "firmar" })}
            alAdenda={() => setFirma({ nota: n, tipo: "adenda" })}
          />
        ),
      )}
      {firma && (
        <ModalFirma
          titulo={firma.tipo === "firmar" ? "Firmar nota SOEP" : "Agregar adenda firmada"}
          descripcion={
            firma.tipo === "firmar"
              ? "Al firmar, la nota queda cerrada: ya no podrá editarse."
              : "La nota original no cambia: la adenda se agrega debajo, con su propia firma."
          }
          conTexto={firma.tipo === "adenda"}
          alCerrar={() => setFirma(null)}
          alFirmar={async ({ codigo, texto }) => {
            if (firma.tipo === "firmar") await firmarNota(firma.nota.id, codigo);
            else await adendaNota(firma.nota.id, texto, codigo);
            setFirma(null);
            recargar();
          }}
        />
      )}
    </div>
  );
};

export default NotasSOEP;

// Módulo "Consentimientos" (RF-06): consentimientos recientes y plantillas versionadas.
// Una plantilla no se edita: guardar con el mismo código crea la versión siguiente, y los
// consentimientos ya firmados conservan el texto exacto de su versión.
import { ClipboardSignature, Plus } from "lucide-react";
import { useCallback, useState } from "react";
import { Link } from "react-router-dom";
import {
  Alert,
  Badge,
  Button,
  Card,
  EmptyState,
  Input,
  Modal,
  PageHeader,
  Select,
  Skeleton,
  Tabs,
  Textarea,
} from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { getProcedimientos } from "../../services/catalogosService";
import { mensajeDe } from "../../services/http";
import { crearPlantilla, getConsentimientos, getPlantillas } from "../../services/planesService";
import { ETIQUETA_METODO_FIRMA } from "../../utils/etiquetas";
import { fmtFechaHora } from "../../utils/fechas";
import { DetalleConsentimiento } from "./ConsentimientosPaciente";

const MARCADORES =
  "{{paciente}} {{documento}} {{edad}} {{procedimiento}} {{odontologo}} {{registro}} {{fecha}} {{firmante}} {{relacion}} {{clinica}}";

const Bandeja = ({ estado }) => {
  const { data, isLoading, error } = useApi(useCallback(() => getConsentimientos(estado), [estado]));
  const [detalle, setDetalle] = useState(null);
  if (isLoading && !data) return <Skeleton className="m-4 h-40" />;
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data.length) return <EmptyState icon={<ClipboardSignature className="size-8" />} title="Sin consentimientos" />;
  return (
    <>
      <ul className="divide-y divide-line">
        {data.map((c) => (
          <li key={c.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 text-sm">
            <button className="min-w-48 flex-1 text-left" onClick={() => setDetalle(c.id)}>
              <span className="font-medium">{c.paciente}</span>
              <span className="block text-xs text-muted">
                {c.titulo} v{c.version} · {c.firmanteNombre} · {ETIQUETA_METODO_FIRMA[c.metodo]}
              </span>
            </button>
            <span className="text-xs text-muted">{fmtFechaHora(c.firmadoEn ?? c.creadoEn)}</span>
            {c.integro === false && <Badge tone="danger">Alterado</Badge>}
            <Link
              to={`/pacientes/${c.pacienteId}?pestana=consentimientos`}
              className="text-xs text-brand-700 hover:underline"
            >
              Ver ficha
            </Link>
          </li>
        ))}
      </ul>
      {detalle && <DetalleConsentimiento id={detalle} alCerrar={() => setDetalle(null)} />}
    </>
  );
};

const NuevaVersion = ({ inicial, alCerrar, alGuardar }) => {
  const procs = useApi(useCallback(() => getProcedimientos(), []));
  const [f, setF] = useState({
    codigo: inicial?.codigo ?? "",
    titulo: inicial?.titulo ?? "",
    texto: inicial?.texto ?? "",
    procedimientoId: inicial?.procedimientoId ?? "",
  });
  const [campos, setCampos] = useState({});
  const [error, setError] = useState("");
  const [enviando, setEnviando] = useState(false);
  return (
    <Modal open title={inicial ? `Nueva versión de ${inicial.codigo}` : "Nueva plantilla"} onClose={alCerrar}>
      <form
        className="space-y-3"
        noValidate
        onSubmit={async (e) => {
          e.preventDefault();
          setEnviando(true);
          setError("");
          try {
            alGuardar(await crearPlantilla(f));
          } catch (err) {
            setError(mensajeDe(err));
            setCampos(err.campos ?? {});
          } finally {
            setEnviando(false);
          }
        }}
      >
        {error && <Alert tone="danger">{error}</Alert>}
        <div className="grid gap-3 sm:grid-cols-2">
          <Input
            label="Código"
            value={f.codigo}
            disabled={!!inicial}
            onChange={(e) => setF({ ...f, codigo: e.target.value.toUpperCase() })}
            error={campos.codigo}
          />
          <Select
            label="Procedimiento (opcional)"
            value={f.procedimientoId}
            onChange={(e) => setF({ ...f, procedimientoId: e.target.value })}
          >
            <option value="">General</option>
            {procs.data?.map((p) => (
              <option key={p.id} value={p.id}>
                {p.codigo} · {p.nombre}
              </option>
            ))}
          </Select>
        </div>
        <Input
          label="Título"
          value={f.titulo}
          onChange={(e) => setF({ ...f, titulo: e.target.value })}
          error={campos.titulo}
        />
        <Textarea
          label="Texto"
          rows={10}
          value={f.texto}
          onChange={(e) => setF({ ...f, texto: e.target.value })}
          error={campos.texto}
          hint={`Marcadores: ${MARCADORES}`}
        />
        <Alert tone="warn">
          Texto de ejemplo: cada clínica debe usar el que apruebe su asesoría jurídica según la normativa de su país.
        </Alert>
        <div className="flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={alCerrar}>
            Cancelar
          </Button>
          <Button type="submit" loading={enviando}>
            Guardar versión
          </Button>
        </div>
      </form>
    </Modal>
  );
};

const Plantillas = () => {
  const { puede } = useAuth();
  const { data, isLoading, error, recargar } = useApi(useCallback(() => getPlantillas(), []));
  const [editor, setEditor] = useState(null); // null | { inicial?: plantilla }
  const edita = puede("catalogos.gestionar");
  if (isLoading && !data) return <Skeleton className="m-4 h-40" />;
  if (error) return <Alert tone="danger">{error}</Alert>;
  return (
    <div className="divide-y divide-line">
      {edita && (
        <div className="p-3">
          <Button variant="secondary" onClick={() => setEditor({})}>
            <Plus className="size-4" /> Nueva plantilla
          </Button>
        </div>
      )}
      {data.map((p) => (
        <details key={p.id} className="px-4 py-3">
          <summary className="flex cursor-pointer flex-wrap items-center gap-x-3 text-sm">
            <span className="font-medium">
              {p.codigo} · {p.titulo}
            </span>
            <Badge tone="brand">v{p.version}</Badge>
            <span className="text-xs text-muted">
              {p.procedimiento ?? "General"} · {fmtFechaHora(p.creadoEn)}
            </span>
          </summary>
          <p className="mt-2 whitespace-pre-wrap rounded-lg bg-canvas/60 p-3 text-sm">{p.texto}</p>
          {edita && (
            <Button className="mt-2" variant="secondary" onClick={() => setEditor({ inicial: p })}>
              Crear versión {p.version + 1}
            </Button>
          )}
        </details>
      ))}
      {editor && (
        <NuevaVersion
          inicial={editor.inicial}
          alCerrar={() => setEditor(null)}
          alGuardar={() => {
            setEditor(null);
            recargar();
          }}
        />
      )}
    </div>
  );
};

const Consentimientos = () => {
  const { puede } = useAuth();
  const gestiona = puede("consentimientos.gestionar");
  const pestanas = [
    ...(gestiona
      ? [
          { id: "FIRMADO", label: "Firmados" },
          { id: "PENDIENTE_OTP", label: "Esperan código" },
        ]
      : []),
    { id: "plantillas", label: "Plantillas" },
  ];
  const [pestana, setPestana] = useState(pestanas[0].id);
  return (
    <>
      <PageHeader
        title="Consentimientos informados"
        description="Firma en pantalla (imagen del trazo) o con un código enviado al paciente. El documento firmado se guarda con su huella SHA-256 y no se modifica."
      />
      <Tabs value={pestana} onChange={setPestana} tabs={pestanas} />
      <Card className="mt-4">{pestana === "plantillas" ? <Plantillas /> : <Bandeja estado={pestana} />}</Card>
      {gestiona && (
        <p className="mt-3 text-xs text-muted">
          Para generar un consentimiento, abra la ficha del paciente → pestaña “Consentimientos”.
        </p>
      )}
    </>
  );
};

export default Consentimientos;

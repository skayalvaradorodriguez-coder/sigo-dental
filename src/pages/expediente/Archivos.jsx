// Archivos del paciente: radiografías, fotografías y documentos (JPG, PNG o PDF de hasta 10 MB).
// El servidor identifica el tipo por el contenido, guarda su SHA-256 y lo verifica en cada descarga.
import { Download, Eye, FileImage, FileText, Upload } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Alert, Badge, Button, Card, EmptyState, Input, Modal, Select, Skeleton } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { mensajeDe } from "../../services/http";
import { descargarArchivo, getArchivos, subirArchivo } from "../../services/planesService";
import { ETIQUETA_CATEGORIA_ARCHIVO } from "../../utils/etiquetas";
import { fmtFechaHora } from "../../utils/fechas";
import { fmtTamano } from "../../utils/documentos";

const MAX = 10 * 1024 * 1024;
const Visor = ({ archivo, alCerrar }) => {
  const [url, setUrl] = useState("");
  const [error, setError] = useState("");
  useEffect(() => {
    let u = "";
    descargarArchivo(archivo.id)
      .then((blob) => {
        u = URL.createObjectURL(blob);
        setUrl(u);
      })
      .catch((e) => setError(mensajeDe(e)));
    return () => u && URL.revokeObjectURL(u);
  }, [archivo.id]);
  return (
    <Modal open title={archivo.nombre} onClose={alCerrar}>
      {error && <Alert tone="danger">{error}</Alert>}
      {!url && !error && <Skeleton className="h-60" />}
      {url && archivo.tipoMime.startsWith("image/") && (
        <img src={url} alt={archivo.descripcion ?? archivo.nombre} className="max-h-[60vh] w-full object-contain" />
      )}
      {url && archivo.tipoMime === "application/pdf" && (
        <iframe src={url} title={archivo.nombre} className="h-[60vh] w-full" />
      )}
      {url && (
        <div className="mt-3 flex items-center justify-between">
          <p className="text-xs text-muted">
            Huella verificada · SHA-256 <code className="font-mono">{archivo.sha256.slice(0, 16)}…</code>
          </p>
          <a
            href={url}
            download={archivo.nombre}
            className="inline-flex items-center gap-1 text-sm text-brand-700 hover:underline"
          >
            <Download className="size-4" /> Descargar
          </a>
        </div>
      )}
    </Modal>
  );
};

const Archivos = ({ paciente }) => {
  const { puede } = useAuth();
  const { data, isLoading, error, recargar } = useApi(useCallback(() => getArchivos(paciente.id), [paciente.id]));
  const [archivo, setArchivo] = useState(null);
  const [categoria, setCategoria] = useState("RADIOGRAFIA");
  const [descripcion, setDescripcion] = useState("");
  const [errorSubida, setErrorSubida] = useState("");
  const [subiendo, setSubiendo] = useState(false);
  const [visor, setVisor] = useState(null);
  const [entrada, setEntrada] = useState(0); // para limpiar el <input type=file>

  const subir = async (e) => {
    e.preventDefault();
    setErrorSubida("");
    if (!archivo) return setErrorSubida("Seleccione un archivo.");
    if (archivo.size > MAX) return setErrorSubida("El archivo supera 10 MB.");
    setSubiendo(true);
    try {
      await subirArchivo(paciente.id, { archivo, categoria, descripcion: descripcion.trim() });
      setArchivo(null);
      setDescripcion("");
      setEntrada((n) => n + 1);
      recargar();
    } catch (err) {
      setErrorSubida(err.campos?.archivo ?? mensajeDe(err));
    } finally {
      setSubiendo(false);
    }
  };

  if (isLoading && !data) return <Skeleton className="h-48" />;
  if (error) return <Alert tone="danger">{error}</Alert>;
  return (
    <div className="space-y-4">
      {puede("clinica.escribir") && !paciente.anamnesisPendiente && (
        <Card className="p-4">
          <form onSubmit={subir} noValidate className="space-y-3">
            {errorSubida && <Alert tone="danger">{errorSubida}</Alert>}
            <div className="grid gap-3 sm:grid-cols-3">
              <div>
                <label htmlFor="archivo-paciente" className="mb-1 block text-sm font-medium">
                  Archivo (JPG, PNG o PDF, máx. 10 MB)
                </label>
                <input
                  key={entrada}
                  id="archivo-paciente"
                  type="file"
                  accept="image/jpeg,image/png,application/pdf"
                  onChange={(e) => setArchivo(e.target.files?.[0] ?? null)}
                  className="block w-full text-sm file:mr-3 file:rounded-lg file:border file:border-line file:bg-white file:px-3 file:py-2"
                />
              </div>
              <Select label="Categoría" value={categoria} onChange={(e) => setCategoria(e.target.value)}>
                {Object.entries(ETIQUETA_CATEGORIA_ARCHIVO).map(([v, l]) => (
                  <option key={v} value={v}>
                    {l}
                  </option>
                ))}
              </Select>
              <Input
                label="Descripción (opcional)"
                value={descripcion}
                onChange={(e) => setDescripcion(e.target.value)}
              />
            </div>
            <div className="flex justify-end">
              <Button type="submit" loading={subiendo}>
                <Upload className="size-4" /> Subir
              </Button>
            </div>
          </form>
        </Card>
      )}
      {data.length === 0 ? (
        <Card>
          <EmptyState icon={<FileImage className="size-8" />} title="Sin archivos" />
        </Card>
      ) : (
        <Card>
          <ul className="divide-y divide-line">
            {data.map((a) => (
              <li key={a.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 text-sm">
                {a.tipoMime === "application/pdf" ? (
                  <FileText className="size-5 text-muted" aria-hidden />
                ) : (
                  <FileImage className="size-5 text-muted" aria-hidden />
                )}
                <span className="min-w-48 flex-1">
                  <span className="font-medium">{a.nombre}</span>
                  <span className="block text-xs text-muted">
                    {a.descripcion && <>{a.descripcion} · </>}
                    {fmtTamano(a.tamano)} · {a.subidoPor} · {fmtFechaHora(a.subidoEn)}
                  </span>
                </span>
                <Badge tone="brand">{ETIQUETA_CATEGORIA_ARCHIVO[a.categoria]}</Badge>
                <Button variant="secondary" onClick={() => setVisor(a)}>
                  <Eye className="size-4" /> Ver
                </Button>
              </li>
            ))}
          </ul>
        </Card>
      )}
      {visor && <Visor archivo={visor} alCerrar={() => setVisor(null)} />}
    </div>
  );
};

export default Archivos;

// Lista y búsqueda de pacientes (RF-01). Cada búsqueda queda auditada en el backend.
import { Plus, Search, UserRound } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { PuntoAlerta } from "../../components/AlertasMedicas";
import { Alert, Badge, Button, Card, EmptyState, PageHeader, Skeleton } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { buscarPacientes } from "../../services/pacientesService";
import { ETIQUETA_DOCUMENTO } from "../../utils/etiquetas";

const LIMITE = 20;

// Retrasa la búsqueda hasta que el usuario deja de escribir (evita una consulta por tecla).
const useRetrasado = (valor, ms = 300) => {
  const [v, setV] = useState(valor);
  useEffect(() => {
    const t = setTimeout(() => setV(valor), ms);
    return () => clearTimeout(t);
  }, [valor, ms]);
  return v;
};

const Pacientes = () => {
  const { puede } = useAuth();
  const navegar = useNavigate();
  const [texto, setTexto] = useState("");
  const [pagina, setPagina] = useState(1);
  const termino = useRetrasado(texto.trim());
  const { data, isLoading, error } = useApi(
    useCallback(() => buscarPacientes({ q: termino, page: pagina, limit: LIMITE }), [termino, pagina]),
  );
  const paginas = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1;

  return (
    <>
      <PageHeader
        title="Pacientes"
        description="Ficha única del paciente. Las alertas médicas críticas se marcan en rojo."
        actions={
          puede("pacientes.escribir") && (
            <Button onClick={() => navegar("/pacientes/nuevo")}>
              <Plus className="size-4" /> Nuevo paciente
            </Button>
          )
        }
      />
      <Card>
        <div className="border-b border-line p-3">
          <div className="relative max-w-md">
            <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted" aria-hidden />
            <input
              type="search"
              value={texto}
              onChange={(e) => {
                setTexto(e.target.value);
                setPagina(1);
              }}
              placeholder="Buscar por nombre, apellido o documento"
              aria-label="Buscar pacientes"
              className="min-h-11 w-full rounded-lg border border-line pl-9 pr-3 text-sm"
            />
          </div>
        </div>
        {isLoading && !data ? (
          <div className="space-y-2 p-4">
            {[0, 1, 2, 3, 4].map((i) => (
              <Skeleton key={i} className="h-12" />
            ))}
          </div>
        ) : error ? (
          <div className="p-4">
            <Alert tone="danger">{error}</Alert>
          </div>
        ) : data.data.length === 0 ? (
          <EmptyState
            icon={<UserRound className="size-8" />}
            title={termino ? "Sin resultados" : "Aún no hay pacientes"}
          >
            {termino
              ? `No hay pacientes que coincidan con “${termino}”.`
              : puede("pacientes.escribir")
                ? "Registre el primero con “Nuevo paciente”."
                : null}
          </EmptyState>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px] text-sm">
              <thead className="border-b border-line text-left text-xs text-muted">
                <tr>
                  <th className="px-4 py-2.5 font-medium">Paciente</th>
                  <th className="px-4 py-2.5 font-medium">Documento</th>
                  <th className="px-4 py-2.5 font-medium">Edad</th>
                  <th className="px-4 py-2.5 font-medium">Convenio</th>
                  <th className="px-4 py-2.5 font-medium">Alertas</th>
                </tr>
              </thead>
              <tbody>
                {data.data.map((p) => {
                  const criticas = (p.alertas ?? []).filter((a) => a.severidad === "CRITICA");
                  return (
                    <tr key={p.id} className="border-b border-line last:border-0 hover:bg-canvas/60">
                      <td className="px-4 py-3">
                        <Link to={`/pacientes/${p.id}`} className="font-medium text-brand-800 hover:underline">
                          {p.apellidos}, {p.nombres}
                        </Link>
                        {p.telefono && <p className="text-xs text-muted">{p.telefono}</p>}
                      </td>
                      <td className="px-4 py-3 text-muted">
                        {ETIQUETA_DOCUMENTO[p.tipoDocumento]} {p.numeroDocumento}
                      </td>
                      <td className="px-4 py-3">
                        {p.edad} años {p.esMenor && <Badge tone="brand">Menor</Badge>}
                      </td>
                      <td className="px-4 py-3 text-muted">{p.convenio ?? "—"}</td>
                      <td className="px-4 py-3">
                        <div className="flex flex-wrap items-center gap-1.5">
                          <PuntoAlerta criticas={criticas.length} pendiente={p.anamnesisPendiente} />
                          {criticas.length > 0 && (
                            <span className="text-xs font-medium text-danger-700">
                              {criticas.map((a) => a.etiqueta).join(", ")}
                            </span>
                          )}
                          {!criticas.length && !p.anamnesisPendiente && <span className="text-xs text-muted">—</span>}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
        {data && data.total > data.limit && (
          <div className="flex items-center justify-between border-t border-line px-4 py-3 text-sm">
            <p className="text-muted">
              {data.total} pacientes · página {pagina} de {paginas}
            </p>
            <div className="flex gap-2">
              <Button variant="secondary" disabled={pagina <= 1} onClick={() => setPagina(pagina - 1)}>
                Anterior
              </Button>
              <Button variant="secondary" disabled={pagina >= paginas} onClick={() => setPagina(pagina + 1)}>
                Siguiente
              </Button>
            </div>
          </div>
        )}
      </Card>
    </>
  );
};

export default Pacientes;

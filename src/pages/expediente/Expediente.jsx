// Entrada "Expediente clínico": buscar al paciente y abrir su odontograma, periodontograma,
// evolución SOEP y procedimientos (todo vive en la ficha, en pestañas).
import { Search, Stethoscope } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { PuntoAlerta } from "../../components/AlertasMedicas";
import { Alert, Card, EmptyState, PageHeader, Skeleton } from "../../components/ui";
import { useApi } from "../../hooks/useApi";
import { buscarPacientes } from "../../services/pacientesService";

const Expediente = () => {
  const [texto, setTexto] = useState("");
  const [termino, setTermino] = useState("");
  useEffect(() => {
    const t = setTimeout(() => setTermino(texto.trim()), 300);
    return () => clearTimeout(t);
  }, [texto]);
  const { data, isLoading, error } = useApi(useCallback(() => buscarPacientes({ q: termino, limit: 20 }), [termino]));

  return (
    <>
      <PageHeader
        title="Expediente clínico"
        description="Odontograma (FDI/Universal), periodontograma, evolución SOEP con firma y procedimientos realizados. Cada consulta queda auditada."
      />
      <Card>
        <div className="border-b border-line p-3">
          <div className="relative max-w-md">
            <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted" aria-hidden />
            <input
              type="search"
              value={texto}
              onChange={(e) => setTexto(e.target.value)}
              placeholder="Buscar paciente por nombre o documento"
              aria-label="Buscar paciente"
              className="min-h-11 w-full rounded-lg border border-line pl-9 pr-3 text-sm"
            />
          </div>
        </div>
        {isLoading && !data ? (
          <Skeleton className="m-4 h-40" />
        ) : error ? (
          <div className="p-4">
            <Alert tone="danger">{error}</Alert>
          </div>
        ) : data.data.length === 0 ? (
          <EmptyState icon={<Stethoscope className="size-8" />} title="Sin resultados" />
        ) : (
          <ul className="divide-y divide-line">
            {data.data.map((p) => (
              <li key={p.id} className="flex flex-wrap items-center gap-3 px-4 py-3 text-sm">
                <Link
                  to={`/pacientes/${p.id}?pestana=odontograma`}
                  className="font-medium text-brand-800 hover:underline"
                >
                  {p.apellidos}, {p.nombres}
                </Link>
                <span className="text-muted">
                  {p.numeroDocumento} · {p.edad} años
                </span>
                <PuntoAlerta
                  criticas={(p.alertas ?? []).filter((a) => a.severidad === "CRITICA").length}
                  pendiente={p.anamnesisPendiente}
                />
                <span className="ml-auto flex gap-3 text-xs">
                  <Link to={`/pacientes/${p.id}?pestana=evolucion`} className="text-brand-700 hover:underline">
                    Evolución
                  </Link>
                  <Link to={`/pacientes/${p.id}?pestana=periodontograma`} className="text-brand-700 hover:underline">
                    Periodontograma
                  </Link>
                </span>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </>
  );
};

export default Expediente;

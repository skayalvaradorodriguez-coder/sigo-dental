// Parámetros configurables (RNF-05 inactividad, RF-05 horas de bloqueo de notas, intentos de login).
import { useCallback, useState } from "react";
import { Alert, Button, Card, PageHeader, Skeleton } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useAccion } from "../../hooks/useAccion";
import { useApi } from "../../hooks/useApi";
import { getAjustes, guardarAjuste } from "../../services/sistemaService";

const FilaAjuste = ({ a, alGuardar }) => {
  const { recargar } = useAuth();
  const [valor, setValor] = useState(String(a.valor));
  const [guardado, setGuardado] = useState(false);
  const { ejecutar, enviando, error, reiniciar } = useAccion(guardarAjuste);
  const cambiado = valor !== String(a.valor);

  const guardar = async () => {
    try {
      await ejecutar(a.clave, Number(valor));
      setGuardado(true);
      alGuardar();
      await recargar(); // el tiempo de inactividad se refleja en la sesión actual
    } catch {
      // el mensaje queda en "error"
    }
  };

  return (
    <div className="grid gap-3 border-b border-line p-4 last:border-0 sm:grid-cols-[1fr_auto] sm:items-center">
      <div>
        <label htmlFor={a.clave} className="font-medium">
          {a.etiqueta}
        </label>
        <p className="text-sm text-muted">
          {a.ayuda} Rango {a.min}–{a.max}. Valor por defecto: {a.defecto}.
        </p>
        {error && <p className="mt-1 text-sm text-danger-700">{error}</p>}
        {guardado && !cambiado && !error && <p className="mt-1 text-sm text-ok-700">Guardado.</p>}
      </div>
      <div className="flex items-center gap-2">
        <input
          id={a.clave}
          type="number"
          inputMode="numeric"
          min={a.min}
          max={a.max}
          value={valor}
          onChange={(e) => {
            setValor(e.target.value);
            setGuardado(false);
            reiniciar();
          }}
          className="min-h-10 w-24 rounded-lg border border-line px-3 text-right"
        />
        <Button variant="secondary" disabled={!cambiado || valor === ""} loading={enviando} onClick={guardar}>
          Guardar
        </Button>
      </div>
    </div>
  );
};

const Ajustes = () => {
  const { data, isLoading, error, recargar } = useApi(useCallback(() => getAjustes(), []));
  return (
    <>
      <PageHeader
        title="Configuración"
        description="Parámetros de seguridad y reglas clínicas. Cada cambio queda registrado en la auditoría con el valor anterior."
      />
      <Card>
        {isLoading && !data && (
          <div className="space-y-3 p-4">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-14" />
            ))}
          </div>
        )}
        {error && (
          <div className="p-4">
            <Alert tone="danger">{error}</Alert>
          </div>
        )}
        {data?.map((a) => (
          <FilaAjuste key={`${a.clave}-${a.valor}`} a={a} alGuardar={recargar} />
        ))}
      </Card>
    </>
  );
};

export default Ajustes;

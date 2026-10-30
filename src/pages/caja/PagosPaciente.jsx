// Pestaña "Pagos" de la ficha: historial de cobros del paciente con sus comprobantes.
import { useCallback } from "react";
import { Alert, Skeleton } from "../../components/ui";
import { useApi } from "../../hooks/useApi";
import { getCobrosPaciente } from "../../services/cajaService";
import ListaCobros from "./ListaCobros";

const PagosPaciente = ({ pacienteId }) => {
  const { data, error, recargar } = useApi(useCallback(() => getCobrosPaciente(pacienteId), [pacienteId]));
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data) return <Skeleton className="h-40" />;
  return <ListaCobros cobros={data} puedeAnular={false} mostrarPaciente={false} alCambiar={recargar} />;
};

export default PagosPaciente;

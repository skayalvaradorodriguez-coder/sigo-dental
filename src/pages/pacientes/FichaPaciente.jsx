// Ficha del paciente: cabecera con alertas de alta visibilidad (RF-02), datos, anamnesis y citas.
import { ArrowLeft, CalendarPlus, Pencil } from "lucide-react";
import { useCallback, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { AlertasMedicas } from "../../components/AlertasMedicas";
import { Alert, Badge, Button, Card, EmptyState, SectionTitle, Spinner, Tabs } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useApi } from "../../hooks/useApi";
import { getCitas } from "../../services/citasService";
import { getPaciente } from "../../services/pacientesService";
import { ETIQUETA_CANAL, ETIQUETA_DOCUMENTO, ETIQUETA_ESTADO, ETIQUETA_SEXO, TONO_ESTADO } from "../../utils/etiquetas";
import { fmtFechaHora, sumarDias } from "../../utils/fechas";
import PanelAnamnesis from "./PanelAnamnesis";

const Campo = ({ etiqueta, valor }) => (
  <div>
    <dt className="text-xs text-muted">{etiqueta}</dt>
    <dd className="text-sm">{valor || "—"}</dd>
  </div>
);

const CitasDelPaciente = ({ pacienteId }) => {
  const { data, isLoading, error } = useApi(
    useCallback(
      () =>
        getCitas({
          pacienteId,
          desde: sumarDias(new Date(), -15).toISOString(),
          hasta: sumarDias(new Date(), 15).toISOString(),
        }),
      [pacienteId],
    ),
  );
  if (isLoading && !data) return <Spinner />;
  if (error) return <Alert tone="danger">{error}</Alert>;
  if (!data.length)
    return (
      <Card>
        <EmptyState title="Sin citas en los últimos y próximos 15 días" />
      </Card>
    );
  return (
    <Card>
      <ul className="divide-y divide-line">
        {data.map((c) => (
          <li key={c.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 text-sm">
            <span className="font-medium">{fmtFechaHora(c.inicio)}</span>
            <span>{c.procedimiento ?? "Sin procedimiento"}</span>
            <span className="text-muted">
              {c.odontologo} · {c.sillon}
            </span>
            <Badge tone={TONO_ESTADO[c.estado]}>{ETIQUETA_ESTADO[c.estado]}</Badge>
          </li>
        ))}
      </ul>
    </Card>
  );
};

const FichaPaciente = () => {
  const { id } = useParams();
  const { puede } = useAuth();
  const navegar = useNavigate();
  const ubicacion = useLocation();
  const [pestana, setPestana] = useState("datos");
  const { data: p, isLoading, error, recargar } = useApi(useCallback(() => getPaciente(id), [id]));

  if (isLoading && !p) return <Spinner label="Cargando ficha…" />;
  if (error || !p)
    return (
      <Alert tone="danger" title="No se pudo cargar el paciente">
        {error || "Puede que no exista o no tenga permiso."}
      </Alert>
    );

  const pestanas = [
    { id: "datos", label: "Datos personales" },
    ...(puede("clinica.leer", "anamnesis.escribir") ? [{ id: "anamnesis", label: "Anamnesis" }] : []),
    ...(puede("citas.leer") ? [{ id: "citas", label: "Citas" }] : []),
  ];

  return (
    <div className="space-y-5">
      <Link to="/pacientes" className="inline-flex items-center gap-1 text-sm text-brand-700 hover:underline">
        <ArrowLeft className="size-4" /> Pacientes
      </Link>
      {ubicacion.state?.guardado && <Alert tone="ok">Ficha guardada correctamente.</Alert>}

      <Card className="p-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">
              {p.nombres} {p.apellidos}
            </h1>
            <p className="mt-1 text-sm text-muted">
              {ETIQUETA_DOCUMENTO[p.tipoDocumento]} {p.numeroDocumento} · {p.edad} años · {ETIQUETA_SEXO[p.sexo]}
              {p.convenio && <> · {p.convenio}</>}
            </p>
            {p.esMenor && (
              <p className="mt-1">
                <Badge tone="brand">Menor de edad · tutor: {p.tutor?.nombre}</Badge>
              </p>
            )}
          </div>
          <div className="flex gap-2">
            {puede("citas.escribir") && (
              <Button variant="secondary" onClick={() => navegar(`/agenda?nuevo=${p.id}`)}>
                <CalendarPlus className="size-4" /> Agendar
              </Button>
            )}
            {puede("pacientes.escribir") && (
              <Button variant="secondary" onClick={() => navegar(`/pacientes/${p.id}/editar`)}>
                <Pencil className="size-4" /> Editar
              </Button>
            )}
          </div>
        </div>
        <div className="mt-4">
          <AlertasMedicas alertas={p.alertas} anamnesisPendiente={p.anamnesisPendiente} />
        </div>
      </Card>

      <Tabs value={pestana} onChange={setPestana} tabs={pestanas} />

      {pestana === "datos" && (
        <div className="grid gap-4 lg:grid-cols-2">
          <Card className="p-5">
            <SectionTitle>Contacto</SectionTitle>
            <dl className="grid gap-3 sm:grid-cols-2">
              <Campo etiqueta="Teléfono" valor={p.telefono} />
              <Campo etiqueta="Correo" valor={p.email} />
              <Campo etiqueta="Dirección" valor={p.direccion} />
              <Campo etiqueta="Fecha de nacimiento" valor={p.fechaNacimiento} />
              <Campo etiqueta="Canal preferido" valor={ETIQUETA_CANAL[p.canalPreferido]} />
              <Campo
                etiqueta="Autoriza mensajes"
                valor={
                  p.autorizaMensajes ? "Sí" : <span className="text-warn-700">No (no se envían recordatorios)</span>
                }
              />
            </dl>
          </Card>
          <Card className="p-5">
            <SectionTitle>Emergencia y convenio</SectionTitle>
            <dl className="grid gap-3 sm:grid-cols-2">
              <Campo etiqueta="Contacto de emergencia" valor={p.emergenciaNombre} />
              <Campo etiqueta="Teléfono de emergencia" valor={p.emergenciaTelefono} />
              <Campo etiqueta="Parentesco" valor={p.emergenciaParentesco} />
              <Campo etiqueta="Convenio" valor={p.convenio ?? "Particular"} />
              <Campo etiqueta="N.º de afiliado" valor={p.numeroAfiliado} />
            </dl>
          </Card>
          {p.tutor && (
            <Card className="p-5 lg:col-span-2">
              <SectionTitle>Tutor legal</SectionTitle>
              <dl className="grid gap-3 sm:grid-cols-3">
                <Campo etiqueta="Nombre" valor={p.tutor.nombre} />
                <Campo etiqueta="Parentesco" valor={p.tutor.parentesco} />
                <Campo
                  etiqueta="Documento"
                  valor={`${ETIQUETA_DOCUMENTO[p.tutor.tipoDocumento]} ${p.tutor.numeroDocumento}`}
                />
                <Campo etiqueta="Teléfono" valor={p.tutor.telefono} />
                <Campo etiqueta="Correo" valor={p.tutor.email} />
              </dl>
            </Card>
          )}
          <p className="text-xs text-muted lg:col-span-2">
            Registrado el {fmtFechaHora(p.creadoEn)}. La consulta de esta ficha quedó registrada en la auditoría.
          </p>
        </div>
      )}
      {pestana === "anamnesis" && <PanelAnamnesis paciente={p} alGuardar={recargar} />}
      {pestana === "citas" && <CitasDelPaciente pacienteId={p.id} />}
    </div>
  );
};

export default FichaPaciente;

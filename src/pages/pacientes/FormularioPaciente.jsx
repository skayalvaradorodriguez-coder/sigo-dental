// Alta y edición de la ficha única (RF-01). El backend repite TODAS estas validaciones;
// aquí sólo se adelantan para dar respuesta inmediata.
import { ArrowLeft } from "lucide-react";
import { useCallback, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Alert, Button, Card, Checkbox, Input, PageHeader, SectionTitle, Select, Spinner } from "../../components/ui";
import { useAccion } from "../../hooks/useAccion";
import { useApi } from "../../hooks/useApi";
import { getConvenios } from "../../services/catalogosService";
import { actualizarPaciente, crearPaciente, getPaciente } from "../../services/pacientesService";
import { ETIQUETA_CANAL, ETIQUETA_DOCUMENTO, ETIQUETA_SEXO } from "../../utils/etiquetas";
import { edadDesde } from "../../utils/fechas";

const TELEFONO = /^\+?[0-9 ()-]{7,20}$/;
const DOCUMENTO = /^[A-Za-z0-9-]{5,20}$/;
const EMAIL = /^\S+@\S+\.\S+$/;

const VACIO = {
  tipoDocumento: "CEDULA",
  numeroDocumento: "",
  nombres: "",
  apellidos: "",
  fechaNacimiento: "",
  sexo: "",
  telefono: "",
  email: "",
  direccion: "",
  emergenciaNombre: "",
  emergenciaTelefono: "",
  emergenciaParentesco: "",
  canalPreferido: "WHATSAPP",
  autorizaMensajes: false,
  convenioId: "",
  numeroAfiliado: "",
  t_nombre: "",
  t_tipoDocumento: "CEDULA",
  t_numeroDocumento: "",
  t_parentesco: "",
  t_telefono: "",
  t_email: "",
};

const desdePaciente = (p) => ({
  ...VACIO,
  ...Object.fromEntries(
    Object.keys(VACIO)
      .filter((k) => !k.startsWith("t_"))
      .map((k) => [k, p[k] ?? VACIO[k]]),
  ),
  t_nombre: p.tutor?.nombre ?? "",
  t_tipoDocumento: p.tutor?.tipoDocumento ?? "CEDULA",
  t_numeroDocumento: p.tutor?.numeroDocumento ?? "",
  t_parentesco: p.tutor?.parentesco ?? "",
  t_telefono: p.tutor?.telefono ?? "",
  t_email: p.tutor?.email ?? "",
});

const validar = (f, menor) => {
  const e = {};
  if (!DOCUMENTO.test(f.numeroDocumento)) e.numeroDocumento = "5 a 20 letras, números o guiones.";
  if (f.nombres.trim().length < 2) e.nombres = "Ingrese los nombres.";
  if (f.apellidos.trim().length < 2) e.apellidos = "Ingrese los apellidos.";
  const edad = edadDesde(f.fechaNacimiento);
  if (edad === null) e.fechaNacimiento = "Ingrese la fecha de nacimiento.";
  else if (edad < 0) e.fechaNacimiento = "La fecha no puede ser futura.";
  else if (edad > 120) e.fechaNacimiento = "Revise la fecha.";
  if (!f.sexo) e.sexo = "Seleccione una opción.";
  if (f.telefono && !TELEFONO.test(f.telefono)) e.telefono = "Teléfono inválido.";
  if (f.email && !EMAIL.test(f.email)) e.email = "Correo inválido.";
  if (f.emergenciaNombre.trim().length < 3) e.emergenciaNombre = "Obligatorio.";
  if (!TELEFONO.test(f.emergenciaTelefono)) e.emergenciaTelefono = "Teléfono inválido.";
  if (f.autorizaMensajes && f.canalPreferido === "EMAIL" && !f.email)
    e.email = "Necesario para recibir recordatorios por correo.";
  if (f.autorizaMensajes && f.canalPreferido !== "EMAIL" && !f.telefono)
    e.telefono = "Necesario para recibir recordatorios.";
  if (menor) {
    if (f.t_nombre.trim().length < 3) e.t_nombre = "Obligatorio para menores de edad.";
    if (!DOCUMENTO.test(f.t_numeroDocumento)) e.t_numeroDocumento = "Documento inválido.";
    if (f.t_parentesco.trim().length < 2) e.t_parentesco = "Indique el parentesco.";
    if (!TELEFONO.test(f.t_telefono)) e.t_telefono = "Teléfono inválido.";
    if (f.t_email && !EMAIL.test(f.t_email)) e.t_email = "Correo inválido.";
  }
  return e;
};

const guardar = ({ id, datos }) => (id ? actualizarPaciente(id, datos) : crearPaciente(datos));

const Formulario = ({ inicial }) => {
  const navegar = useNavigate();
  const [f, setF] = useState(inicial ? desdePaciente(inicial) : VACIO);
  const [errores, setErrores] = useState({});
  const [aviso, setAviso] = useState("");
  const { ejecutar, enviando, error } = useAccion(guardar);
  const convenios = useApi(useCallback(() => getConvenios(), []));
  const edad = edadDesde(f.fechaNacimiento);
  const menor = edad !== null && edad >= 0 && edad < 18;
  const poner = (k, v) => setF((x) => ({ ...x, [k]: v }));

  const enviar = async (e) => {
    e.preventDefault();
    const errs = validar(f, menor);
    setErrores(errs);
    setAviso(Object.keys(errs).length ? "Revise los campos marcados en rojo." : "");
    if (Object.keys(errs).length) return;
    const datos = {
      tipoDocumento: f.tipoDocumento,
      numeroDocumento: f.numeroDocumento.trim(),
      nombres: f.nombres.trim(),
      apellidos: f.apellidos.trim(),
      fechaNacimiento: f.fechaNacimiento,
      sexo: f.sexo,
      telefono: f.telefono.trim(),
      email: f.email.trim(),
      direccion: f.direccion.trim(),
      emergenciaNombre: f.emergenciaNombre.trim(),
      emergenciaTelefono: f.emergenciaTelefono.trim(),
      emergenciaParentesco: f.emergenciaParentesco.trim(),
      canalPreferido: f.canalPreferido,
      autorizaMensajes: f.autorizaMensajes,
      convenioId: f.convenioId,
      numeroAfiliado: f.convenioId ? f.numeroAfiliado.trim() : "",
    };
    if (menor)
      datos.tutor = {
        nombre: f.t_nombre.trim(),
        tipoDocumento: f.t_tipoDocumento,
        numeroDocumento: f.t_numeroDocumento.trim(),
        parentesco: f.t_parentesco.trim(),
        telefono: f.t_telefono.trim(),
        email: f.t_email.trim(),
      };
    else if (inicial?.tutor) datos.quitarTutor = true; // ya es mayor de edad: se retira el tutor
    try {
      const p = await ejecutar({ id: inicial?.id, datos });
      navegar(`/pacientes/${p.id}`, { state: { guardado: true } });
    } catch (err) {
      if (err.campos) setErrores(err.campos);
    }
  };

  const campo = (k) => ({ value: String(f[k]), error: errores[k], onChange: (e) => poner(k, e.target.value) });
  const opciones = (mapa) =>
    Object.entries(mapa).map(([k, v]) => (
      <option key={k} value={k}>
        {v}
      </option>
    ));

  return (
    <form onSubmit={enviar} noValidate className="space-y-5">
      <Link
        to={inicial ? `/pacientes/${inicial.id}` : "/pacientes"}
        className="inline-flex items-center gap-1 text-sm text-brand-700 hover:underline"
      >
        <ArrowLeft className="size-4" /> Volver
      </Link>
      <PageHeader
        title={inicial ? `Editar: ${inicial.nombres} ${inicial.apellidos}` : "Nuevo paciente"}
        description="Los campos se validan también en el servidor. Los cambios quedan auditados con el valor anterior."
      />
      {(aviso || error) && <Alert tone="danger">{error || aviso}</Alert>}

      <Card className="p-5">
        <SectionTitle>Identificación</SectionTitle>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Select
            label="Tipo de documento"
            value={f.tipoDocumento}
            onChange={(e) => poner("tipoDocumento", e.target.value)}
          >
            {opciones(ETIQUETA_DOCUMENTO)}
          </Select>
          <Input label="Número de documento" {...campo("numeroDocumento")} />
          <Input
            label="Fecha de nacimiento"
            type="date"
            max={new Date().toISOString().slice(0, 10)}
            {...campo("fechaNacimiento")}
            hint={edad !== null && edad >= 0 ? `${edad} años${menor ? " · menor de edad" : ""}` : undefined}
          />
          <Input label="Nombres" {...campo("nombres")} />
          <Input label="Apellidos" {...campo("apellidos")} />
          <Select label="Sexo" value={f.sexo} error={errores.sexo} onChange={(e) => poner("sexo", e.target.value)}>
            <option value="">Seleccione…</option>
            {opciones(ETIQUETA_SEXO)}
          </Select>
        </div>
      </Card>

      {menor && (
        <Card className="border-brand-200 p-5">
          <SectionTitle>Tutor legal (obligatorio: paciente menor de edad)</SectionTitle>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <Input label="Nombre completo" {...campo("t_nombre")} />
            <Input label="Parentesco" placeholder="Madre, padre, tutor…" {...campo("t_parentesco")} />
            <Select
              label="Tipo de documento"
              value={f.t_tipoDocumento}
              onChange={(e) => poner("t_tipoDocumento", e.target.value)}
            >
              {opciones(ETIQUETA_DOCUMENTO)}
            </Select>
            <Input label="Número de documento" {...campo("t_numeroDocumento")} />
            <Input label="Teléfono" type="tel" {...campo("t_telefono")} />
            <Input label="Correo (opcional)" type="email" {...campo("t_email")} />
          </div>
        </Card>
      )}

      <Card className="p-5">
        <SectionTitle>Contacto y recordatorios</SectionTitle>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Input label="Teléfono" type="tel" {...campo("telefono")} />
          <Input label="Correo" type="email" {...campo("email")} />
          <Input label="Dirección" {...campo("direccion")} />
          <Select
            label="Canal preferido"
            value={f.canalPreferido}
            onChange={(e) => poner("canalPreferido", e.target.value)}
          >
            {opciones(ETIQUETA_CANAL)}
          </Select>
          <div className="flex items-end pb-2 sm:col-span-2">
            <Checkbox
              label="El paciente autoriza recibir confirmaciones y recordatorios por el canal elegido"
              hint="Sin esta autorización no se programan mensajes."
              checked={f.autorizaMensajes}
              onChange={(v) => poner("autorizaMensajes", v)}
            />
          </div>
        </div>
      </Card>

      <Card className="p-5">
        <SectionTitle>Contacto de emergencia</SectionTitle>
        <div className="grid gap-4 sm:grid-cols-3">
          <Input label="Nombre" {...campo("emergenciaNombre")} />
          <Input label="Teléfono" type="tel" {...campo("emergenciaTelefono")} />
          <Input label="Parentesco (opcional)" {...campo("emergenciaParentesco")} />
        </div>
      </Card>

      <Card className="p-5">
        <SectionTitle>Seguro dental o convenio</SectionTitle>
        <div className="grid gap-4 sm:grid-cols-2">
          <Select label="Convenio" value={f.convenioId} onChange={(e) => poner("convenioId", e.target.value)}>
            <option value="">Particular (sin convenio)</option>
            {convenios.data?.map((c) => (
              <option key={c.id} value={c.id}>
                {c.nombre}
              </option>
            ))}
          </Select>
          <Input label="N.º de afiliado" disabled={!f.convenioId} {...campo("numeroAfiliado")} />
        </div>
      </Card>

      <div className="flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={() => navegar(-1)}>
          Cancelar
        </Button>
        <Button type="submit" loading={enviando}>
          {inicial ? "Guardar cambios" : "Registrar paciente"}
        </Button>
      </div>
    </form>
  );
};

const FormularioPaciente = () => {
  const { id } = useParams();
  const cargar = useCallback(() => (id ? getPaciente(id) : Promise.resolve(null)), [id]);
  const { data, isLoading, error } = useApi(cargar);
  if (id && isLoading) return <Spinner />;
  if (id && error) return <Alert tone="danger">{error}</Alert>;
  return <Formulario key={id ?? "nuevo"} inicial={data} />;
};

export default FormularioPaciente;

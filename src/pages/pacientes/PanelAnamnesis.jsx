// Anamnesis versionada (RF-02): cada guardado crea una versión nueva; las anteriores no se modifican
// (la tabla tiene un trigger de "sólo inserción"). Las alertas usan siempre la versión vigente.
import { History, Plus, Trash2 } from "lucide-react";
import { useCallback, useState } from "react";
import {
  Alert,
  Badge,
  Button,
  Card,
  EmptyState,
  Input,
  SectionTitle,
  Select,
  Skeleton,
  Textarea,
  YesNo,
} from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useAccion } from "../../hooks/useAccion";
import { useApi } from "../../hooks/useApi";
import { getAnamnesis, registrarAnamnesis } from "../../services/pacientesService";
import { ETIQUETA_ALERGIA } from "../../utils/etiquetas";
import { fmtFechaHora } from "../../utils/fechas";

const Fila = ({ etiqueta, children }) => (
  <div className="grid gap-1 border-b border-line py-2.5 last:border-0 sm:grid-cols-[220px_1fr]">
    <dt className="text-sm text-muted">{etiqueta}</dt>
    <dd className="text-sm">{children}</dd>
  </div>
);

const siNo = (v) => (v === null || v === undefined ? "—" : v ? "Sí" : "No");

const Vista = ({ h }) => (
  <dl>
    <Fila etiqueta="Enfermedades sistémicas">
      {h.enfermedadesSistemicas.length ? h.enfermedadesSistemicas.join(", ") : "Niega"}
    </Fila>
    <Fila etiqueta="Alergias">
      {h.alergias.length ? (
        <ul className="space-y-1">
          {h.alergias.map((a, i) => (
            <li key={i}>
              <span
                className={a.tipo === "MEDICAMENTO" || a.tipo === "MATERIAL" ? "font-semibold text-danger-700" : ""}
              >
                {a.sustancia}
              </span>{" "}
              <span className="text-muted">
                ({ETIQUETA_ALERGIA[a.tipo]}
                {a.reaccion ? ` · ${a.reaccion}` : ""})
              </span>
            </li>
          ))}
        </ul>
      ) : (
        "Niega alergias"
      )}
    </Fila>
    <Fila etiqueta="Coagulopatía">
      <span className={h.coagulopatia ? "font-semibold text-danger-700" : ""}>{siNo(h.coagulopatia)}</span>
    </Fila>
    <Fila etiqueta="Anticoagulado">
      <span className={h.anticoagulado ? "font-semibold text-danger-700" : ""}>{siNo(h.anticoagulado)}</span>
    </Fila>
    <Fila etiqueta="Antecedentes quirúrgicos">{h.antecedentesQuirurgicos || "Niega"}</Fila>
    <Fila etiqueta="Medicación actual">{h.medicacionActual || "Ninguna"}</Fila>
    {h.embarazo !== null && <Fila etiqueta="Embarazo">{siNo(h.embarazo)}</Fila>}
    {h.otraCondicionCritica && (
      <Fila etiqueta="Otra condición crítica">
        <span className="font-semibold text-danger-700">{h.otraCondicionCritica}</span>
      </Fila>
    )}
    {h.observaciones && <Fila etiqueta="Observaciones">{h.observaciones}</Fila>}
    <Fila etiqueta="Registrada por">{h.registradoPor}</Fila>
  </dl>
);

const guardar = ({ id, datos }) => registrarAnamnesis(id, datos);

const FormularioAnamnesis = ({ paciente, base, alTerminar }) => {
  const [enfermedades, setEnfermedades] = useState(base?.enfermedadesSistemicas ?? []);
  const [nuevaEnfermedad, setNuevaEnfermedad] = useState("");
  const [alergias, setAlergias] = useState(base?.alergias.map((a) => ({ ...a, reaccion: a.reaccion ?? "" })) ?? []);
  // null = sin responder: el formulario obliga a contestar Sí o No (no se asume "No").
  const [coagulopatia, setCoagulopatia] = useState(base ? base.coagulopatia : null);
  const [anticoagulado, setAnticoagulado] = useState(base ? base.anticoagulado : null);
  const [embarazo, setEmbarazo] = useState(base?.embarazo ?? null);
  const [texto, setTexto] = useState({
    antecedentesQuirurgicos: base?.antecedentesQuirurgicos ?? "",
    medicacionActual: base?.medicacionActual ?? "",
    otraCondicionCritica: base?.otraCondicionCritica ?? "",
    observaciones: base?.observaciones ?? "",
  });
  const [errores, setErrores] = useState({});
  const { ejecutar, enviando, error } = useAccion(guardar);
  const preguntaEmbarazo = paciente.sexo === "FEMENINO" && paciente.edad >= 10;

  const enviar = async (e) => {
    e.preventDefault();
    const errs = {};
    if (coagulopatia === null) errs.coagulopatia = "Responda Sí o No.";
    if (anticoagulado === null) errs.anticoagulado = "Responda Sí o No.";
    alergias.forEach((a, i) => {
      if (a.sustancia.trim().length < 2) errs[`al${i}`] = "Indique la sustancia.";
    });
    setErrores(errs);
    if (Object.keys(errs).length) return;
    const datos = {
      enfermedadesSistemicas: enfermedades,
      alergias: alergias.map((a) => ({ sustancia: a.sustancia.trim(), tipo: a.tipo, reaccion: a.reaccion.trim() })),
      coagulopatia,
      anticoagulado,
      antecedentesQuirurgicos: texto.antecedentesQuirurgicos.trim(),
      medicacionActual: texto.medicacionActual.trim(),
      otraCondicionCritica: texto.otraCondicionCritica.trim(),
      observaciones: texto.observaciones.trim(),
    };
    if (preguntaEmbarazo && embarazo !== null) datos.embarazo = embarazo;
    try {
      await ejecutar({ id: paciente.id, datos });
      alTerminar(true);
    } catch (err) {
      if (err.campos) setErrores(err.campos);
    }
  };

  const agregarEnfermedad = () => {
    const d = nuevaEnfermedad.trim();
    if (d && !enfermedades.includes(d)) setEnfermedades([...enfermedades, d]);
    setNuevaEnfermedad("");
  };
  const cambiarAlergia = (i, campo, valor) =>
    setAlergias(alergias.map((x, j) => (j === i ? { ...x, [campo]: valor } : x)));

  return (
    <form onSubmit={enviar} noValidate className="space-y-6">
      <Alert tone="info">
        Cada vez que guarda se crea una <strong>versión nueva</strong>; las anteriores se conservan sin cambios. Las
        listas vacías se registran como “niega”.
      </Alert>
      {error && <Alert tone="danger">{error}</Alert>}
      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-5">
          <div className="grid gap-4 sm:grid-cols-2">
            <YesNo
              label="¿Coagulopatía?"
              value={coagulopatia}
              onChange={setCoagulopatia}
              error={errores.coagulopatia}
              danger
            />
            <YesNo
              label="¿Toma anticoagulantes?"
              value={anticoagulado}
              onChange={setAnticoagulado}
              error={errores.anticoagulado}
              danger
            />
            {preguntaEmbarazo && <YesNo label="¿Embarazo?" value={embarazo} onChange={setEmbarazo} />}
          </div>
          <div>
            <p className="mb-1 text-sm font-medium">Enfermedades sistémicas</p>
            <div className="flex gap-2">
              <input
                value={nuevaEnfermedad}
                onChange={(e) => setNuevaEnfermedad(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    agregarEnfermedad();
                  }
                }}
                placeholder="Ej.: Diabetes tipo 2"
                aria-label="Agregar enfermedad sistémica"
                className="min-h-10 flex-1 rounded-lg border border-line px-3 text-sm"
              />
              <Button type="button" variant="secondary" onClick={agregarEnfermedad}>
                <Plus className="size-4" /> Agregar
              </Button>
            </div>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {enfermedades.length === 0 && <span className="text-sm text-muted">Niega</span>}
              {enfermedades.map((d) => (
                <button
                  type="button"
                  key={d}
                  onClick={() => setEnfermedades(enfermedades.filter((x) => x !== d))}
                  className="rounded-md border border-line bg-canvas px-2 py-1 text-xs hover:border-danger-600"
                  title="Quitar"
                >
                  {d} ✕
                </button>
              ))}
            </div>
          </div>
          <Textarea
            label="Antecedentes quirúrgicos"
            value={texto.antecedentesQuirurgicos}
            onChange={(e) => setTexto({ ...texto, antecedentesQuirurgicos: e.target.value })}
            placeholder="Vacío = niega"
          />
          <Textarea
            label="Medicación actual"
            value={texto.medicacionActual}
            onChange={(e) => setTexto({ ...texto, medicacionActual: e.target.value })}
          />
        </div>
        <div className="space-y-5">
          <div>
            <div className="mb-1 flex items-center justify-between">
              <p className="text-sm font-medium">Alergias</p>
              <Button
                type="button"
                variant="ghost"
                onClick={() => setAlergias([...alergias, { sustancia: "", tipo: "MEDICAMENTO", reaccion: "" }])}
              >
                <Plus className="size-4" /> Agregar alergia
              </Button>
            </div>
            {alergias.length === 0 && <p className="text-sm text-muted">Niega alergias</p>}
            <div className="space-y-2">
              {alergias.map((a, i) => (
                <div key={i} className="grid gap-2 rounded-lg border border-line p-2 sm:grid-cols-[1fr_150px_1fr_auto]">
                  <Input
                    label="Sustancia"
                    value={a.sustancia}
                    error={errores[`al${i}`]}
                    onChange={(e) => cambiarAlergia(i, "sustancia", e.target.value)}
                  />
                  <Select label="Tipo" value={a.tipo} onChange={(e) => cambiarAlergia(i, "tipo", e.target.value)}>
                    {Object.entries(ETIQUETA_ALERGIA).map(([k, v]) => (
                      <option key={k} value={k}>
                        {v}
                      </option>
                    ))}
                  </Select>
                  <Input
                    label="Reacción"
                    value={a.reaccion}
                    onChange={(e) => cambiarAlergia(i, "reaccion", e.target.value)}
                  />
                  <button
                    type="button"
                    onClick={() => setAlergias(alergias.filter((_, j) => j !== i))}
                    className="self-end rounded-lg p-2.5 text-muted hover:bg-danger-50 hover:text-danger-700"
                    aria-label="Quitar alergia"
                  >
                    <Trash2 className="size-4" />
                  </button>
                </div>
              ))}
            </div>
            <p className="mt-1 text-xs text-muted">
              Las alergias a medicamentos o materiales dentales se muestran como alerta crítica.
            </p>
          </div>
          <Input
            label="Otra condición crítica (opcional)"
            placeholder="Ej.: marcapasos, endocarditis previa"
            value={texto.otraCondicionCritica}
            onChange={(e) => setTexto({ ...texto, otraCondicionCritica: e.target.value })}
          />
          <Textarea
            label="Observaciones"
            value={texto.observaciones}
            onChange={(e) => setTexto({ ...texto, observaciones: e.target.value })}
          />
        </div>
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={() => alTerminar(false)}>
          Cancelar
        </Button>
        <Button type="submit" loading={enviando}>
          Guardar anamnesis
        </Button>
      </div>
    </form>
  );
};

const PanelAnamnesis = ({ paciente, alGuardar }) => {
  const { puede } = useAuth();
  const puedeEscribir = puede("anamnesis.escribir");
  const [editando, setEditando] = useState(false);
  const [version, setVersion] = useState(null);
  const {
    data: versiones,
    isLoading,
    error,
    recargar,
  } = useApi(useCallback(() => getAnamnesis(paciente.id), [paciente.id]));

  if (isLoading && !versiones)
    return (
      <Card className="space-y-2 p-5">
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-8" />
        ))}
      </Card>
    );
  if (error) return <Alert tone="danger">{error}</Alert>;

  const vigente = versiones[0];
  const mostrada = versiones.find((v) => v.version === version) ?? vigente;
  const terminar = (guardado) => {
    setEditando(false);
    if (guardado) {
      setVersion(null);
      recargar();
      alGuardar(); // la cabecera recalcula las alertas
    }
  };

  if (editando)
    return (
      <Card className="p-5">
        <FormularioAnamnesis paciente={paciente} base={vigente} alTerminar={terminar} />
      </Card>
    );
  if (!vigente)
    return (
      <Card>
        <EmptyState title="Anamnesis pendiente">
          <p>Es obligatoria (RF-02) y debe registrarse antes de cualquier procedimiento.</p>
          {puedeEscribir && (
            <Button className="mt-4" onClick={() => setEditando(true)}>
              Registrar anamnesis
            </Button>
          )}
        </EmptyState>
      </Card>
    );

  return (
    <div className="grid gap-4 lg:grid-cols-[1fr_260px]">
      <Card className="p-5">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <div>
            <SectionTitle>
              Versión {mostrada.version}
              {mostrada === vigente ? " · vigente" : ""}
            </SectionTitle>
            <p className="-mt-2 text-xs text-muted">Registrada el {fmtFechaHora(mostrada.registradoEn)}</p>
          </div>
          {puedeEscribir && (
            <Button variant="secondary" onClick={() => setEditando(true)}>
              Actualizar anamnesis
            </Button>
          )}
        </div>
        {mostrada !== vigente && (
          <div className="mb-3">
            <Alert tone="warn">Está viendo una versión anterior. Las alertas usan siempre la vigente.</Alert>
          </div>
        )}
        <Vista h={mostrada} />
      </Card>
      <Card className="h-fit p-4">
        <p className="mb-2 flex items-center gap-2 text-sm font-medium">
          <History className="size-4" /> Historial
        </p>
        <ul className="space-y-1">
          {versiones.map((v) => (
            <li key={v.id}>
              <button
                onClick={() => setVersion(v.version)}
                className={`w-full rounded-lg px-2 py-1.5 text-left text-sm ${v === mostrada ? "bg-brand-50 text-brand-800" : "hover:bg-canvas"}`}
              >
                Versión {v.version} {v === vigente && <Badge tone="ok">Vigente</Badge>}
                <span className="block text-xs text-muted">{fmtFechaHora(v.registradoEn)}</span>
              </button>
            </li>
          ))}
        </ul>
      </Card>
    </div>
  );
};

export default PanelAnamnesis;

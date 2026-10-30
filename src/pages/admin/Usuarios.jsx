// Gestión de usuarios: alta, rol, desactivación y restablecimiento de contraseña / MFA (Etapa 1).
import { Copy, KeyRound, Pencil, Plus, ShieldOff, Users } from "lucide-react";
import { useCallback, useState } from "react";
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
} from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import { useAccion } from "../../hooks/useAccion";
import { useApi } from "../../hooks/useApi";
import * as api from "../../services/sistemaService";
import { ETIQUETA_ROL } from "../../utils/etiquetas";
import { fmtRelativo } from "../../utils/fechas";

const ROLES = Object.keys(ETIQUETA_ROL);

const PasswordTemporal = ({ email, password }) => {
  const [copiada, setCopiada] = useState(false);
  return (
    <div className="space-y-3">
      <Alert tone="warn" title="Se muestra una sola vez">
        Entréguela por un canal seguro. Al ingresar, el usuario deberá cambiarla y configurar MFA.
      </Alert>
      <div className="rounded-lg border border-line bg-canvas p-3">
        <p className="text-xs text-muted">{email}</p>
        <div className="mt-1 flex items-center justify-between gap-2">
          <code className="font-mono text-lg">{password}</code>
          <Button variant="ghost" onClick={() => navigator.clipboard?.writeText(password).then(() => setCopiada(true))}>
            <Copy className="size-4" /> {copiada ? "Copiada" : "Copiar"}
          </Button>
        </div>
      </div>
    </div>
  );
};

const guardarUsuario = async (usuario, f) => {
  if (usuario) {
    const cambios = { nombre: f.nombre, rol: f.rol, registroProfesional: f.registroProfesional, activo: f.activo };
    return { usuario: await api.actualizarUsuario(usuario.id, cambios) };
  }
  return api.crearUsuario({ email: f.email, nombre: f.nombre, rol: f.rol, registroProfesional: f.registroProfesional });
};

const FormularioUsuario = ({ usuario, alTerminar }) => {
  const [f, setF] = useState({
    email: usuario?.email ?? "",
    nombre: usuario?.nombre ?? "",
    rol: usuario?.rol ?? "RECEPCIONISTA",
    registroProfesional: usuario?.registroProfesional ?? "",
    activo: usuario?.activo ?? true,
  });
  const [errores, setErrores] = useState({});
  const { ejecutar, enviando, error } = useAccion(guardarUsuario);

  const enviar = async (e) => {
    e.preventDefault();
    const errs = {};
    if (!usuario && !/^\S+@\S+\.\S+$/.test(f.email)) errs.email = "Ingrese un correo válido.";
    if (f.nombre.trim().length < 3) errs.nombre = "Ingrese el nombre completo.";
    if (f.rol === "ODONTOLOGO" && !f.registroProfesional.trim())
      errs.registroProfesional = "Obligatorio para odontólogos.";
    setErrores(errs);
    if (Object.keys(errs).length) return;
    try {
      alTerminar(await ejecutar(usuario, f));
    } catch (err) {
      if (err.campos) setErrores(err.campos);
    }
  };

  return (
    <form onSubmit={enviar} className="space-y-4" noValidate>
      {error && <Alert tone="danger">{error}</Alert>}
      <Input
        label="Correo electrónico"
        type="email"
        value={f.email}
        disabled={!!usuario}
        error={errores.email}
        onChange={(e) => setF({ ...f, email: e.target.value })}
      />
      <Input
        label="Nombre completo"
        value={f.nombre}
        error={errores.nombre}
        onChange={(e) => setF({ ...f, nombre: e.target.value })}
      />
      <Select label="Rol" value={f.rol} onChange={(e) => setF({ ...f, rol: e.target.value })}>
        {ROLES.map((r) => (
          <option key={r} value={r}>
            {ETIQUETA_ROL[r]}
          </option>
        ))}
      </Select>
      {f.rol === "ODONTOLOGO" && (
        <Input
          label="Registro profesional"
          value={f.registroProfesional}
          error={errores.registroProfesional}
          hint="Se mostrará al firmar notas clínicas."
          onChange={(e) => setF({ ...f, registroProfesional: e.target.value })}
        />
      )}
      {usuario && (
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={f.activo}
            onChange={(e) => setF({ ...f, activo: e.target.checked })}
            className="size-4 accent-brand-700"
          />
          Usuario activo (al desactivarlo se cierran sus sesiones)
        </label>
      )}
      <div className="flex justify-end gap-2 pt-2">
        <Button type="submit" loading={enviando}>
          {usuario ? "Guardar cambios" : "Crear usuario"}
        </Button>
      </div>
    </form>
  );
};

const ConfirmarReset = ({ dialogo, alTerminar }) => {
  const restablecer = useCallback(
    () =>
      dialogo.tipo === "reset-password"
        ? api.restablecerPassword(dialogo.usuario.id)
        : api.restablecerMFA(dialogo.usuario.id),
    [dialogo],
  );
  const { ejecutar, enviando, error } = useAccion(restablecer);
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted">
        {dialogo.tipo === "reset-password"
          ? "Se generará una contraseña temporal y se cerrarán todas las sesiones abiertas de "
          : "Se eliminará la configuración de MFA y se cerrarán las sesiones de "}
        <strong className="text-ink">{dialogo.usuario.nombre}</strong>.
      </p>
      {error && <Alert tone="danger">{error}</Alert>}
      <div className="flex justify-end">
        <Button
          variant="danger"
          loading={enviando}
          onClick={() =>
            ejecutar()
              .then((r) => alTerminar(r?.passwordTemporal))
              .catch(() => {})
          }
        >
          Confirmar
        </Button>
      </div>
    </div>
  );
};

const TITULOS = {
  crear: "Nuevo usuario",
  editar: "Editar usuario",
  "reset-password": "Restablecer contraseña",
  "reset-mfa": "Restablecer MFA",
  temporal: "Contraseña temporal",
};

const Usuarios = () => {
  const { usuario: yo } = useAuth();
  const [dialogo, setDialogo] = useState(null);
  const [filtro, setFiltro] = useState("");
  const { data, isLoading, error, recargar } = useApi(useCallback(() => api.getUsuarios(), []));
  const filas = (data ?? []).filter((u) => `${u.nombre} ${u.email}`.toLowerCase().includes(filtro.toLowerCase()));

  return (
    <>
      <PageHeader
        title="Usuarios"
        description="Alta de usuarios, asignación de roles y restablecimiento de credenciales. Cada cambio queda auditado."
        actions={
          <Button onClick={() => setDialogo({ tipo: "crear" })}>
            <Plus className="size-4" /> Nuevo usuario
          </Button>
        }
      />
      <Card>
        <div className="border-b border-line p-3">
          <input
            type="search"
            placeholder="Buscar por nombre o correo"
            aria-label="Buscar usuarios"
            value={filtro}
            onChange={(e) => setFiltro(e.target.value)}
            className="min-h-10 w-full max-w-sm rounded-lg border border-line px-3 text-sm"
          />
        </div>
        {isLoading && !data ? (
          <div className="space-y-2 p-4">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-10" />
            ))}
          </div>
        ) : error ? (
          <div className="p-4">
            <Alert tone="danger">{error}</Alert>
          </div>
        ) : filas.length === 0 ? (
          <EmptyState icon={<Users className="size-8" />} title={filtro ? "Sin coincidencias" : "Aún no hay usuarios"}>
            {filtro ? "Pruebe con otro término de búsqueda." : "Cree el primer usuario con el botón “Nuevo usuario”."}
          </EmptyState>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px] text-sm">
              <thead className="border-b border-line text-left text-xs text-muted">
                <tr>
                  <th className="px-4 py-2.5 font-medium">Usuario</th>
                  <th className="px-4 py-2.5 font-medium">Rol</th>
                  <th className="px-4 py-2.5 font-medium">Estado</th>
                  <th className="px-4 py-2.5 font-medium">Último acceso</th>
                  <th className="px-4 py-2.5 font-medium">
                    <span className="sr-only">Acciones</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {filas.map((u) => (
                  <tr key={u.id} className="border-b border-line last:border-0">
                    <td className="px-4 py-3">
                      <p className="font-medium">{u.nombre}</p>
                      <p className="text-xs text-muted">
                        {u.email}
                        {u.registroProfesional && ` · ${u.registroProfesional}`}
                      </p>
                    </td>
                    <td className="px-4 py-3">{ETIQUETA_ROL[u.rol]}</td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap gap-1">
                        {u.activo ? <Badge tone="ok">Activo</Badge> : <Badge>Inactivo</Badge>}
                        {!u.mfaActivo && u.activo && <Badge tone="warn">Sin MFA</Badge>}
                        {u.debeCambiarPassword && <Badge tone="brand">Clave temporal</Badge>}
                        {u.bloqueadoHasta && new Date(u.bloqueadoHasta) > new Date() && (
                          <Badge tone="danger">Bloqueado</Badge>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-muted">{fmtRelativo(u.ultimoAcceso)}</td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          className="px-2"
                          aria-label={`Editar ${u.nombre}`}
                          onClick={() => setDialogo({ tipo: "editar", usuario: u })}
                        >
                          <Pencil className="size-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          className="px-2"
                          aria-label={`Restablecer contraseña de ${u.nombre}`}
                          title="Restablecer contraseña"
                          disabled={u.id === yo?.id}
                          onClick={() => setDialogo({ tipo: "reset-password", usuario: u })}
                        >
                          <KeyRound className="size-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          className="px-2"
                          aria-label={`Restablecer MFA de ${u.nombre}`}
                          title="Restablecer MFA"
                          disabled={!u.mfaActivo || u.id === yo?.id}
                          onClick={() => setDialogo({ tipo: "reset-mfa", usuario: u })}
                        >
                          <ShieldOff className="size-4" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <Modal open={dialogo !== null} title={TITULOS[dialogo?.tipo]} onClose={() => setDialogo(null)}>
        {dialogo?.tipo === "crear" && (
          <FormularioUsuario
            alTerminar={({ usuario, passwordTemporal }) => {
              recargar();
              setDialogo({ tipo: "temporal", usuario, password: passwordTemporal });
            }}
          />
        )}
        {dialogo?.tipo === "editar" && (
          <FormularioUsuario
            usuario={dialogo.usuario}
            alTerminar={() => {
              recargar();
              setDialogo(null);
            }}
          />
        )}
        {(dialogo?.tipo === "reset-password" || dialogo?.tipo === "reset-mfa") && (
          <ConfirmarReset
            dialogo={dialogo}
            alTerminar={(temp) => {
              recargar();
              setDialogo(temp ? { tipo: "temporal", usuario: dialogo.usuario, password: temp } : null);
            }}
          />
        )}
        {dialogo?.tipo === "temporal" && <PasswordTemporal email={dialogo.usuario.email} password={dialogo.password} />}
      </Modal>
    </>
  );
};

export default Usuarios;

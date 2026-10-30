// Pantallas del inicio de sesión en pasos (RNF-05):
//   Login (contraseña) → CambiarPassword (primer ingreso) → ConfigurarMFA / VerificarMFA → sesión completa.
import { KeyRound, LockKeyhole, ShieldCheck, Smartphone } from "lucide-react";
import QRCode from "qrcode";
import { useEffect, useState } from "react";
import { Alert, Button, Input, Spinner } from "../../components/ui";
import { useAuth } from "../../context/useAuth";
import * as auth from "../../services/authService";
import { mensajeDe } from "../../services/http";

const Marco = ({ icono, titulo, subtitulo, children }) => {
  const { perfil, salir } = useAuth();
  return (
    <div className="grid min-h-dvh lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
      <div className="relative hidden flex-col justify-between overflow-hidden bg-brand-800 p-10 text-white lg:flex">
        <div className="flex items-center gap-3">
          <img src="/favicon.svg" alt="" className="size-10" />
          <div>
            <p className="text-lg font-semibold tracking-tight">SIGO-Dental</p>
            <p className="text-sm text-brand-100/80">Gestión odontológica y expediente clínico digital</p>
          </div>
        </div>
        <ul className="space-y-4 text-sm text-brand-100">
          <li className="flex gap-3">
            <ShieldCheck className="size-5 shrink-0 text-brand-200" aria-hidden /> Acceso con verificación en dos pasos
            para información clínica y financiera.
          </li>
          <li className="flex gap-3">
            <LockKeyhole className="size-5 shrink-0 text-brand-200" aria-hidden /> Cada consulta y modificación queda
            registrada en una bitácora de auditoría.
          </li>
        </ul>
        <p className="text-xs text-brand-100/50">Entorno de demostración académica · todos los datos son ficticios</p>
      </div>
      <div className="flex items-center justify-center px-5 py-10">
        <div className="w-full max-w-sm">
          <div className="mb-6 flex size-11 items-center justify-center rounded-xl bg-brand-50 text-brand-700">
            {icono}
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">{titulo}</h1>
          <p className="mt-1 text-sm text-muted">{subtitulo}</p>
          <div className="mt-6">{children}</div>
          {perfil && (
            <button
              onClick={() => salir()}
              className="mt-6 text-sm text-muted underline-offset-2 hover:text-ink hover:underline"
            >
              Cancelar y volver al inicio de sesión
            </button>
          )}
        </div>
      </div>
    </div>
  );
};

export const Login = () => {
  const { ingresar, motivoCierre } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState(null);
  const [enviando, setEnviando] = useState(false);

  const enviar = async (e) => {
    e.preventDefault();
    setError(null);
    if (!email.trim() || !password) return setError("Ingrese su correo y contraseña.");
    setEnviando(true);
    try {
      await ingresar(email.trim(), password);
    } catch (err) {
      setError(mensajeDe(err));
    } finally {
      setEnviando(false);
    }
  };

  return (
    <Marco
      icono={<KeyRound className="size-5" />}
      titulo="Iniciar sesión"
      subtitulo="Ingrese con su cuenta institucional."
    >
      <form onSubmit={enviar} className="space-y-4" noValidate>
        {motivoCierre && !error && <Alert tone="warn">{motivoCierre}</Alert>}
        {error && <Alert tone="danger">{error}</Alert>}
        <Input
          label="Correo electrónico"
          type="email"
          autoComplete="username"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          autoFocus
        />
        <Input
          label="Contraseña"
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        <Button type="submit" loading={enviando} className="w-full">
          Continuar
        </Button>
        <p className="text-xs text-muted">
          ¿Olvidó su contraseña? Solicite al administrador del sistema que la restablezca.
        </p>
      </form>
    </Marco>
  );
};

const FormularioCodigo = ({ alEnviar, texto }) => {
  const [codigo, setCodigo] = useState("");
  const [error, setError] = useState(null);
  const [enviando, setEnviando] = useState(false);

  const enviar = async (e) => {
    e.preventDefault();
    if (!/^\d{6}$/.test(codigo)) return setError("El código tiene 6 dígitos.");
    setEnviando(true);
    setError(null);
    try {
      await alEnviar(codigo);
    } catch (err) {
      setError(mensajeDe(err));
      setCodigo("");
    } finally {
      setEnviando(false);
    }
  };

  return (
    <form onSubmit={enviar} className="space-y-4" noValidate>
      {error && <Alert tone="danger">{error}</Alert>}
      <Input
        label="Código de verificación"
        inputMode="numeric"
        autoComplete="one-time-code"
        maxLength={6}
        value={codigo}
        onChange={(e) => setCodigo(e.target.value.replace(/\D/g, ""))}
        className="[&_input]:text-center [&_input]:font-mono [&_input]:text-xl [&_input]:tracking-[0.4em]"
        autoFocus
      />
      <Button type="submit" loading={enviando} className="w-full">
        {texto}
      </Button>
    </form>
  );
};

export const VerificarMFA = () => {
  const { aplicarSesion } = useAuth();
  return (
    <Marco
      icono={<Smartphone className="size-5" />}
      titulo="Verificación en dos pasos"
      subtitulo="Abra su aplicación autenticadora e ingrese el código de 6 dígitos de SIGO-Dental."
    >
      <FormularioCodigo texto="Verificar" alEnviar={async (c) => aplicarSesion(await auth.verificarMFA(c))} />
    </Marco>
  );
};

const REGLAS = [
  { prueba: (p) => [...p].length >= 10, texto: "Al menos 10 caracteres" },
  { prueba: (p) => /\p{L}/u.test(p), texto: "Al menos una letra" },
  { prueba: (p) => /\d/.test(p), texto: "Al menos un número" },
];

export const CambiarPassword = () => {
  const { recargar } = useAuth();
  const [form, setForm] = useState({ actual: "", nueva: "", confirmar: "" });
  const [error, setError] = useState(null);
  const [enviando, setEnviando] = useState(false);
  const noCoincide = form.confirmar.length > 0 && form.nueva !== form.confirmar;

  const enviar = async (e) => {
    e.preventDefault();
    setError(null);
    if (!REGLAS.every((r) => r.prueba(form.nueva))) return setError("La nueva contraseña no cumple los requisitos.");
    if (form.nueva !== form.confirmar) return setError("Las contraseñas no coinciden.");
    setEnviando(true);
    try {
      await auth.cambiarPassword(form.actual, form.nueva);
      await recargar();
    } catch (err) {
      setError(mensajeDe(err));
    } finally {
      setEnviando(false);
    }
  };

  return (
    <Marco
      icono={<LockKeyhole className="size-5" />}
      titulo="Cree una contraseña nueva"
      subtitulo="Su contraseña actual es temporal. Defina una personal para continuar."
    >
      <form onSubmit={enviar} className="space-y-4" noValidate>
        {error && <Alert tone="danger">{error}</Alert>}
        <Input
          label="Contraseña actual (temporal)"
          type="password"
          autoComplete="current-password"
          value={form.actual}
          onChange={(e) => setForm({ ...form, actual: e.target.value })}
        />
        <Input
          label="Nueva contraseña"
          type="password"
          autoComplete="new-password"
          value={form.nueva}
          onChange={(e) => setForm({ ...form, nueva: e.target.value })}
        />
        <ul className="space-y-1 text-xs" aria-label="Requisitos de la contraseña">
          {REGLAS.map((r) => (
            <li key={r.texto} className={r.prueba(form.nueva) ? "text-ok-700" : "text-muted"}>
              {r.prueba(form.nueva) ? "✓" : "•"} {r.texto}
            </li>
          ))}
        </ul>
        <Input
          label="Confirmar nueva contraseña"
          type="password"
          autoComplete="new-password"
          value={form.confirmar}
          error={noCoincide ? "No coincide con la nueva contraseña." : undefined}
          onChange={(e) => setForm({ ...form, confirmar: e.target.value })}
        />
        <Button type="submit" loading={enviando} className="w-full">
          Guardar contraseña
        </Button>
      </form>
    </Marco>
  );
};

export const ConfigurarMFA = () => {
  const { aplicarSesion } = useAuth();
  const [config, setConfig] = useState(null); // { secreto, otpauthUrl, qr }
  const [error, setError] = useState(null);

  useEffect(() => {
    let activo = true;
    auth
      .configurarMFA()
      .then(async (r) => {
        // El QR se dibuja en el navegador: el secreto no viaja a ningún servicio externo.
        const qr = await QRCode.toDataURL(r.otpauthUrl, { margin: 1, width: 192 });
        if (activo) setConfig({ ...r, qr });
      })
      .catch((e) => activo && setError(mensajeDe(e)));
    return () => {
      activo = false;
    };
  }, []);

  return (
    <Marco
      icono={<ShieldCheck className="size-5" />}
      titulo="Active la verificación en dos pasos"
      subtitulo="Es obligatoria para acceder a expedientes clínicos e información financiera."
    >
      {error && <Alert tone="danger">{error}</Alert>}
      {!config && !error && <Spinner label="Generando código…" />}
      {config && (
        <div className="space-y-5">
          <ol className="list-decimal space-y-1 pl-5 text-sm text-muted">
            <li>Instale una app autenticadora compatible con TOTP (Google Authenticator, Microsoft Authenticator…).</li>
            <li>Escanee este código QR.</li>
            <li>Ingrese el código de 6 dígitos que muestra la app.</li>
          </ol>
          <div className="flex justify-center rounded-xl border border-line bg-white p-4">
            <img src={config.qr} alt="Código QR para la app autenticadora" className="size-48" />
          </div>
          <details className="text-sm">
            <summary className="cursor-pointer text-brand-700">
              ¿No puede escanear? Ingrese la clave manualmente
            </summary>
            <code className="mt-2 block break-all rounded-lg bg-canvas p-2 font-mono text-xs">{config.secreto}</code>
          </details>
          <FormularioCodigo texto="Activar" alEnviar={async (c) => aplicarSesion(await auth.activarMFA(c))} />
        </div>
      )}
    </Marco>
  );
};

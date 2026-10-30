import { LogOut, Menu, ShieldCheck, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../context/useAuth";
import { useInactividad } from "../hooks/useInactividad";
import BannerSinConexion from "./BannerSinConexion";
import * as authApi from "../services/authService";

// Aviso permanente en el despliegue de demostración (APP_ENV=demo).
const AvisoDemo = () => {
  const [demo, setDemo] = useState(false);
  useEffect(() => {
    authApi.estado().then((r) => setDemo(Boolean(r?.demo)), () => {});
  }, []);
  if (!demo) return null;
  return (
    <div className="bg-amber-100 px-4 py-1.5 text-center text-xs font-medium text-amber-900">
      DEMOSTRACIÓN · todos los pacientes y datos son ficticios · facturación y mensajería simuladas
    </div>
  );
};
import InstalarApp from "./InstalarApp";
import { ETAPA_ACTUAL, MODULOS } from "./modulos";
import { Button, Modal } from "./ui";
import { cx } from "../utils/cx";

const Marca = () => (
  <div className="flex items-center gap-2.5 px-2">
    <img src="/favicon.svg" alt="" className="size-8" />
    <div className="leading-tight">
      <p className="font-semibold tracking-tight text-white">SIGO-Dental</p>
      <p className="text-[11px] text-brand-100/80">Gestión odontológica</p>
    </div>
  </div>
);

const Opcion = ({ m, alNavegar }) => {
  const Icono = m.icono;
  const pendiente = m.etapa > ETAPA_ACTUAL;
  return (
    <NavLink
      to={m.ruta}
      end={m.ruta === "/"}
      onClick={alNavegar}
      className={({ isActive }) =>
        cx(
          "flex min-h-10 items-center gap-3 rounded-lg px-3 text-sm transition-colors",
          isActive ? "bg-white/12 font-medium text-white" : "text-brand-100 hover:bg-white/8 hover:text-white",
          pendiente && !isActive && "text-brand-100/60",
        )
      }
    >
      <Icono className="size-[18px] shrink-0" aria-hidden />
      <span className="flex-1 truncate">{m.etiqueta}</span>
      {pendiente && (
        <span
          className="rounded bg-white/10 px-1.5 py-0.5 text-[10px] font-medium text-brand-100/80"
          title={`Se implementa en la Etapa ${m.etapa}`}
        >
          E{m.etapa}
        </span>
      )}
    </NavLink>
  );
};

// Estructura de las pantallas con sesión: menú lateral (cajón en el celular), cabecera y contenido.
const Layout = () => {
  const { perfil, usuario, puede, salir } = useAuth();
  const [abierto, setAbierto] = useState(false);
  const alVencer = useCallback(() => salir("Su sesión se cerró por inactividad."), [salir]);
  const { segundosRestantes, seguirActivo } = useInactividad(perfil?.sesion?.minutosInactividad ?? 15, alVencer);

  if (!usuario) return null;
  const visibles = MODULOS.filter((m) => puede(...m.permisos));
  const principal = visibles.filter((m) => m.grupo === "principal");
  const admin = visibles.filter((m) => m.grupo === "admin");
  const cerrar = () => setAbierto(false);

  const menu = (
    <nav
      aria-label="Navegación principal"
      className="flex h-full flex-col gap-6 overflow-y-auto bg-brand-800 px-3 py-5"
    >
      <Marca />
      <div className="space-y-0.5">
        {principal.map((m) => (
          <Opcion key={m.ruta} m={m} alNavegar={cerrar} />
        ))}
      </div>
      {admin.length > 0 && (
        <div>
          <p className="mb-1 px-3 text-[11px] font-medium uppercase tracking-wider text-brand-100/60">Administración</p>
          <div className="space-y-0.5">
            {admin.map((m) => (
              <Opcion key={m.ruta} m={m} alNavegar={cerrar} />
            ))}
          </div>
        </div>
      )}
      <div className="mt-auto space-y-2">
        <div className="overflow-hidden rounded-lg bg-white">
          <InstalarApp alTerminar={cerrar} />
        </div>
        <p className="px-3 text-[11px] leading-relaxed text-brand-100/50">Entorno de demostración · datos ficticios</p>
      </div>
    </nav>
  );

  return (
    <div className="min-h-dvh lg:grid lg:grid-cols-[248px_1fr]">
      <aside className="sticky top-0 hidden h-dvh lg:block">{menu}</aside>
      {abierto && (
        <div className="fixed inset-0 z-40 lg:hidden" role="dialog" aria-modal="true" aria-label="Menú">
          <div className="absolute inset-0 bg-ink/40" onClick={cerrar} />
          <div className="absolute inset-y-0 left-0 w-72">
            {menu}
            <button
              onClick={cerrar}
              aria-label="Cerrar menú"
              className="absolute right-3 top-5 rounded p-1 text-white/80 hover:bg-white/10"
            >
              <X className="size-5" />
            </button>
          </div>
        </div>
      )}
      <div className="flex min-w-0 flex-col">
        <BannerSinConexion />
        <AvisoDemo />
        <header className="sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-line bg-white/95 px-4 backdrop-blur sm:px-6">
          <button
            onClick={() => setAbierto(true)}
            className="rounded-lg p-2 text-ink hover:bg-canvas lg:hidden"
            aria-label="Abrir menú"
          >
            <Menu className="size-5" />
          </button>
          <div className="ml-auto flex items-center gap-3">
            <div className="hidden text-right sm:block">
              <p className="text-sm font-medium leading-tight">{usuario.nombre}</p>
              <p className="text-xs text-muted">{usuario.etiquetaRol}</p>
            </div>
            <span
              className="hidden items-center gap-1 rounded-md bg-ok-50 px-2 py-1 text-xs font-medium text-ok-700 md:inline-flex"
              title="Sesión verificada con segundo factor"
            >
              <ShieldCheck className="size-3.5" aria-hidden /> MFA
            </span>
            <Button variant="secondary" onClick={() => salir()} className="px-3">
              <LogOut className="size-4" aria-hidden />
              <span className="hidden sm:inline">Salir</span>
            </Button>
          </div>
        </header>
        <main className="mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6 lg:px-8">
          <Outlet />
        </main>
      </div>
      <Modal
        open={segundosRestantes !== null}
        title="¿Sigue ahí?"
        onClose={seguirActivo}
        footer={
          <>
            <Button variant="secondary" onClick={() => salir()}>
              Cerrar sesión
            </Button>
            <Button onClick={seguirActivo}>Continuar trabajando</Button>
          </>
        }
      >
        <p className="text-sm text-muted">
          Por seguridad, la sesión se cerrará en <strong className="text-ink">{segundosRestantes} s</strong> por
          inactividad.
        </p>
      </Modal>
    </div>
  );
};

export default Layout;

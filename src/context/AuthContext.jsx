// Estado de la sesión: perfil (/api/me), permisos y el paso pendiente del inicio de sesión.
//   perfil = { usuario, permisos, sesion: { mfaVerificado, expiraEn, minutosInactividad }, siguientePaso }
// siguientePaso: VERIFICAR_MFA | CAMBIAR_PASSWORD | CONFIGURAR_MFA | null (sesión completa)
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import * as auth from "../services/authService";
import { ApiError, EVENTO_SESION_PERDIDA, guardarToken, leerToken } from "../services/http";
import { AuthContext } from "./AuthContextValue";

const MOTIVOS = {
  SESION_EXPIRADA: "Su sesión se cerró por inactividad.",
  NO_AUTENTICADO: "Su sesión finalizó. Ingrese nuevamente.",
};

export const AuthProvider = ({ children }) => {
  const [perfil, setPerfil] = useState(null);
  const [cargando, setCargando] = useState(() => !!leerToken());
  const [motivoCierre, setMotivoCierre] = useState(null);
  const hayPerfil = useRef(false);
  useEffect(() => {
    hayPerfil.current = !!perfil;
  }, [perfil]);

  const recargar = useCallback(async () => {
    try {
      setPerfil(await auth.getPerfil());
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) setPerfil(null);
      else throw e;
    }
  }, []);

  // Al abrir la app con un token guardado en la pestaña, se recupera el perfil.
  useEffect(() => {
    if (!leerToken()) return;
    let activo = true;
    auth
      .getPerfil()
      .then((p) => activo && setPerfil(p))
      .catch(() => activo && setPerfil(null))
      .finally(() => activo && setCargando(false));
    return () => {
      activo = false;
    };
  }, []);

  // El cliente HTTP avisa cuando el servidor rechaza el token (inactividad, revocación...).
  useEffect(() => {
    const alPerder = (ev) => {
      // Sólo se avisa si había una sesión abierta (no al abrir la app sin sesión).
      if (hayPerfil.current) setMotivoCierre(MOTIVOS[ev.detail] ?? MOTIVOS.NO_AUTENTICADO);
      setPerfil(null);
    };
    window.addEventListener(EVENTO_SESION_PERDIDA, alPerder);
    return () => window.removeEventListener(EVENTO_SESION_PERDIDA, alPerder);
  }, []);

  const ingresar = useCallback(
    async (email, password) => {
      const r = await auth.login(email, password);
      guardarToken(r.token);
      setMotivoCierre(null);
      await recargar();
      return r.siguientePaso;
    },
    [recargar],
  );

  // Respuestas que rotan el token (verificar/activar MFA): traen { token, ...perfil }.
  const aplicarSesion = useCallback(({ token, ...nuevoPerfil }) => {
    if (token) guardarToken(token);
    setPerfil(nuevoPerfil);
  }, []);

  const salir = useCallback(async (motivo) => {
    try {
      await auth.logout();
    } catch {
      // la sesión puede haber expirado ya en el servidor
    }
    guardarToken(null);
    setPerfil(null);
    setMotivoCierre(motivo ?? null);
  }, []);

  const valor = useMemo(() => {
    const permisos = perfil?.permisos ?? [];
    return {
      perfil,
      usuario: perfil?.usuario ?? null,
      cargando,
      motivoCierre,
      ingresar,
      salir,
      recargar,
      aplicarSesion,
      // true si el usuario tiene AL MENOS uno de los permisos indicados
      puede: (...ps) => ps.length === 0 || ps.some((p) => permisos.includes(p)),
    };
  }, [perfil, cargando, motivoCierre, ingresar, salir, recargar, aplicarSesion]);

  return <AuthContext.Provider value={valor}>{children}</AuthContext.Provider>;
};

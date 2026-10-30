import { Navigate, Route, Routes } from "react-router-dom";
import AvisoActualizacion from "./components/AvisoActualizacion";
import Layout from "./components/Layout";
import { ETAPA_ACTUAL, MODULOS } from "./components/modulos";
import { Spinner } from "./components/ui";
import { useAuth } from "./context/useAuth";
import Ajustes from "./pages/admin/Ajustes";
import Auditoria from "./pages/admin/Auditoria";
import Usuarios from "./pages/admin/Usuarios";
import { CambiarPassword, ConfigurarMFA, Login, VerificarMFA } from "./pages/auth/PasosIngreso";
import { ModuloPendiente, NoEncontrada } from "./pages/ModuloPendiente";
import Panel from "./pages/Panel";

const PANTALLAS = {
  "/usuarios": Usuarios,
  "/configuracion": Ajustes,
  "/auditoria": Auditoria,
};

// Mientras falte un paso del inicio de sesión (siguientePaso), no se muestra ninguna otra pantalla.
const PASOS = { VERIFICAR_MFA: VerificarMFA, CAMBIAR_PASSWORD: CambiarPassword, CONFIGURAR_MFA: ConfigurarMFA };

const App = () => {
  const { perfil, cargando, puede } = useAuth();
  if (cargando) return <Spinner label="Cargando SIGO-Dental…" />;

  let contenido;
  if (!perfil) contenido = <Login />;
  else if (perfil.siguientePaso && PASOS[perfil.siguientePaso]) {
    const Paso = PASOS[perfil.siguientePaso];
    contenido = <Paso />;
  } else
    contenido = (
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Panel />} />
          {MODULOS.filter((m) => m.ruta !== "/" && puede(...m.permisos)).map((m) => {
            const Pantalla = PANTALLAS[m.ruta];
            return (
              <Route
                key={m.ruta}
                path={m.ruta.slice(1)}
                element={m.etapa <= ETAPA_ACTUAL && Pantalla ? <Pantalla /> : <ModuloPendiente modulo={m} />}
              />
            );
          })}
          <Route path="login" element={<Navigate to="/" replace />} />
          <Route path="*" element={<NoEncontrada />} />
        </Route>
      </Routes>
    );

  return (
    <>
      {contenido}
      <AvisoActualizacion />
    </>
  );
};

export default App;

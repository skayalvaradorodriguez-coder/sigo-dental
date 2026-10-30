import { Navigate, Route, Routes } from "react-router-dom";
import AvisoActualizacion from "./components/AvisoActualizacion";
import Layout from "./components/Layout";
import { ETAPA_ACTUAL, MODULOS } from "./components/modulos";
import { Spinner } from "./components/ui";
import { useAuth } from "./context/useAuth";
import Agenda from "./pages/agenda/Agenda";
import Ajustes from "./pages/admin/Ajustes";
import Auditoria from "./pages/admin/Auditoria";
import Catalogos from "./pages/admin/Catalogos";
import Usuarios from "./pages/admin/Usuarios";
import { CambiarPassword, ConfigurarMFA, Login, VerificarMFA } from "./pages/auth/PasosIngreso";
import Consentimientos from "./pages/consentimientos/Consentimientos";
import Expediente from "./pages/expediente/Expediente";
import { ModuloPendiente, NoEncontrada } from "./pages/ModuloPendiente";
import FichaPaciente from "./pages/pacientes/FichaPaciente";
import FormularioPaciente from "./pages/pacientes/FormularioPaciente";
import Pacientes from "./pages/pacientes/Pacientes";
import Panel from "./pages/Panel";
import Presupuestos from "./pages/presupuestos/Presupuestos";

const PANTALLAS = {
  "/usuarios": Usuarios,
  "/configuracion": Ajustes,
  "/auditoria": Auditoria,
  "/catalogos": Catalogos,
  "/pacientes": Pacientes,
  "/agenda": Agenda,
  "/expediente": Expediente,
  "/presupuestos": Presupuestos,
  "/consentimientos": Consentimientos,
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
          {puede("pacientes.escribir") && <Route path="pacientes/nuevo" element={<FormularioPaciente />} />}
          {puede("pacientes.leer") && <Route path="pacientes/:id" element={<FichaPaciente />} />}
          {puede("pacientes.escribir") && <Route path="pacientes/:id/editar" element={<FormularioPaciente />} />}
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

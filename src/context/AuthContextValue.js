import { createContext } from "react";

// Separado de AuthContext.jsx para que el archivo del componente sólo exporte componentes (Fast Refresh).
export const AuthContext = createContext(null);

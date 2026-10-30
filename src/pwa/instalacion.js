// Estado de instalación de la PWA. Se importa en main.jsx ANTES de montar React porque el
// navegador puede disparar "beforeinstallprompt" apenas carga la página.

let eventoInstalacion = null; // evento guardado para mostrar el diálogo cuando el usuario quiera
const oyentes = new Set();
const avisar = () => oyentes.forEach((oyente) => oyente());

const enModoApp = () =>
  window.matchMedia?.("(display-mode: standalone)").matches || window.navigator.standalone === true;

window.addEventListener("beforeinstallprompt", (evento) => {
  evento.preventDefault(); // evitamos el aviso automático: lo ofrecemos desde el menú
  eventoInstalacion = evento;
  avisar();
});

window.addEventListener("appinstalled", () => {
  eventoInstalacion = null;
  avisar();
});

export const suscribirInstalacion = (oyente) => {
  oyentes.add(oyente);
  return () => oyentes.delete(oyente);
};

// "disponible": Chrome/Edge/Android ofrecen instalar · "ios": Safari de iPhone/iPad (instalación manual)
// · "instalada": ya se abrió como app · "no": el navegador no lo permite.
export const estadoInstalacion = () => {
  if (enModoApp()) return "instalada";
  if (eventoInstalacion) return "disponible";
  const esIOS =
    /iphone|ipad|ipod/i.test(navigator.userAgent) ||
    (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
  if (esIOS) return "ios";
  return "no";
};

// Muestra el diálogo nativo de instalación. Devuelve true si el usuario aceptó.
export const mostrarDialogoInstalacion = async () => {
  if (!eventoInstalacion) return false;
  const evento = eventoInstalacion;
  eventoInstalacion = null; // el evento solo se puede usar una vez
  await evento.prompt();
  const { outcome } = await evento.userChoice;
  avisar();
  return outcome === "accepted";
};

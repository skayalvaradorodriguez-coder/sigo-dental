// Impresión de documentos (receta, consentimiento, presupuesto) en un iframe aparte: así se imprime
// sólo el documento, sin el menú ni la interfaz. El contenido se escapa (nunca HTML del usuario).
// Los estilos vienen de /imprimir.css (la CSP no admite <style> en línea).

export const escapar = (s) =>
  String(s ?? "").replace(
    /[&<>"']/g,
    (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c],
  );


// cuerpoHTML debe construirse con escapar() en todos los datos.
export const imprimir = (titulo, cuerpoHTML) => {
  const iframe = document.createElement("iframe");
  iframe.setAttribute("aria-hidden", "true");
  iframe.style.cssText = "position:fixed;right:0;bottom:0;width:0;height:0;border:0";
  document.body.appendChild(iframe);
  const doc = iframe.contentDocument;
  doc.open();
  doc.write(
    `<!doctype html><html lang="es"><head><meta charset="utf-8"><title>${escapar(titulo)}</title><link rel="stylesheet" href="${window.location.origin}/imprimir.css"></head><body>${cuerpoHTML}</body></html>`,
  );
  doc.close();
  const lanzar = () => {
    iframe.contentWindow.focus();
    iframe.contentWindow.print();
    setTimeout(() => iframe.remove(), 1000);
  };
  // Espera a que carguen la hoja de estilos y las imágenes (firma) antes de imprimir.
  const pendientes = [...doc.images].filter((i) => !i.complete);
  const hoja = doc.querySelector('link[rel="stylesheet"]');
  const esperas = pendientes.map((i) => new Promise((r) => (i.onload = i.onerror = r)));
  if (hoja && !hoja.sheet) esperas.push(new Promise((r) => (hoja.onload = hoja.onerror = r)));
  Promise.all(esperas).then(() => setTimeout(lanzar, 50));
};

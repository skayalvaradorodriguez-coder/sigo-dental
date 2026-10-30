// Impresión de documentos (receta, consentimiento, presupuesto) en un iframe aparte: así se imprime
// sólo el documento, sin el menú ni la interfaz. El contenido se escapa (nunca HTML del usuario).

export const escapar = (s) =>
  String(s ?? "").replace(
    /[&<>"']/g,
    (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c],
  );

const ESTILO = `
  body { font: 13px/1.5 system-ui, sans-serif; color: #111; margin: 24px; }
  h1 { font-size: 18px; margin: 0 0 4px; } h2 { font-size: 14px; margin: 18px 0 6px; }
  .muted { color: #555; font-size: 11px; } .pre { white-space: pre-wrap; }
  table { width: 100%; border-collapse: collapse; margin-top: 8px; }
  th, td { border-bottom: 1px solid #ccc; padding: 4px 6px; text-align: left; vertical-align: top; }
  td.num, th.num { text-align: right; } img.firma { max-height: 110px; border-bottom: 1px solid #333; }
  .pie { margin-top: 28px; font-size: 10px; color: #555; border-top: 1px solid #ccc; padding-top: 6px; }
`;

// cuerpoHTML debe construirse con escapar() en todos los datos.
export const imprimir = (titulo, cuerpoHTML) => {
  const iframe = document.createElement("iframe");
  iframe.setAttribute("aria-hidden", "true");
  iframe.style.cssText = "position:fixed;right:0;bottom:0;width:0;height:0;border:0";
  document.body.appendChild(iframe);
  const doc = iframe.contentDocument;
  doc.open();
  doc.write(
    `<!doctype html><html lang="es"><head><meta charset="utf-8"><title>${escapar(titulo)}</title><style>${ESTILO}</style></head><body>${cuerpoHTML}</body></html>`,
  );
  doc.close();
  const lanzar = () => {
    iframe.contentWindow.focus();
    iframe.contentWindow.print();
    setTimeout(() => iframe.remove(), 1000);
  };
  // Espera a que carguen las imágenes (firma) antes de imprimir.
  const imgs = [...doc.images].filter((i) => !i.complete);
  if (imgs.length === 0) setTimeout(lanzar, 50);
  else Promise.all(imgs.map((i) => new Promise((r) => (i.onload = i.onerror = r)))).then(lanzar);
};

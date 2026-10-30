// Recuadro de firma manuscrita: el firmante traza con el dedo, lápiz o mouse (Pointer Events).
// Se guarda la IMAGEN del trazo (PNG). No es una firma biométrica ni una firma digital con certificado.
import { Eraser } from "lucide-react";
import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import { Button } from "../../components/ui";

const LienzoFirma = forwardRef(function LienzoFirma({ alCambiar }, ref) {
  const canvas = useRef(null);
  const dibujando = useRef(false);
  const puntos = useRef(0);
  const [vacio, setVacio] = useState(true);

  // Ajusta la resolución al tamaño real (pantallas de alta densidad) y fondo blanco.
  useEffect(() => {
    const c = canvas.current;
    const r = c.getBoundingClientRect();
    const escala = window.devicePixelRatio || 1;
    c.width = Math.max(1, Math.round(r.width * escala));
    c.height = Math.max(1, Math.round(r.height * escala));
    const ctx = c.getContext("2d");
    if (!ctx) return;
    ctx.scale(escala, escala);
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, r.width, r.height);
    ctx.lineWidth = 2.2;
    ctx.lineCap = "round";
    ctx.lineJoin = "round";
    ctx.strokeStyle = "#0b1f3a";
  }, []);

  const pos = (e) => {
    const r = canvas.current.getBoundingClientRect();
    return [e.clientX - r.left, e.clientY - r.top];
  };

  const limpiar = () => {
    const c = canvas.current;
    const ctx = c.getContext("2d");
    if (ctx) {
      ctx.save();
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.fillStyle = "#fff";
      ctx.fillRect(0, 0, c.width, c.height);
      ctx.restore();
    }
    puntos.current = 0;
    setVacio(true);
    alCambiar?.(true);
  };

  useImperativeHandle(ref, () => ({
    // Un trazo muy corto (un toque accidental) no cuenta como firma.
    vacio: () => puntos.current < 15,
    aDataURL: () => canvas.current.toDataURL("image/png"),
    limpiar,
  }));

  return (
    <div>
      <div className="relative">
        <canvas
          ref={canvas}
          aria-label="Recuadro de firma: firme con el dedo, un lápiz o el mouse"
          role="img"
          className="h-44 w-full touch-none rounded-lg border-2 border-dashed border-line bg-white"
          onPointerDown={(e) => {
            e.currentTarget.setPointerCapture?.(e.pointerId);
            dibujando.current = true;
            const ctx = canvas.current.getContext("2d");
            const [x, y] = pos(e);
            ctx?.beginPath();
            ctx?.moveTo(x, y);
          }}
          onPointerMove={(e) => {
            if (!dibujando.current) return;
            const ctx = canvas.current.getContext("2d");
            const [x, y] = pos(e);
            ctx?.lineTo(x, y);
            ctx?.stroke();
            puntos.current += 1;
            if (vacio && puntos.current >= 15) {
              setVacio(false);
              alCambiar?.(false);
            }
          }}
          onPointerUp={() => (dibujando.current = false)}
          onPointerLeave={() => (dibujando.current = false)}
        />
        {vacio && (
          <span className="pointer-events-none absolute inset-0 flex items-center justify-center text-sm text-muted">
            Firme aquí
          </span>
        )}
        <span className="pointer-events-none absolute bottom-6 left-6 right-6 border-b border-ink/30" aria-hidden />
      </div>
      <div className="mt-1 flex items-center justify-between">
        <p className="text-xs text-muted">Se guarda la imagen del trazo y su huella SHA-256.</p>
        <Button type="button" variant="ghost" onClick={limpiar}>
          <Eraser className="size-4" /> Borrar
        </Button>
      </div>
    </div>
  );
});

export default LienzoFirma;

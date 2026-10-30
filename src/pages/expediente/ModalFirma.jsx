// Firma electrónica simple: el profesional vuelve a confirmar su identidad con el código MFA y el
// servidor guarda un hash SHA-256 del contenido. NO es una firma digital con certificado.
import { PenLine } from "lucide-react";
import { useState } from "react";
import { Alert, Button, Input, Modal, Textarea } from "../../components/ui";
import { mensajeDe } from "../../services/http";

const ModalFirma = ({ titulo, descripcion, conTexto, etiquetaTexto = "Texto de la adenda", alFirmar, alCerrar }) => {
  const [codigo, setCodigo] = useState("");
  const [texto, setTexto] = useState("");
  const [error, setError] = useState("");
  const [enviando, setEnviando] = useState(false);

  const enviar = async (e) => {
    e.preventDefault();
    if (conTexto && texto.trim().length < 3) return setError("Escriba el texto (al menos 3 caracteres).");
    if (!/^\d{6}$/.test(codigo)) return setError("El código tiene 6 dígitos.");
    setEnviando(true);
    setError("");
    try {
      await alFirmar({ codigo, texto: texto.trim() });
    } catch (err) {
      setError(mensajeDe(err));
      setCodigo("");
    } finally {
      setEnviando(false);
    }
  };

  return (
    <Modal open title={titulo} onClose={alCerrar}>
      <form onSubmit={enviar} className="space-y-4" noValidate>
        {descripcion && <p className="text-sm text-muted">{descripcion}</p>}
        {error && <Alert tone="danger">{error}</Alert>}
        {conTexto && (
          <Textarea label={etiquetaTexto} rows={4} value={texto} onChange={(e) => setTexto(e.target.value)} autoFocus />
        )}
        <Input
          label="Código de verificación (app autenticadora)"
          inputMode="numeric"
          autoComplete="one-time-code"
          maxLength={6}
          value={codigo}
          onChange={(e) => setCodigo(e.target.value.replace(/\D/g, ""))}
          className="[&_input]:text-center [&_input]:font-mono [&_input]:text-lg [&_input]:tracking-[0.3em]"
          autoFocus={!conTexto}
        />
        <p className="text-xs text-muted">
          Firma electrónica simple: confirma su identidad con el código y se guarda una huella (hash SHA-256) del
          contenido con fecha, hora y su registro profesional. Lo firmado ya no se modifica; las correcciones se hacen
          con adendas.
        </p>
        <div className="flex justify-end gap-2 border-t border-line pt-4">
          <Button type="button" variant="secondary" onClick={alCerrar}>
            Cancelar
          </Button>
          <Button type="submit" loading={enviando}>
            <PenLine className="size-4" /> Firmar
          </Button>
        </div>
      </form>
    </Modal>
  );
};

export default ModalFirma;

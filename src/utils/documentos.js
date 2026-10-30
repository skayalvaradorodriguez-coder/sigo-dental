// Impresión de recetas y consentimientos y formatos compartidos por las pantallas de la Etapa 4.
import { ETIQUETA_METODO_FIRMA } from "./etiquetas";
import { fmtFechaHora, fmtFechaHoraSeg } from "./fechas";
import { escapar, imprimir } from "./imprimir";

export const fmtTamano = (b) =>
  b < 1024 ? `${b} B` : b < 1024 * 1024 ? `${(b / 1024).toFixed(0)} KB` : `${(b / 1024 / 1024).toFixed(1)} MB`;

export const imprimirConsentimiento = (c) =>
  imprimir(
    c.titulo,
    `<h1>${escapar(c.titulo)}</h1><p class="muted">Versión ${c.version} · ${escapar(c.paciente)}</p>` +
      `<p class="pre">${escapar(c.texto)}</p>` +
      (c.firmaImagen ? `<p><img class="firma" src="${escapar(c.firmaImagen)}" alt="Firma"></p>` : "") +
      `<p>${escapar(c.firmanteNombre)} · ${escapar(c.firmanteDocumento)} (${c.firmanteRelacion === "TUTOR" ? "tutor legal" : "paciente"})</p>` +
      `<p class="pie">${escapar(ETIQUETA_METODO_FIRMA[c.metodo])}${c.metodo === "OTP" ? ` a ${escapar(c.otpDestinatario)}${c.otpSimulado ? " (proveedor SIMULADO)" : ""}` : ""} · firmado ${escapar(fmtFechaHoraSeg(c.firmadoEn))}<br>` +
      `Huella del documento (SHA-256): ${escapar(c.hashDocumento)}<br>Huella de la firma: ${escapar(c.hashFirma)}<br>` +
      `Firma electrónica simple; no es una firma digital con certificado.</p>`,
  );

export const imprimirReceta = (r, paciente) =>
  imprimir(
    "Receta",
    `<h1>Receta</h1><p class="muted">${escapar(fmtFechaHora(r.emitidaEn))}</p>` +
      `<p>Paciente: <strong>${escapar(paciente.nombres)} ${escapar(paciente.apellidos)}</strong> · ${escapar(paciente.numeroDocumento)} · ${paciente.edad} años</p>` +
      `<table><thead><tr><th>Medicamento</th><th>Presentación</th><th>Dosis</th><th>Vía</th><th>Frecuencia</th><th>Duración</th></tr></thead><tbody>` +
      r.items
        .map(
          (it) =>
            `<tr><td><strong>${escapar(it.medicamento)}</strong>${it.indicaciones ? `<br><span class="muted">${escapar(it.indicaciones)}</span>` : ""}</td><td>${escapar(it.presentacion)}</td><td>${escapar(it.dosis)}</td><td>${escapar(it.via)}</td><td>${escapar(it.frecuencia)}</td><td>${escapar(it.duracion)}</td></tr>`,
        )
        .join("") +
      `</tbody></table>` +
      (r.indicaciones ? `<h2>Indicaciones</h2><p class="pre">${escapar(r.indicaciones)}</p>` : "") +
      `<p style="margin-top:48px">______________________________<br>${escapar(r.odontologo)}${r.registroProfesional ? ` · Reg. ${escapar(r.registroProfesional)}` : ""}</p>` +
      `<p class="pie">Firma electrónica simple (código MFA) · huella SHA-256 ${escapar(r.hashFirma)}. Validez legal sujeta a la normativa de prescripción del país.</p>`,
  );

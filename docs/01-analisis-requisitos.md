# 1. Análisis de requisitos — SIGO-Dental

Fuente: *Documento de Especificación de Requerimientos de Software (SRS/BRD) v1.0, 29/10/2026*.
Convención usada en este documento:

- **RF / RNF / RN**: requisito tal como aparece en el SRS.
- **S-xx (supuesto)**: decisión menor tomada por el equipo para poder avanzar. No está en el SRS.
- **P-xx (propuesta)**: funcionalidad que el SRS menciona sin especificar, o que se sugiere agregar. Requiere aprobación.
- **Q-xx (pregunta)**: decisión que corresponde al cliente y que bloquea alguna etapa futura.

---

## 1.1 Inventario de requisitos

### Requerimientos funcionales (14)

| ID | Módulo | Resumen | Obligatorio en el SRS |
|---|---|---|---|
| RF-01 | Pacientes | Ficha única: datos demográficos, contacto de emergencia, tutor legal si es menor, seguro/convenio. Alta, consulta y modificación. | Sí |
| RF-02 | Pacientes | Anamnesis **obligatoria** (enfermedades sistémicas, alergias, antecedentes quirúrgicos, coagulopatías). Condición crítica ⇒ *badge* de alta visibilidad en la cabecera del expediente. | Sí |
| RF-03 | Clínico | Odontograma 2D interactivo: 32 permanentes y 20 temporales; FDI y Universal; marcado por superficie (M, D, O/I, V, L/P); catálogo visual (caries rojo, restauración azul, endodoncia realizada/requerida, implante, prótesis fija/removible, exodoncia, sellante). | Sí |
| RF-04 | Clínico | Periodontograma: profundidad de sondaje en 6 puntos por diente, margen gingival, cálculo de NIC, furca, movilidad, sangrado. | Sí |
| RF-05 | Clínico | Notas SOEP en bitácora cronológica inmutable; bloqueo tras plazo configurable (24 h por defecto); firma del profesional. | Sí |
| RF-06 | Clínico | Consentimiento informado por procedimiento, con firma en pantalla táctil u OTP enviado al paciente. | Sí |
| RF-07 | Presupuestos | Presupuestos por fases/sesiones: honorarios, costo de laboratorio, descuentos autorizados. | Sí |
| RF-08 | Presupuestos | Aprobación total o parcial; ítems aprobados ⇒ tareas agendables. | Sí |
| RF-09 | Agenda | Calendario día/semana filtrable por consultorio/sillón y profesional. | Sí |
| RF-10 | Agenda | Estados: Programado, Confirmado, En sala de espera, En atención, Atendido, Cancelado, No asistió. | Sí |
| RF-11 | Agenda | Mensajes de confirmación y recordatorios 48 h y 24 h vía WhatsApp/SMS/Email. | Sí |
| RF-12 | Caja | Comprobantes electrónicos autorizados por la autoridad tributaria local; pagos mixtos (efectivo, tarjetas, transferencia, seguros). | Sí |
| RF-13 | Honorarios | Comisiones configurables: % fijo, monto fijo por procedimiento, % tras deducir laboratorio/materiales. | Sí |
| RF-14 | Inventario | Descargo automático de insumos al completar un procedimiento, según receta estándar de consumo. | Sí |

### Requerimientos no funcionales (8)

| ID | Resumen | ¿Se resuelve con código o con infraestructura? |
|---|---|---|
| RNF-01 | Expediente + odontograma < 1,5 s con red ≥ 15 Mbps | Ambos. Código (consultas indexadas, carga mínima) + medición con datos y hardware definidos. |
| RNF-02 | Disponibilidad 99,9 % de 07:00 a 21:00 | **Infraestructura** (redundancia, monitoreo, despliegue). El código sólo aporta *health check* y arranque/parada limpios. |
| RNF-03 | 150 usuarios concurrentes sin degradación | Ambos. Diseño sin estado en memoria + prueba de carga. |
| RNF-04 | TLS 1.3; AES-256 en reposo para BD y archivos | **Infraestructura** (proxy inverso/TLS, cifrado de disco/BD gestionada, *bucket* cifrado). El código aplica cabeceras de seguridad (helmet) y no guarda datos clínicos en el navegador (sin caché de API en la PWA). |
| RNF-05 | RBAC con menor privilegio; MFA para expedientes e información financiera; cierre tras 15 min de inactividad | **Código** (implementado en la Etapa 1). |
| RNF-06 | Log inmutable de cada CRUD sobre historias clínicas: quién, qué, cuándo (ISO-8601), IP | **Código + BD** (implementado en la Etapa 1 para autenticación y administración; se extiende a cada entidad clínica en las Etapas 2–6). |
| RNF-07 | Responsive: 1920×1080, 1366×768, tablets ≥ 10,5" | Código (frontend). |
| RNF-08 | Últimas 2 versiones de Chrome, Firefox, Edge y Safari | Código + pruebas en navegadores. |

### Reglas de negocio (3)

| ID | Regla | Dónde se garantiza |
|---|---|---|
| RN-01 | Un procedimiento "Realizado" no se borra; las correcciones son adendas firmadas. | API (no existe endpoint de borrado) **y** BD (trigger que rechaza `DELETE`/`UPDATE` de campos clínicos). Etapa 3. |
| RN-02 | Cierre de caja ciego: el recepcionista no ve el total teórico antes de declarar el efectivo contado. | API (el endpoint del recepcionista nunca devuelve el teórico) + permisos (`caja.revisar` sólo Auditor). Etapa 5. |
| RN-03 | Nunca dos citas simultáneas en el mismo sillón, aunque los odontólogos sean distintos. | API (validación con mensaje claro) **y** BD (restricción `EXCLUDE USING gist` que también resuelve peticiones concurrentes). Etapa 2. |

### Perfiles de usuario

Administrador TI, Recepcionista/Cajero, Odontólogo General/Especialista, Asistente Dental y Auditor/Contador. La matriz detallada está en [04-matriz-permisos.md](04-matriz-permisos.md).

### Integraciones externas (sección 7.1)

Facturación electrónica (REST), pasarela de pagos (Stripe, dLocal o pasarela bancaria), mensajería (Meta Cloud API para WhatsApp; Twilio para SMS/Email) y PACS/DICOM (**futuro**, fuera de la Fase 1).

### Exclusiones (Fase 1)

App nativa para pacientes (se abordará con PWA en Fase 2) y procesamiento de imágenes DICOM 3D (sólo almacenamiento/visualización de archivos planos).

### Criterios de aceptación técnica (8.2)

Cobertura ≥ 80 % en endpoints críticos; DAST/SAST sin hallazgos Altos o Críticos (OWASP Top 10); especificación OpenAPI 3.

---

## 1.2 Obligatorio vs. sugerido

| Elemento | Naturaleza | Decisión |
|---|---|---|
| React/Angular/Vue + Tailwind | Sugerido ("Arquitectura sugerida") | React 19 en **JavaScript** + Tailwind 4 + Vite, como PWA (stack del curso). |
| Backend | No especificado | **Go 1.24 + Fiber v2**, pgx y go-redis (stack del curso, mismo de catalogo-multinivel). |
| Microservicios | Sugerido y el propio diagrama dice "Microservicio / Modular" | **Monolito modular** (ver 02-arquitectura). |
| PostgreSQL o MySQL | Sugerido | PostgreSQL (necesario para `EXCLUDE` en RN-03 y `jsonb`). |
| Redis | Sugerido | Se usa como caché *Cache-Aside* de catálogos y parámetros (igual que en catalogo-multinivel). Si Redis no responde, la API sigue funcionando sin caché (`/api/health` = `degraded`). |
| S3/Azure Blob | Sugerido | Interfaz de almacenamiento con implementación local en disco para desarrollo. |
| JWT (en el diagrama) | Sugerido | **JWT HS256** en `Authorization: Bearer` (igual que catalogo-multinivel) **más** una tabla `sesiones` en el servidor indexada por el `jti` del token. Así se puede revocar al instante, exigir MFA y cerrar por inactividad, cosa que un JWT solo no permite. |
| Stripe, dLocal, Meta, Twilio | Ejemplos de proveedores | Adaptadores intercambiables; simulados en desarrollo. |
| TLS 1.3, AES-256, 99,9 % | Obligatorios | Dependen de la infraestructura de despliegue; se documentan, no se "cumplen" desde el código. |

---

## 1.3 Ambigüedades, contradicciones y vacíos

| # | Hallazgo | Tipo | Tratamiento |
|---|---|---|---|
| A-01 | **Recetas**: aparecen en los objetivos (2.2), en los permisos del odontólogo y en la Fase 2, pero no hay un RF que las describa. | Vacío | **P-01**: módulo de recetas (medicamento, dosis, vía, frecuencia, duración, indicaciones; PDF firmado). Se reservó el permiso `recetas.escribir`. Pendiente de aprobación (**Q-03**). |
| A-02 | **Reportes BI** en la Fase 3 sin RF. | Vacío | **P-02**: reportes operativos derivados de los RF (cierres de caja, liquidación de honorarios, consumo de insumos, ausentismo). BI externo fuera del alcance. |
| A-03 | **Recuperación de contraseña** no mencionada. | Vacío | **S-01**: en la Etapa 1, el administrador restablece la contraseña (temporal + cambio obligatorio). **P-03**: autoservicio por correo cuando exista el adaptador de email. |
| A-04 | RF-05 dice "bitácora **inmutable**" y a la vez "bloqueada **transcurrido** un plazo": implica que antes del plazo sí se puede editar. | Contradicción aparente | **S-02**: la nota es editable sólo por su autor y sólo mientras no esté firmada **y** no haya vencido el plazo. Firmar la bloquea de inmediato. Después del bloqueo sólo se agregan adendas (RN-01). Cada edición previa al bloqueo se audita con el valor anterior. |
| A-05 | RF-05 exige "firma **digital**" sin definir el tipo. | Ambigüedad legal | **S-03**: se implementará **firma electrónica simple**: re-autenticación del profesional (código TOTP), hash SHA-256 del contenido, fecha/hora e identidad (nombre + registro profesional), todo inmutable. **No** es una firma digital criptográfica con certificado X.509 ni se afirma su validez legal. Se deja una interfaz para un proveedor de firma con certificado (**Q-02**). |
| A-06 | RF-06 habla de "firma **biométrica**". | Ambigüedad técnica | Capturar un trazo en pantalla produce una **imagen de firma manuscrita** (con metadatos de fecha, dispositivo y hash del documento), no una verificación biométrica. Llamarla biométrica requeriría captura dinámica (presión, velocidad) y comparación con una muestra; no se promete. La alternativa OTP sí se implementará (**Q-02**). |
| A-07 | RNF-05 pide MFA "para acceso a expedientes e información financiera". Casi todos los roles acceden a una de las dos. | Alcance | **S-07**: MFA obligatorio para todos los usuarios; la administración de usuarios y la auditoría también se protegen (contienen quién consultó qué paciente). |
| A-08 | El Asistente "toma notas clínicas", pero RF-05 exige firma del profesional. | Ambigüedad | **S-04**: el asistente registra signos vitales y **borradores** de notas; sólo el odontólogo firma (permiso `clinica.firmar`). |
| A-09 | RN-02 no dice quién ve la diferencia del cierre ciego, ni si hay tolerancia o aprobación. | Vacío | **S-05**: el recepcionista declara el conteo y recibe sólo la confirmación; el Auditor (`caja.revisar`) ve teórico, contado y diferencia. Tolerancia: configurable, 0 por defecto (**Q-04**). |
| A-10 | "Seguros" como medio de pago: no se distingue copago de cuenta por cobrar a la aseguradora. | Ambigüedad | **S-06**: el seguro se registra como un medio de pago que genera una cuenta por cobrar a la aseguradora; el copago se cobra con los demás medios. |
| A-11 | RF-12 menciona la "autoridad tributaria local" sin indicar el país. | **Dato faltante** | No se asume ninguna autoridad. Interfaz Go `ProveedorFacturacion` con un proveedor **simulado** claramente rotulado (**Q-01**), igual que la mensajería (`notificaciones.Mensajeria` → `Simulada`). |
| A-12 | RF-11: qué hacer si la cita se agenda con menos de 48 h; consentimiento del paciente para WhatsApp; horario de envío. | Vacío | **S-08** (implementado en la Etapa 2): un recordatorio cuyo momento ya pasó se marca OMITIDO con el motivo; sin autorización de contacto o sin teléfono/correo, todos los mensajes quedan OMITIDOS; el canal preferido y la autorización se registran en la ficha. La restricción de horario de envío (p. ej. 08:00–20:00) queda como mejora pendiente. |
| A-13 | "Agendamiento multirrecinto" (RF-09): ¿varias sedes o varios consultorios? | Ambigüedad | El modelo admite **Sede → Consultorio → Sillón** desde el inicio; con una sola sede funciona igual. |
| A-14 | El objetivo 2.2 pide inventario "por consultorio **y profesional**"; RF-14 sólo describe el descargo. | Ambigüedad | Stock por ubicación (consultorio/bodega). "Por profesional" se resuelve como reporte de consumo por profesional, no como stock separado. |
| A-15 | RNF-01 no define volumen de datos ni hardware. | Medición incompleta | Protocolo de medición propuesto en la Etapa 7 (expediente con 10 años de historia, 200 notas, odontograma completo). |
| A-16 | El diagrama sugiere microservicios pero también dice "Modular". | Ambigüedad | Monolito modular con límites claros, extraíble a servicios si alguna vez hiciera falta. |
| A-17 | El Auditor/Contador es financiero; ¿accede a la bitácora de seguridad? | Ambigüedad | **S-11**: no; `auditoria.leer` sólo para el Administrador TI. El Auditor tendrá trazas financieras propias (**Q-05**). |
| A-18 | Catálogo visual: sólo caries (rojo) y restauración (azul) tienen color. | Vacío | **P-04**: convención habitual — **rojo = hallazgo/pendiente**, **azul = existente/realizado** (p. ej. endodoncia requerida en rojo, realizada en azul), con símbolos distintos por tipo y leyenda visible. |
| A-19 | RF-13 "porcentaje tras deducir costos de laboratorio/materiales": ¿el costo de materiales sale del inventario? | Ambigüedad | **S-12**: costo de laboratorio del ítem de presupuesto + costo estándar de la receta de consumo (RF-14). |
| A-20 | RF-07 "descuentos autorizados": no dice quién autoriza ni el tope. | Vacío | **S-13**: tope de descuento configurable por rol; superarlo exige aprobación de un usuario con permiso (**Q-06**). |
| A-21 | RN-03 no dice qué estados ocupan el sillón. | Vacío | **S-14**: *Cancelado* y *No asistió* liberan el sillón; todos los demás lo ocupan. |
| A-22 | RNF-05: el cierre por inactividad se pide "en estaciones de trabajo clínicas". | Alcance | Se aplica a todas las sesiones (más simple y más seguro). Configurable entre 5 y 15 min. |
| A-23 | Cumplimiento normativo (LOPDP/HIPAA, 2.2). | Legal | El sistema aporta controles técnicos (MFA, auditoría, cifrado), pero el cumplimiento depende de procesos, contratos y del país. No se declara "cumplimiento". |
| A-24 | Las fases del SRS ponen "facturación básica" en la Fase 1; este plan la ubica en la Etapa 5. | Diferencia de orden | El orden de etapas sigue la instrucción del proyecto (base → pacientes y agenda → clínica → finanzas). Se puede adelantar si el cliente lo pide. |

Otros supuestos:

- **S-09** Política de contraseñas: ≥ 10 caracteres, al menos una letra y un número, sin incluir el usuario del correo; hash **bcrypt** (costo 12), como en catalogo-multinivel.
- **S-10** Bloqueo tras 5 intentos fallidos durante 15 min (ambos configurables).
- **S-15** Duración máxima absoluta de sesión: 12 h, aunque haya actividad.
- **S-16** (Etapa 2) Criterio de alerta **crítica**: alergia a medicamento o material dental, anticoagulado, coagulopatía u "otra condición crítica" escrita por el profesional. Enfermedades sistémicas, embarazo y alergias alimentarias son **advertencias**. Las alertas se calculan siempre desde la anamnesis vigente.
- **S-17** (Etapa 2) Se admite registrar una cita que empezó hace hasta 15 min (paciente que llega sin turno); más atrás no.
- **S-18** (Etapa 2) Duración de cita: múltiplo de 5 min, entre 10 min y 8 h. "No asistió" sólo puede marcarse desde la hora de inicio; cancelar exige motivo; sólo se reprograman citas Programadas o Confirmadas.
- **S-19** (Etapa 2) Las citas y los pacientes no se borran (trigger en BD): se cancelan o se corrigen, para conservar trazabilidad.
- **P-05** (propuesta) Impedir también que un mismo odontólogo tenga dos citas simultáneas. El SRS sólo lo exige por sillón (RN-03), por eso **no** se implementó; puede activarse con una restricción análoga si la clínica lo pide.

## 1.4 Preguntas para el cliente

Ninguna bloquea las Etapas 1 y 2. Se necesitan antes de la etapa indicada.

| # | Pregunta | Necesaria antes de |
|---|---|---|
| Q-01 | ¿En qué país operará la clínica? Determina la autoridad tributaria (p. ej. SRI, SAT, DIAN, SUNAT), el formato del comprobante y la ley de protección de datos aplicable. | Etapa 5 (integración real) |
| Q-02 | ¿Qué tipo de firma se exige para notas SOEP y consentimientos: firma electrónica simple (re-autenticación + hash) o firma con certificado de una entidad acreditada? | Etapa 3 |
| Q-03 | ¿Se incluye el módulo de recetas (P-01) en esta versión? | Etapa 3 |
| Q-04 | Cierre ciego: ¿el recepcionista ve la diferencia después de declarar? ¿Hay tolerancia? ¿Quién aprueba un descuadre? | Etapa 5 |
| Q-05 | ¿El Auditor/Contador debe consultar la bitácora de accesos clínicos? | Etapa 5 |
| Q-06 | ¿Quién autoriza descuentos y con qué tope? | Etapa 4 |

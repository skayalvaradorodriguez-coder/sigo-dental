# REQUIREMENTS CHECKLIST — SIGO-Dental

> Fuente de verdad del progreso. Leer al inicio de cada sesión; marcar `- [x]`
> y actualizar "Estado General" al completar cualquier tarea.
>
> Una casilla se marca sólo cuando su prueba pasa (ver `docs/05-trazabilidad-y-pruebas.md`).
> Lo que depende de infraestructura o de mediciones queda como ⏳ **pendiente de validación**,
> aunque el código esté preparado.

## Estado General

| Campo | Valor |
|---|---|
| **Etapa actual** | Etapa 6 completada · siguiente: Etapa 7 (calidad y salida) |
| **Última tarea completada** | Etapa 6: inventario con kardex, descargo automático y atómico al registrar procedimientos, recetas de consumo; honorarios con las tres reglas, precedencia y liquidación por periodo (2026-10-30) |
| **Siguiente tarea pendiente** | E7-01: medición de RNF-01 (expediente < 1,5 s) con volumen de datos definido |
| **Pruebas** | Go: 33 unitarias + 61 de integración ✅ · Frontend: 29 (Vitest) ✅ · Humo PowerShell ✅ · Navegador: 21 (Etapas 1–2) + 10 (Etapa 3) + 12 (Etapa 4) + 7 (Etapa 5) + 6 (Etapa 6) pasos ✅ |
| **Cobertura backend** | Capa de endpoints 80,7 % ✅ (meta SRS 8.2: ≥ 80 %) · total del backend 80,4 % |
| **Bloqueadores** | Ninguno para la Etapa 7. Preguntas abiertas al cliente: Q-01…Q-06 (`docs/01-analisis-requisitos.md` §1.4); Q-01 (país) impide conectar la facturación real |

## Stack (igual a catalogo-multinivel)

- [x] Frontend React 19 + **JavaScript** + Vite + Tailwind + React Router, en la raíz
- [x] PWA (`vite-plugin-pwa`): manifiesto, íconos, service worker, aviso de actualización, instalar app — **sin caché de datos clínicos**
- [x] Backend Go + Fiber en `sigo-backend/`: `routes → controllers → repository`, `models.APIError`
- [x] PostgreSQL 16 (pgx) y Redis (go-redis, Cache-Aside, degradación sin Redis)
- [x] JWT (Bearer) + bcrypt; `apiFetch` único en `services/http.js`
- [x] Inyección de dependencias por interfaces (`DBRepository`, `CacheRepository`, `Mensajeria`)
- [x] Goroutine en segundo plano con apagado ordenado (`WaitGroup`)
- [x] `scripts/prueba-api.ps1`, `PROJECT_MAP.md`, `.windsurfrules`, plan por etapas
- [ ] Dockerfile + `render.yaml` + Neon (Etapa 7)

## Requerimientos funcionales (SRS §4)

### Pacientes
- [x] **RF-01** Ficha única: datos demográficos, contacto de emergencia, convenio, alta/consulta/modificación
  - [x] Tutor legal obligatorio para menores de 18 (API `TUTOR_REQUERIDO` + formulario)
  - [x] Documento único por tipo (409); fecha de nacimiento no futura
  - [x] Modificaciones auditadas con valor anterior y nuevo; lecturas y búsquedas auditadas
  - [x] Pacientes no se borran (trigger)
- [x] **RF-02** Anamnesis obligatoria y alertas
  - [x] Enfermedades sistémicas, alergias (tipo y reacción), antecedentes quirúrgicos, coagulopatía, anticoagulado, embarazo, otra condición crítica
  - [x] Versionada; versiones anteriores inmutables (trigger)
  - [x] *Badge* rojo de alerta crítica en la cabecera, la lista, la agenda y el detalle de la cita
  - [x] Recepción ve las alertas pero no el detalle clínico
  - [x] Bloquear registros clínicos si la anamnesis está pendiente (409 `ANAMNESIS_PENDIENTE`)

### Clínico
- [x] **RF-03** Odontograma 2D interactivo (SVG): 32 permanentes y 20 temporales, FDI y Universal, superficies M/D/O-I/V/L-P, catálogo visual (caries rojo, restauración azul, endodoncia requerida/realizada, implante, prótesis fija/removible, exodoncia, sellante, ausente)
  - [x] Cada cambio es un evento: el historial no se pierde (sólo inserción en la BD)
- [x] **RF-04** Periodontograma: sondaje en 6 puntos, margen gingival, NIC calculado por la BD, furca, movilidad, sangrado, resumen
- [x] **RF-05** Notas SOEP: bitácora cronológica, bloqueo tras el plazo configurable (24 h) en API y BD, firma del profesional (firma electrónica simple con MFA + hash), adendas
- [x] **RF-06** Consentimiento informado por procedimiento: plantillas versionadas, vista previa del texto exacto, firma en pantalla (imagen del trazo, **no** biométrica) u OTP (proveedor **SIMULADO**), tutor para menores, inmutable con SHA-256
- [x] **P-01** Recetas con revisión de alergias/anticoagulación, confirmación explícita y firma con MFA (Q-03 abierta: validez legal según el país)
- [x] Archivos del paciente (radiografías, fotos, PDF): tipo por contenido, ≤ 10 MB, SHA-256 verificado al descargar

### Presupuestos (Etapa 4)
- [x] **RF-07** Presupuestos por fases/sesiones: honorarios, laboratorio, descuentos; sobre el tope (configurable, 10 %) los autoriza otro odontólogo con MFA
- [x] **RF-08** Aprobación total o parcial por ítem → una tarea por ítem aprobado; agendar y registrar como realizado desde la tarea

### Agenda
- [x] **RF-09** Calendario día/semana por sillón y profesional, multisede (sede → consultorio → sillón)
- [x] **RF-10** Estados Programado, Confirmado, En sala de espera, En atención, Atendido, Cancelado, No asistió, con historial
- [x] **RF-11** Confirmación y recordatorios 48 h y 24 h por canal preferido — proveedor **SIMULADO** rotulado
  - [ ] ⏳ Proveedor real (WhatsApp Business / Twilio) — requiere credenciales del cliente

### Caja y facturación (Etapa 5)
- [x] **RF-12** Pagos mixtos (efectivo, tarjeta, transferencia, seguro → cuenta por cobrar) que cuadran al centavo (API y BD); comprobante vía adaptador `facturacion.Proveedor` con proveedor **SIMULADO**; anulación con nota de crédito; reintento si la facturación falla
  - [ ] ⏳ Proveedor de facturación real e impuestos — requiere el país y su autoridad tributaria (Q-01)

### Honorarios e inventario (Etapa 6)
- [x] **RF-13** Reglas de honorarios: % de lo cobrado, monto fijo por procedimiento y % del neto (cobrado − laboratorio del presupuesto − materiales del inventario); por odontólogo y/o procedimiento con vigencia; liquidación por periodo, una sola vez por cobro
- [x] **RF-14** Descargo automático de insumos según la receta de consumo, en la misma transacción que el procedimiento realizado (si falla, no queda ninguno); stock por ubicación, kardex de sólo inserción, alertas de mínimo y saldo negativo

## Reglas de negocio (SRS §6)
- [x] **RN-01** Procedimiento realizado no se borra ni se edita (trigger en la BD); correcciones por adenda firmada
- [x] **RN-02** Cierre de caja ciego: ninguna respuesta a `caja.operar` contiene el teórico (prueba que revisa el JSON completo); teórico guardado en la misma transacción del cierre; sólo el Auditor (`caja.revisar`) concilia, con tolerancia configurable y nota obligatoria si hay diferencias
- [x] **RN-03** Sin dos citas simultáneas en el mismo sillón: `EXCLUDE USING gist` + 409 con la cita en conflicto; 20 peticiones simultáneas ⇒ 1 éxito

## Requerimientos no funcionales (SRS §5)
- [ ] ⏳ **RNF-01** Expediente < 1,5 s — medición en la Etapa 7
- [ ] ⏳ **RNF-02** Disponibilidad 99,9 % — infraestructura
- [ ] ⏳ **RNF-03** 150 usuarios concurrentes — prueba de carga en la Etapa 7
- [ ] ⏳ **RNF-04** TLS 1.3 y AES-256 en reposo — infraestructura (Etapa 7)
- [x] **RNF-05** Seguridad de acceso
  - [x] RBAC con menor privilegio; toda ruta con política (prueba automática)
  - [x] MFA TOTP obligatorio, anti-replay, rotación de token, enrolamiento con QR
  - [x] Cierre por inactividad (15 min, configurable 5–15) en servidor y cliente
  - [x] Bloqueo tras 5 intentos (configurable), límite de 10 logins/min, sin enumeración de usuarios
- [x] **RNF-06** Auditoría inmutable: quién, qué, cuándo (ISO-8601), IP; cadena de hashes verificable
  - [x] Autenticación, administración, pacientes (lecturas y cambios), anamnesis, citas
  - [x] Odontograma, periodontograma, notas, adendas y procedimientos (lecturas y escrituras); firmas fallidas
  - [x] Presupuestos, autorizaciones de descuento, decisiones, consentimientos, recetas y archivos (lecturas y escrituras); archivo alterado
  - [x] Caja: apertura, cobros, anulaciones, comprobantes, cierre y revisión
  - [x] Inventario (insumos, movimientos, recetas) y honorarios (reglas, liquidaciones)
- [x] **RNF-07** Responsive: escritorio y celular verificados · [ ] tablet 10,5" y 1920×1080 automatizados (Etapa 7)
- [ ] **RNF-08** Navegadores: hoy probado en Chromium; Firefox, WebKit/Safari y Edge en la Etapa 7

## Criterios técnicos (SRS §8.2)
- [x] OpenAPI 3 generado del código (`go run . openapi`)
- [x] Cobertura ≥ 80 % en la capa de endpoints — medida 80,7 % (total 80,4 %)
- [ ] ⏳ DAST/SAST sin hallazgos Altos/Críticos (ZAP, gosec, govulncheck, npm audit) — Etapa 7

## Etapas

- [x] **Etapa 1 — Base** (configuración, BD, autenticación, roles, navegación)
- [x] **Etapa 2 — Pacientes y agenda**
- [x] **Etapa 3 — Núcleo clínico** (RF-03, RF-04, RF-05, RN-01)
- [x] **Etapa 4 — Planes y consentimientos** (RF-06, RF-07, RF-08, recetas, archivos)
- [x] **Etapa 5 — Caja y facturación** (RF-12, RN-02)
- [x] **Etapa 6 — Inventario y honorarios** (RF-13, RF-14)
- [ ] **Etapa 7 — Calidad y salida** (RNF-01…04, RNF-08, 8.2, despliegue Render + Neon)

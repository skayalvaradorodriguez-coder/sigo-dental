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
| **Etapa actual** | Etapas 1–7 completadas · pendiente sólo lo que depende del despliegue y de decisiones del cliente (⏳) |
| **Última tarea completada** | Etapa 7: mediciones de RNF-01 y RNF-03, pruebas de navegador por tamaño, SAST/DAST con hallazgos corregidos, CSP estricta, imagen Docker, blueprint de Render, integración continua y guía de despliegue (2026-10-30) |
| **Siguiente tarea pendiente** | Desplegar (DESPLIEGUE.md) y completar las verificaciones ⏳ de RNF-02, RNF-04 y RNF-08 (Firefox/WebKit/Edge en la CI) |
| **Pruebas** | Go: 34 unitarias + 63 de integración ✅ · Frontend: 29 (Vitest) ✅ · Playwright en el repositorio: 21 en Chromium (escritorio, tablet 834×1112, 1920×1080, celular) ✅ · Humo PowerShell ✅ · Recorridos de las Etapas 1–6 ✅ |
| **Cobertura backend** | Capa de endpoints 80,9 % ✅ (meta SRS 8.2: ≥ 80 %) · total del backend 80,0 % |
| **Bloqueadores** | Ninguno técnico. Para producción con datos reales: Q-01 (país → facturación real), mensajería real, Q-02 (tipo de firma) y textos legales (DESPLIEGUE.md §8) |

## Stack (igual a catalogo-multinivel)

- [x] Frontend React 19 + **JavaScript** + Vite + Tailwind + React Router, en la raíz
- [x] PWA (`vite-plugin-pwa`): manifiesto, íconos, service worker, aviso de actualización, instalar app — **sin caché de datos clínicos**
- [x] Backend Go + Fiber en `sigo-backend/`: `routes → controllers → repository`, `models.APIError`
- [x] PostgreSQL 16 (pgx) y Redis (go-redis, Cache-Aside, degradación sin Redis)
- [x] JWT (Bearer) + bcrypt; `apiFetch` único en `services/http.js`
- [x] Inyección de dependencias por interfaces (`DBRepository`, `CacheRepository`, `Mensajeria`)
- [x] Goroutine en segundo plano con apagado ordenado (`WaitGroup`)
- [x] `scripts/prueba-api.ps1`, `PROJECT_MAP.md`, `.windsurfrules`, plan por etapas
- [x] Dockerfile multi-etapa + `render.yaml` (Render + Neon) + `.github/workflows/ci.yml` + `DESPLIEGUE.md` · [ ] ⏳ construir la imagen y desplegar (en este entorno no hay acceso a Docker Hub)

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
- [x] **RNF-01** Expediente < 1,5 s — **medido en desarrollo** con un paciente de 10 años de historia: API p95 ≤ 35 ms por pestaña; navegador (Chromium, carga en frío) máx. 1 002 ms (`docs/mediciones/`) · [ ] ⏳ repetir en el entorno de producción
- [ ] ⏳ **RNF-02** Disponibilidad 99,9 % — depende de la infraestructura: monitor externo de `/api/health` tras desplegar (DESPLIEGUE.md §6)
- [x] **RNF-03** 150 usuarios concurrentes — **medido en desarrollo** (2 vCPU): 60 s, 5 160 pedidos, p95 9 ms, 0 % de errores (`go run . carga`) · [ ] ⏳ repetir en un entorno de pruebas igual al de producción
- [ ] ⏳ **RNF-04** TLS 1.3 y AES-256 en reposo — depende del proveedor (Render/Neon): verificar tras desplegar (DESPLIEGUE.md §6). Desde el código: HSTS en producción y la API no arranca sin TLS hacia la base
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
- [x] **RNF-07** Responsive: escritorio, celular, tablet 10,5" (834×1112) y 1920×1080 automatizados con Playwright, sin desplazamiento horizontal
- [x] **RNF-08** Chromium ✅ medido · [ ] ⏳ Firefox y WebKit (Safari): configurados en Playwright y en la CI (aquí no pudieron instalarse) · [ ] ⏳ Edge: proyecto `edge` (requiere Edge instalado, `scripts\medir.ps1`)

## Criterios técnicos (SRS §8.2)
- [x] OpenAPI 3 generado del código (`go run . openapi`)
- [x] Cobertura ≥ 80 % en la capa de endpoints — medida 80,9 % (total 80,0 %)
- [x] DAST/SAST sin hallazgos Altos/Críticos: gosec 0, npm audit (producción) 0, OWASP ZAP 0 altos/medios/bajos tras corregir 2 medios (`docs/seguridad/`) · [ ] ⏳ govulncheck (en la CI)

## Etapas

- [x] **Etapa 1 — Base** (configuración, BD, autenticación, roles, navegación)
- [x] **Etapa 2 — Pacientes y agenda**
- [x] **Etapa 3 — Núcleo clínico** (RF-03, RF-04, RF-05, RN-01)
- [x] **Etapa 4 — Planes y consentimientos** (RF-06, RF-07, RF-08, recetas, archivos)
- [x] **Etapa 5 — Caja y facturación** (RF-12, RN-02)
- [x] **Etapa 6 — Inventario y honorarios** (RF-13, RF-14)
- [x] **Etapa 7 — Calidad y salida** (mediciones, seguridad, navegadores, despliegue preparado) · ⏳ lo que depende de desplegar

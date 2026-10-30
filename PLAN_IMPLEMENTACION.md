# PLAN DE IMPLEMENTACIÓN — SIGO-Dental

> Plan checkeable por etapas, derivado del SRS v1.0 y de `docs/01-analisis-requisitos.md`.
>
> **Regla:** una tarea se marca `[x]` sólo cuando su verificación pasa (prueba en `go test`,
> `npm test`, `scripts/prueba-api.ps1` o recorrido en el navegador). Al cerrar cada etapa se
> actualizan `REQUIREMENTS_CHECKLIST.md`, `PROJECT_MAP.md` y `docs/05-trazabilidad-y-pruebas.md`.
>
> **Forma de trabajo (igual que catalogo-multinivel):** una rama por etapa
> (`etapa-N-nombre`), commits pequeños con mensaje en español, Pull Request hacia `main`
> con el resumen y las pruebas ejecutadas, y merge cuando todo pasa.
>
> **Principios:** denegar por defecto (toda ruta declara su política), reglas críticas también
> en la base de datos, lo clínico no se borra, datos ficticios, proveedores externos simulados
> y rotulados, ningún RNF se da por cumplido sin medición.

---

## ETAPA 1 — Base ✅ (rama `etapa-1-base`)

> Objetivo: configuración, base de datos, autenticación, roles y navegación.

- [x] `docker-compose.yml` (PostgreSQL 16 + Redis), `.env.example`, `.env.test`
- [x] Migración `001_base.sql`: `usuarios`, `sesiones`, `ajustes`, `auditoria` + triggers de inmutabilidad
- [x] Login con bcrypt, bloqueo tras N intentos, límite de 10 logins/min, mensaje sin enumeración
- [x] JWT con `jti` + tabla `sesiones`: revocación, máximo absoluto de 12 h, inactividad configurable (15 min)
- [x] MFA TOTP (RFC 6238) con anti-replay, enrolamiento con QR y rotación de token
- [x] Primer ingreso: contraseña temporal → cambio obligatorio → configurar MFA
- [x] RBAC: 5 roles, catálogo de permisos, `RequirePermisos`, prueba de "toda ruta tiene política"
- [x] Auditoría encadenada (SHA-256 + advisory lock) y verificación de integridad
- [x] Usuarios (alta, rol, desactivar, restablecer contraseña/MFA), ajustes, panel por rol
- [x] Frontend: pasos de ingreso, layout con menú por permisos, panel, usuarios, configuración, auditoría, aviso de inactividad
- [x] **Verificación:** `go test ./pruebas/` (auth y permisos) y recorrido con los 5 roles + usuario de primer ingreso

## ETAPA 2 — Pacientes y agenda ✅ (rama `etapa-2-pacientes-agenda`)

> Objetivo: recepción opera de punta a punta. RF-01, RF-02, RF-09, RF-10, RF-11, RN-03.

- [x] Migración `002_pacientes_agenda.sql`: catálogos, pacientes, tutores, anamnesis, citas (EXCLUDE), eventos, notificaciones
- [x] Catálogos sede → consultorio → sillón, procedimientos y convenios, con caché Redis e invalidación
- [x] Ficha única con tutor obligatorio para menores; documento único; auditoría de lecturas y cambios
- [x] Anamnesis versionada e inmutable; alertas CRÍTICA/ADVERTENCIA calculadas en `domain`
- [x] Agenda: crear, filtrar (sillón, odontólogo, paciente), máximo 31 días, vistas día/semana
- [x] RN-03: `EXCLUDE USING gist` + 409 `SILLON_OCUPADO` con la cita en conflicto
- [x] Estados (7) con máquina de transiciones, motivo obligatorio al cancelar, "No asistió" sólo desde la hora
- [x] Reprogramación con rehecho de recordatorios
- [x] Recordatorios: confirmación, −48 h, −24 h; OMITIDO con motivo; goroutine con `SKIP LOCKED`; proveedor SIMULADO
- [x] Frontend: pacientes, ficha, formulario, anamnesis, agenda, nueva cita, detalle, bandeja; catálogos (admin)
- [x] OpenAPI generado del registro de rutas (`go run . openapi`)
- [x] **Verificación:** 20 citas simultáneas ⇒ 1 éxito; `INSERT` directo superpuesto falla; humo PowerShell "TODO OK"

## ETAPA 3 — Núcleo clínico ✅ (rama `etapa-3-clinica`)

> RF-03, RF-04, RF-05, RN-01. Q-02 (tipo de firma) sigue abierta: se aplicó el supuesto S-03
> (firma electrónica simple). Q-03 (recetas) sigue abierta: P-01 no se implementó.

- [x] Migración `003_clinica.sql`: `odontograma_eventos`, `periodontogramas` + `perio_dientes` + `perio_sitios` (NIC generado), `notas_soep` + `notas_adendas`, `procedimientos_realizados` + `procedimientos_adendas`
- [x] Triggers: nota no editable si está firmada o vencida; procedimiento realizado sin `UPDATE` ni `DELETE` (RN-01); todo lo demás sólo inserción
- [x] Bloqueo de registros clínicos mientras la anamnesis esté pendiente (409 `ANAMNESIS_PENDIENTE`)
- [x] Odontograma SVG interactivo: 32/20 dientes, FDI ↔ Universal (se guarda FDI), superficies con I/P, catálogo con leyenda, historial
- [x] Periodontograma: 6 sitios, convención de signo del MG rotulada, NIC en la BD, furca, movilidad, sangrado, resumen
- [x] Notas SOEP: borrador → firma electrónica simple (código MFA + hash SHA-256 + registro profesional); adendas firmadas; verificación del hash
- [x] Procedimientos realizados firmados e inmutables con adendas
- [x] Auditoría de cada lectura y escritura clínica; firmas fallidas auditadas
- [ ] Almacenamiento de archivos (radiografías, documentos): se pasa a la Etapa 4 junto con los consentimientos
- [x] **Verificación:** edición rechazada a las 24 h por API **y** BD; borrado de procedimiento realizado imposible; tabla FDI↔Universal completa en pruebas; casos numéricos de NIC; recorrido en el navegador

## ETAPA 4 — Planes y consentimientos (rama `etapa-4-presupuestos`)

> RF-06, RF-07, RF-08. Requiere respuesta a Q-06 (tope de descuento).

- [ ] Presupuestos por fases y sesiones con honorarios, laboratorio y descuento (`NUMERIC`)
- [ ] Descuento sobre el tope del rol ⇒ requiere autorización
- [ ] Aprobación total o parcial ⇒ tareas agendables sólo de los ítems aprobados
- [ ] Consentimientos por procedimiento: plantilla, documento con hash, firma manuscrita digitalizada (no biométrica) u OTP (proveedor simulado); inmutable tras firmar
- [ ] Almacenamiento de archivos (interfaz; disco local en desarrollo) para consentimientos firmados y adjuntos clínicos
- [ ] **Verificación:** aprobar 2 de 3 ítems crea exactamente 2 tareas; documento firmado no se modifica

## ETAPA 5 — Caja y facturación (rama `etapa-5-caja`)

> RF-12, RN-02. Requiere respuesta a Q-01 (país) y Q-04 (cierre ciego).

- [ ] Sesión de caja, cobros con pagos mixtos (efectivo, tarjeta, transferencia, seguro → cuenta por cobrar)
- [ ] Cierre ciego: el recepcionista declara sin ver el teórico; el Auditor ve la diferencia
- [ ] Interfaz `ProveedorFacturacion` con proveedor SIMULADO rotulado (sin asumir autoridad tributaria)
- [ ] **Verificación:** la respuesta al recepcionista nunca incluye el teórico; sumas al centavo

## ETAPA 6 — Inventario y honorarios (rama `etapa-6-inventario-honorarios`)

> RF-13, RF-14.

- [ ] Insumos por ubicación, movimientos de stock sólo inserción, recetas de consumo por procedimiento
- [ ] Descargo automático en la misma transacción que el procedimiento realizado
- [ ] Reglas de honorarios (% fijo, monto fijo, % neto) y liquidación por periodo
- [ ] **Verificación:** casos numéricos de las 3 reglas; descargo atómico (si falla, nada se descuenta)

## ETAPA 7 — Calidad y salida (rama `etapa-7-calidad`)

> RNF-01…04, RNF-08, SRS 8.2. Nada de esto se declara cumplido sin la medición.

- [ ] Medición de RNF-01 con un expediente de volumen definido (Playwright / Lighthouse)
- [ ] Prueba de carga k6 con 150 usuarios virtuales (RNF-03), informe p95
- [ ] Playwright en Chromium, Firefox y WebKit; tablet 834×1112 y 1920×1080 (RNF-07, RNF-08)
- [ ] Cobertura ≥ 80 %; `gosec`, `govulncheck`, `npm audit`, OWASP ZAP sin Altos/Críticos
- [ ] Dockerfile multi-etapa, `render.yaml`, base en Neon, `DESPLIEGUE.md`; verificación de TLS 1.3 y cifrado en reposo del proveedor (RNF-04)

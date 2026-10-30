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
> (firma electrónica simple). Las recetas (P-01) se implementaron en la Etapa 4.

- [x] Migración `003_clinica.sql`: `odontograma_eventos`, `periodontogramas` + `perio_dientes` + `perio_sitios` (NIC generado), `notas_soep` + `notas_adendas`, `procedimientos_realizados` + `procedimientos_adendas`
- [x] Triggers: nota no editable si está firmada o vencida; procedimiento realizado sin `UPDATE` ni `DELETE` (RN-01); todo lo demás sólo inserción
- [x] Bloqueo de registros clínicos mientras la anamnesis esté pendiente (409 `ANAMNESIS_PENDIENTE`)
- [x] Odontograma SVG interactivo: 32/20 dientes, FDI ↔ Universal (se guarda FDI), superficies con I/P, catálogo con leyenda, historial
- [x] Periodontograma: 6 sitios, convención de signo del MG rotulada, NIC en la BD, furca, movilidad, sangrado, resumen
- [x] Notas SOEP: borrador → firma electrónica simple (código MFA + hash SHA-256 + registro profesional); adendas firmadas; verificación del hash
- [x] Procedimientos realizados firmados e inmutables con adendas
- [x] Auditoría de cada lectura y escritura clínica; firmas fallidas auditadas
- [x] Almacenamiento de archivos (radiografías, documentos): hecho en la Etapa 4
- [x] **Verificación:** edición rechazada a las 24 h por API **y** BD; borrado de procedimiento realizado imposible; tabla FDI↔Universal completa en pruebas; casos numéricos de NIC; recorrido en el navegador

## ETAPA 4 — Planes y consentimientos ✅ (rama `etapa-4-planes`)

> RF-06, RF-07, RF-08, P-01 (recetas) y archivos. Q-06 sin respuesta: se aplicó S-25 (tope
> configurable del 10 % y autorización por otro odontólogo).

- [x] Migración `004_planes.sql`: `presupuestos` + `presupuesto_items` (`NUMERIC`, descuento ≤ honorario), `tareas_tratamiento`, `citas.tarea_id` y `procedimientos_realizados.tarea_id` (índices únicos), `consentimiento_plantillas` (versionadas), `consentimientos`, `recetas` + `receta_items`, `archivos_paciente`
- [x] Triggers: borrador editable; presentado ⇒ sólo la decisión; decidido ⇒ nada; consentimiento firmado inmutable; plantillas, tareas, recetas y archivos sólo inserción
- [x] Presupuestos por fases y sesiones, totales en centavos (Go y JS), descuento sobre el tope ⇒ autorización de otro odontólogo con MFA, presentación con hash
- [x] Aprobación total o parcial ⇒ una tarea por ítem aprobado; cita y procedimiento realizado desde la tarea (estado derivado)
- [x] Consentimientos: vista previa del texto exacto, firma en pantalla (PNG) u OTP (proveedor SIMULADO, 10 min, 5 intentos), tutor para menores, impresión
- [x] Recetas con alertas de alergia (reactividad cruzada) y anticoagulación, confirmación y firma MFA, impresión
- [x] Archivos: interfaz `Almacen` (disco local; memoria en pruebas), tipo por contenido, SHA-256 verificado al descargar
- [x] **Verificación:** aprobar 2 de 3 ítems crea exactamente 2 tareas; ni la API ni la BD modifican un presupuesto presentado ni un consentimiento firmado; archivo alterado ⇒ 409; recorrido de 12 pasos en el navegador

## ETAPA 5 — Caja y facturación ✅ (rama `etapa-5-caja`)

> RF-12, RN-02. Q-01 (país) y Q-04 (cierre ciego) sin respuesta: proveedor de facturación SIMULADO
> y supuestos S-31…S-34.

- [x] Migración `005_caja.sql`: `caja_sesiones` (una abierta por cajero; teórico y declarado al cerrar), `cobros`, `cobro_conceptos`, `cobro_pagos`, `cobros_anulaciones`, `comprobantes` (cada intento) y secuencias
- [x] Guardias en la BD: cobro/anulación sólo en caja abierta (`FOR SHARE`); conceptos = pagos = total al confirmar (trigger diferido); un realizado se cobra una vez; todo sólo inserción; sesión ABIERTA → CERRADA → REVISADA
- [x] Cobro con pagos mixtos; montos fijados por el sistema (presupuesto aprobado o arancel); seguro → cuenta por cobrar
- [x] Puerto `facturacion.Proveedor` + proveedor SIMULADO; factura, nota de crédito al anular, reintento si falla
- [x] Cierre ciego: la recepción declara sin ver el teórico; el Auditor concilia por medio, con tolerancia configurable y nota obligatoria si hay diferencias
- [x] Pantallas: caja de recepción (apertura, cobro con cambio, cobros, anulación, cierre), conciliación del Auditor, cuentas por cobrar, pestaña Pagos de la ficha, panel
- [x] **Verificación:** ninguna respuesta al recepcionista incluye el teórico (se revisa el JSON completo); sumas al centavo; 10 cobros simultáneos del mismo procedimiento ⇒ 1 éxito; cobro después del cierre rechazado por la API y la BD

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

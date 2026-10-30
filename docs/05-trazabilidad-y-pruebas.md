# 5. Trazabilidad y pruebas

Estados:

- ✅ **Ejecutada**: la prueba existe en el repositorio y pasa.
- 🟡 **Propuesta**: diseñada; se implementa en la etapa indicada.
- ⏳ **Validación pendiente**: requiere medición, infraestructura o una decisión del cliente. **No** se da por cumplida desde el código.

## 5.1 Cómo se ejecutan

| Suite | Comando | Qué necesita | Resultado (29–30/10/2026) |
|---|---|---|---|
| Unitarias Go (`domain`, `seguridad`, `permisos`, `notificaciones`, `repository`) | `cd sigo-backend && go test ./domain/... ./seguridad/... ./permisos/... ./notificaciones/... ./repository/...` | Nada | ✅ 17 pruebas |
| Integración Go (`pruebas/`) contra la API y una base real | `docker compose up -d` y luego `go test ./pruebas/` (usa `.env.test` → base `sigo_dental_test`, la reinicia en cada corrida) | PostgreSQL | ✅ 38 pruebas |
| Todo el backend con cobertura | `go test -coverpkg=./... -coverprofile=cobertura.out ./...` y `go tool cover -func=cobertura.out` | PostgreSQL | ✅ cobertura **76,3 %** de sentencias (78,9 % sin `main.go`) |
| Unitarias del frontend (Vitest + Testing Library) | `npm test` | Nada | ✅ 11 pruebas |
| Lint del frontend | `npm run lint` | Nada | ✅ sin errores |
| Prueba de humo por comandos | `powershell -ExecutionPolicy Bypass -File scripts\prueba-api.ps1` (dentro de `sigo-backend`) | API corriendo | ✅ "TODO OK" (todas las comprobaciones en [OK], también al ejecutarlo dos veces seguidas) |
| Recorrido en navegador (Playwright, 5 roles, escritorio y celular) | Script de verificación usado durante el desarrollo; capturas en `docs/capturas/` | API + `npm run preview` | ✅ 21 pasos |

> **Cobertura (8.2 del SRS pide ≥ 80 % en endpoints críticos):** la medición real da 76,3 % sobre todo el backend. Lo no cubierto son sobre todo `main.go` (comandos de consola) y ramas de error de infraestructura (caída de la BD a mitad de una transacción). Se registra como **pendiente** y se subirá en la Etapa 3; no se declara cumplido.

## 5.2 Matriz requisito → prueba

| Requisito | Dónde (API / Web) | Etapa | Prueba | Estado |
|---|---|---|---|---|
| RF-01 Ficha única | `pacienteController` / Pacientes, ficha, formulario | 2 | Alta adulto; **menor sin tutor ⇒ 400 `TUTOR_REQUERIDO`**; no se quita el tutor a un menor; documento duplicado ⇒ 409; fecha futura y teléfono inválido ⇒ 400; edición auditada con valor anterior y nuevo | ✅ `TestRegistroPacienteAdulto`, `TestMenorExigeTutor`, `TestDocumentoUnicoYValidaciones`, `TestModificacionAuditaValores` |
| RF-02 Anamnesis y alertas | `pacienteController`, `domain/alertas.go` / cabecera roja | 2 | Penicilina y anticoagulado ⇒ CRÍTICA; versionado; las alertas usan la versión vigente; la BD rechaza `UPDATE`/`DELETE`; recepción ve alertas pero no la anamnesis (403) | ✅ `TestAlertasPenicilinaYAnticoagulado`, `TestAlertaPenicilinaCritica`, `TestAnamnesisVersionada`, `TestReglasEnBaseDeDatos`, Vitest `AlertasMedicas` · bloqueo del expediente sin anamnesis 🟡 Etapa 3 |
| RF-03 Odontograma | — | 3 | 32/20 dientes; FDI↔Universal (tabla completa); registro por superficie; historial preservado | 🟡 |
| RF-04 Periodontograma | — | 3 | 6 sitios; NIC = PS + MG con los ejemplos de 03-modelo-datos §3.5; valores fuera de rango rechazados | 🟡 |
| RF-05 Notas SOEP | — | 3 | Editable antes del plazo; **rechazo después del bloqueo (API y trigger)**; firma con re-autenticación y hash | 🟡 · el plazo configurable (24 h por defecto) ✅ `TestAjustes` |
| RF-06 Consentimiento | — | 4 | Documento con hash; firma dibujada u OTP; no editable tras firmar | 🟡 |
| RF-07 Presupuestos | — | 4 | Totales por fase con honorarios, laboratorio y descuento; descuento sobre el tope requiere autorización | 🟡 |
| RF-08 Aprobación parcial | — | 4 | Aprobar 2 de 3 ítems crea exactamente 2 tareas agendables | 🟡 |
| RF-09 Agenda | `citaController` / Agenda día y semana | 2 | Filtros por sillón y odontólogo; rango máx. 31 días; validaciones (pasado, duración, no odontólogo); permisos | ✅ `TestFiltrosAgenda`, `TestValidacionesYPermisosAgenda` · vistas día/semana: recorrido en navegador |
| RF-10 Estados | `domain/citas.go` / Detalle de la cita | 2 | Flujo completo con historial (5 eventos); transición inválida ⇒ 400; cancelar exige motivo; "No asistió" sólo desde la hora; reprogramar rehace recordatorios | ✅ `TestTransicionesCita`, `TestFlujoDeEstados`, `TestReprogramar`, Vitest máquina de estados |
| RF-11 Recordatorios | `notificaciones` / Bandeja | 2 | Confirmación, −48 h y −24 h; cita en 30 h ⇒ 48 h OMITIDO; sin autorización ⇒ todo OMITIDO; cancelación anula pendientes; envío marcado SIMULADO con id `SIM-`; mensaje sin datos clínicos | ✅ `TestPlanificarRecordatorios`, `TestPlanificarOmiteLoQueNoCabe`, `TestPlantillaSinDatosClinicos`, `TestRecordatoriosYProveedorSimulado` · proveedor real ⏳ (credenciales) |
| RF-12 Caja y facturación | — | 5 | Pago mixto suma exacta (decimal); comprobante vía adaptador simulado | 🟡 · integración real ⏳ (Q-01: país) |
| RF-13 Honorarios | — | 6 | Tres tipos de regla con casos numéricos | 🟡 |
| RF-14 Descargo de insumos | — | 6 | Descargo en la misma transacción; si falla, nada se descuenta | 🟡 |
| RN-01 Inmutabilidad clínica | — | 3 | `DELETE` de procedimiento realizado rechazado por API **y** BD; corrección por adenda | 🟡 · pacientes y citas ya no se borran ✅ `TestReglasEnBaseDeDatos`, `TestRestriccionEnLaBaseDeDatos` |
| RN-02 Cierre ciego | — | 5 | La respuesta al recepcionista no contiene el teórico | 🟡 · `caja.revisar` sólo Auditor ✅ `TestMenorPrivilegio` |
| RN-03 Sin superposición por sillón | `EXCLUDE USING gist` + API | 2 | Solape rechazado aunque cambie el odontólogo (409 con la cita en conflicto); consecutivas y otro sillón permitidos; cancelada libera; **20 peticiones simultáneas ⇒ 1 éxito y 19 rechazos**; `INSERT` directo en la BD también falla | ✅ `TestSuperposicionPorSillon`, `TestCanceladaLiberaSillon`, `TestVeinteSolicitudesSimultaneas`, `TestRestriccionEnLaBaseDeDatos` |
| RNF-01 < 1,5 s | — | 7 | Medición con datos y hardware definidos | ⏳ |
| RNF-02 99,9 % | infraestructura | 7 | Monitoreo externo de `/api/health` | ⏳ |
| RNF-03 150 concurrentes | — | 7 | Prueba de carga (k6) con 150 usuarios virtuales, percentiles p95 | ⏳ |
| RNF-04 TLS 1.3 / AES-256 | infraestructura | 7 | Escaneo TLS del dominio; evidencia de cifrado del proveedor | ⏳ |
| RNF-05 RBAC | `routes`, `middleware`, `permisos` | 1 | Toda ruta con política; matriz rol × endpoint; campos extra ⇒ 400; token adulterado ⇒ 401 | ✅ `TestTodasLasRutasTienenPolitica`, `TestMatrizDePermisos`, `TestValidaEntradaYRechazaCamposExtra`, `TestSinTokenYTokenAdulterado`, `TestMenorPrivilegio` |
| RNF-05 MFA | `seguridad/totp.go` | 1 | Sin MFA ⇒ 403 `MFA_REQUERIDO`; vectores RFC 6238; anti-replay; rotación del token; primer ingreso (contraseña + enrolamiento con QR) | ✅ `TestTOTPVectoresRFC6238`, `TestTOTPVentanaYAntiReplay`, `TestMFAObligatorioYRotacionDeToken`, `TestCodigoTOTPNoSeReutiliza`, `TestPrimerIngreso` |
| RNF-05 Inactividad 15 min | `middleware/auth.go` + `useInactividad` | 1 | 16 min ⇒ 401 `SESION_EXPIRADA` y auditoría; 14 min ⇒ sigue activa; parámetro 5–15 | ✅ `TestCierrePorInactividad`, `TestAjustes` |
| RNF-05 Bloqueo por intentos | `usuarios` | 1 | 5 fallos ⇒ bloqueo 15 min; mismo mensaje con correo inexistente | ✅ `TestBloqueoTrasCincoIntentos`, `TestMismoErrorSinEnumeracion` |
| RNF-06 Auditoría | `repository/auditoria.go` + triggers | 1 (base) · 2 (pacientes) | ISO-8601 + IP; `UPDATE`/`DELETE`/`TRUNCATE` rechazados; alteración detectada por la cadena; búsqueda, ficha y anamnesis auditadas | ✅ `TestAuditoriaInmutable`, `TestHashAuditoriaDeterministaYSensible`, `TestLecturasAuditadasYPrivilegios` · notas y odontograma 🟡 Etapa 3 |
| RNF-07 Responsive | web | 1+ | Escritorio 1360×900 y celular 390×844 | ✅ capturas del recorrido · tablet y 1920×1080 🟡 automatizar en Etapa 7 |
| RNF-08 Navegadores | web | 7 | Playwright en Chromium, Firefox y WebKit | 🟡 (hoy sólo Chromium) |
| 8.2 Cobertura ≥ 80 % | api | todas | `go test -coverpkg` | ⏳ medido 76,3 % |
| 8.2 DAST/SAST | CI | 7 | OWASP ZAP + `gosec`/`govulncheck` + `npm audit` | ⏳ |
| 8.2 OpenAPI 3 | api | todas | `go run . openapi` ⇒ `docs/openapi.json` (40 rutas, generado del registro real) | ✅ `TestOpenAPIGenerado` |

# 5. Trazabilidad y pruebas

Estados:

- ✅ **Ejecutada**: la prueba existe en el repositorio y pasa.
- 🟡 **Propuesta**: diseñada; se implementa en la etapa indicada.
- ⏳ **Validación pendiente**: requiere medición, infraestructura o una decisión del cliente. **No** se da por cumplida desde el código.

## 5.1 Cómo se ejecutan

| Suite | Comando | Qué necesita | Resultado (30/10/2026, Etapa 3) |
|---|---|---|---|
| Unitarias Go (`domain`, `seguridad`, `permisos`, `notificaciones`, `repository`) | `cd sigo-backend && go test ./domain/... ./seguridad/... ./permisos/... ./notificaciones/... ./repository/...` | Nada | ✅ 22 pruebas |
| Integración Go (`pruebas/`) contra la API y una base real | `docker compose up -d` y luego `go test ./pruebas/` (usa `.env.test` → base `sigo_dental_test`, que reinicia en cada corrida, y la base 1 de Redis para no mezclar su caché con la de la aplicación) | PostgreSQL | ✅ 49 pruebas |
| Cobertura de la capa de endpoints | `go test -coverpkg=./controllers/...,./middleware/...,./repository/...,./routes/... -coverprofile=c.out ./...` y `go tool cover -func=c.out` | PostgreSQL | ✅ **81,0 %** |
| Cobertura de todo el backend | `go test -coverpkg=./... -coverprofile=c.out ./...` | PostgreSQL | 79,9 % (incluye `main.go`, comandos de consola sin pruebas: 0 %) |
| Unitarias del frontend (Vitest + Testing Library) | `npm test` | Nada | ✅ 17 pruebas |
| Lint del frontend | `npm run lint` | Nada | ✅ sin errores |
| Prueba de humo por comandos | `powershell -ExecutionPolicy Bypass -File scripts\prueba-api.ps1` (dentro de `sigo-backend`) | API corriendo | ✅ "TODO OK" |
| Recorrido en navegador (Playwright) | Script de verificación del desarrollo; capturas en `docs/capturas/` | API + `npm run preview` | ✅ Etapas 1–2: 21 pasos con 5 roles · Etapa 3: 10 pasos (odontograma, periodontograma, nota firmada con MFA, adenda, procedimiento, paciente sin anamnesis, asistente, recepción, celular) |

> **Cobertura (SRS 8.2: ≥ 80 % en endpoints críticos):** medida por paquete, sin redondear a favor: controladores 80,3 %, middleware 96,4 %, repositorio 81,1 %, rutas 81,9 %, dominio 99,3 %, seguridad 96,5 %. La capa de endpoints en conjunto: 81,0 %. Lo menos cubierto: comandos de consola de `main.go`, ramas de error de infraestructura (caída de la BD a mitad de una transacción) y el despachador en segundo plano (se prueba su lógica, no el temporizador).

## 5.2 Matriz requisito → prueba

| Requisito | Dónde (API / Web) | Etapa | Prueba | Estado |
|---|---|---|---|---|
| RF-01 Ficha única | `pacienteController` / Pacientes, ficha, formulario | 2 | Alta adulto; **menor sin tutor ⇒ 400 `TUTOR_REQUERIDO`**; no se quita el tutor a un menor; documento duplicado ⇒ 409; fecha futura y teléfono inválido ⇒ 400; edición auditada con valor anterior y nuevo | ✅ `TestRegistroPacienteAdulto`, `TestMenorExigeTutor`, `TestDocumentoUnicoYValidaciones`, `TestModificacionAuditaValores` |
| RF-02 Anamnesis y alertas | `pacienteController`, `domain/alertas.go` / cabecera roja | 2 | Penicilina y anticoagulado ⇒ CRÍTICA; versionado; las alertas usan la versión vigente; la BD rechaza `UPDATE`/`DELETE`; recepción ve alertas pero no la anamnesis (403) | ✅ `TestAlertasPenicilinaYAnticoagulado`, `TestAlertaPenicilinaCritica`, `TestAnamnesisVersionada`, `TestReglasEnBaseDeDatos`, Vitest `AlertasMedicas` · registros clínicos bloqueados sin anamnesis ✅ `TestAnamnesisPendienteBloqueaRegistrosClinicos` |
| RF-03 Odontograma | `clinicaController`, `domain/dental.go` / pestaña Odontograma (SVG) | 3 | 32/20 dientes; **tabla FDI↔Universal completa (52)**; superficies con I/P según el diente; catálogo con reglas (implante sólo permanentes, sellante sólo oclusal); agregar/quitar como eventos con historial; la BD rechaza borrar | ✅ `TestFDIUniversalTablaCompleta`, `TestSuperficiesYEtiquetas`, `TestValidarMarca`, `TestEstadoActualConservaHistorial`, `TestOdontogramaEventosEHistorial`, Vitest `dental.test.js` |
| RF-04 Periodontograma | `clinicaController`, `domain/perio.go`, columna generada `nic` | 3 | 6 sitios; NIC = PS + MG con los ejemplos de 03-modelo-datos §3.5 (5−2=3, 4+2=6) calculado **por la BD**; resumen (sangrado, bolsas ≥ 4 mm, NIC máx.); fuera de rango, 5 sitios, diente temporal o movilidad 4 ⇒ 400; sólo inserción | ✅ `TestNICYResumen`, `TestPeriodontogramaNIC` |
| RF-05 Notas SOEP | `clinicaController`, trigger `sigo_nota_soep_guardia` / Evolución (SOEP) | 3 | Borrador editable sólo por su autor; **bloqueo a las 24 h rechazado por la API (409) y por la BD**; plazo configurable (2 h probado); firma con código MFA (inválido ⇒ 400 y auditoría `FIRMA_FALLIDA`), hash SHA-256 y registro profesional; firmada ⇒ sin cambios (API y BD); adendas firmadas; alteración directa en la BD detectada por el hash | ✅ `TestNotasSOEPFirmaYAdendas`, `TestBloqueoDeNotasPorPlazo`, `TestNotaVinculadaACita` |
| RF-06 Consentimiento | — | 4 | Documento con hash; firma dibujada u OTP; no editable tras firmar | 🟡 |
| RF-07 Presupuestos | — | 4 | Totales por fase con honorarios, laboratorio y descuento; descuento sobre el tope requiere autorización | 🟡 |
| RF-08 Aprobación parcial | — | 4 | Aprobar 2 de 3 ítems crea exactamente 2 tareas agendables | 🟡 |
| RF-09 Agenda | `citaController` / Agenda día y semana | 2 | Filtros por sillón y odontólogo; rango máx. 31 días; validaciones (pasado, duración, no odontólogo); permisos | ✅ `TestFiltrosAgenda`, `TestValidacionesYPermisosAgenda` · vistas día/semana: recorrido en navegador |
| RF-10 Estados | `domain/citas.go` / Detalle de la cita | 2 | Flujo completo con historial (5 eventos); transición inválida ⇒ 400; cancelar exige motivo; "No asistió" sólo desde la hora; reprogramar rehace recordatorios | ✅ `TestTransicionesCita`, `TestFlujoDeEstados`, `TestReprogramar`, Vitest máquina de estados |
| RF-11 Recordatorios | `notificaciones` / Bandeja | 2 | Confirmación, −48 h y −24 h; cita en 30 h ⇒ 48 h OMITIDO; sin autorización ⇒ todo OMITIDO; cancelación anula pendientes; envío marcado SIMULADO con id `SIM-`; mensaje sin datos clínicos | ✅ `TestPlanificarRecordatorios`, `TestPlanificarOmiteLoQueNoCabe`, `TestPlantillaSinDatosClinicos`, `TestRecordatoriosYProveedorSimulado` · proveedor real ⏳ (credenciales) |
| RF-12 Caja y facturación | — | 5 | Pago mixto suma exacta (decimal); comprobante vía adaptador simulado | 🟡 · integración real ⏳ (Q-01: país) |
| RF-13 Honorarios | — | 6 | Tres tipos de regla con casos numéricos | 🟡 |
| RF-14 Descargo de insumos | — | 6 | Descargo en la misma transacción; si falla, nada se descuenta | 🟡 |
| RN-01 Inmutabilidad clínica | `procedimientos_realizados` + trigger | 3 | Registro firmado con MFA; **`UPDATE` y `DELETE` rechazados por la BD** (la API no tiene rutas para hacerlo); corrección por adenda firmada que no altera el original; el asistente no registra (403) | ✅ `TestProcedimientosRealizadosInmutables`; pacientes y citas sin borrado ✅ `TestReglasEnBaseDeDatos`, `TestRestriccionEnLaBaseDeDatos` |
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
| RNF-06 Auditoría | `repository/auditoria.go` + triggers | 1 (base) · 2 (pacientes) | ISO-8601 + IP; `UPDATE`/`DELETE`/`TRUNCATE` rechazados; alteración detectada por la cadena; búsqueda, ficha, anamnesis, odontograma, periodontograma, notas y procedimientos auditados (lecturas y escrituras); filtros por acción, fecha y usuario | ✅ `TestAuditoriaInmutable`, `TestHashAuditoriaDeterministaYSensible`, `TestLecturasAuditadasYPrivilegios`, `TestOdontogramaEventosEHistorial`, `TestFiltrosDeAuditoria` |
| RNF-07 Responsive | web | 1+ | Escritorio 1360×900 y celular 390×844 | ✅ capturas del recorrido · tablet y 1920×1080 🟡 automatizar en Etapa 7 |
| RNF-08 Navegadores | web | 7 | Playwright en Chromium, Firefox y WebKit | 🟡 (hoy sólo Chromium) |
| 8.2 Cobertura ≥ 80 % | api | todas | `go test -coverpkg` sobre la capa de endpoints | ✅ 81,0 % medido (total del backend 79,9 %) |
| 8.2 DAST/SAST | CI | 7 | OWASP ZAP + `gosec`/`govulncheck` + `npm audit` | ⏳ |
| 8.2 OpenAPI 3 | api | todas | `go run . openapi` ⇒ `docs/openapi.json` (52 rutas, generado del registro real) | ✅ `TestOpenAPIGenerado` |

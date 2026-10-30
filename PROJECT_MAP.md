# PROJECT MAP — SIGO-Dental (sigo-dental)

> Mapa arquitectónico del repositorio. Leer ESTE archivo antes de explorar código.
> Sistema de gestión odontológica construido desde el SRS v1.0. Mismo stack y
> organización que catalogo-multinivel (Framework Programación Web, UPSE 2026-02).

## Visión general

```
React SPA / PWA (:5173) ──apiFetch + Bearer JWT──► Go/Fiber API (:3000) ──pgx──► PostgreSQL 16 (:5434 Docker)
                                                          │
                                                          ├──go-redis──► Redis 7 (:6380 Docker) — caché de catálogos y ajustes
                                                          └──goroutine──► recordatorios (proveedor SIMULADO)
```

## Documentos raíz (meta, no código)

| Archivo | Contenido |
|---|---|
| `PROJECT_MAP.md` | Este archivo. Índice de navegación. |
| `REQUIREMENTS_CHECKLIST.md` | Estado de cada RF/RNF/RN del SRS y de cada etapa. Fuente de verdad del progreso. |
| `PLAN_IMPLEMENTACION.md` | Plan por etapas (1–7) con tareas checkeables y criterios de aceptación. |
| `README.md` | Instalar, migrar, ejecutar y probar (Windows). |
| `.windsurfrules` | Reglas para el agente IA (navegación, seguimiento, convenciones). |
| `docs/01…05` | Análisis de requisitos, arquitectura, modelo de datos, matriz de permisos, trazabilidad y pruebas. |
| `docs/openapi.json` | OpenAPI 3 generado con `go run . openapi`. |
| `docs/capturas/` | Pantallas de referencia. |

## Frontend (raíz del repo) — SPA

Stack: React 19 + **JavaScript** + Vite 8 + Tailwind CSS 4 + React Router 7 + PWA (`vite-plugin-pwa`) + `qrcode` + `lucide-react`.
Comandos: `npm run dev` (:5173) | `npm run build` | `npm run preview` (:4173) | `npm test` (Vitest) | `npm run lint` | `npm run generar-iconos`.
Config: `.env` opcional (ver `.env.example`): `VITE_API_URL` vacía = mismo origen; Vite reenvía `/api` a `VITE_PROXY_TARGET`.

| Ruta (`src/`) | Responsabilidad |
|---|---|
| `main.jsx` | Monta React con `BrowserRouter` + `AuthProvider`; importa `pwa/instalacion.js` antes. |
| `App.jsx` | Máquina de pasos del ingreso (`siguientePaso`) y rutas por permiso; módulos de etapas futuras → `ModuloPendiente`. |
| `config.js` | `API_URL` desde `VITE_API_URL`. |
| `index.css` | Tema Tailwind (`brand`, `ink`, `muted`, `line`, `canvas`, `danger`, `warn`, `ok`). |
| `services/http.js` | `apiFetch` único: Bearer desde `sessionStorage`, `ApiError{status, code, message, extra, campos}`, 401 → evento `sigo:sesion-perdida`, aviso sin conexión. |
| `services/authService.js` | `/login`, `/me`, `/mfa/*`, `/password`, `/logout`, `/heartbeat`. |
| `services/sistemaService.js` | Panel, usuarios, ajustes, auditoría. |
| `services/catalogosService.js` | Estructura, sillones, procedimientos, convenios, odontólogos; alta/edición. |
| `services/pacientesService.js` | Búsqueda, ficha, alta, edición, anamnesis. |
| `services/citasService.js` | Citas, reprogramar, estado, bandeja de mensajes. |
| `services/expedienteService.js` | Odontograma, periodontogramas, notas SOEP (firma, adendas), procedimientos realizados. |
| `context/AuthContext.jsx` (+ `AuthContextValue.js`, `useAuth.jsx`) | Perfil, permisos, `puede(...)`, `ingresar`, `aplicarSesion` (tokens rotados por MFA), `salir`. |
| `hooks/useApi.js` | Lectura `{data, isLoading, error, recargar}` (igual al del catálogo). |
| `hooks/useAccion.js` | Escritura `{ejecutar, enviando, error}`. |
| `hooks/useInactividad.js` | Aviso 60 s antes del cierre por inactividad + heartbeat. |
| `hooks/useNotacion.js` | Preferencia FDI / Universal del usuario (localStorage; no es dato del paciente). |
| `hooks/useOnline.js`, `hooks/useInstalarPWA.js`, `pwa/instalacion.js` | Conexión e instalación de la PWA. |
| `utils/fechas.js` | Fechas locales, edad, semana desde el lunes, formatos `es`. |
| `utils/etiquetas.js` | Etiquetas de roles, acciones de auditoría, estados de cita y transiciones (copia de `domain/citas.go`). |
| `utils/cx.js` | Une clases CSS. |
| `utils/dental.js` | Dientes por arcada, FDI → Universal, superficies (O/I, L/P), sitios del periodontograma, NIC (copia de `domain/dental.go`). |
| `components/ui.jsx` | Botones, campos, modal, pestañas, alertas, estados vacíos. |
| `components/AlertasMedicas.jsx` | Banner rojo de alerta crítica + indicador compacto (RF-02). |
| `components/Layout.jsx` | Menú lateral / cajón móvil, cabecera, modal "¿Sigue ahí?". |
| `components/modulos.js` | Menú: ruta, permisos, etapa, requisitos. |
| `components/AvisoActualizacion.jsx`, `BannerSinConexion.jsx`, `InstalarApp.jsx` | PWA. |
| `pages/auth/PasosIngreso.jsx` | Login, verificar MFA, cambiar contraseña, configurar MFA con QR. |
| `pages/Panel.jsx` | Widgets por rol (`GET /api/panel`). |
| `pages/admin/Usuarios.jsx`, `Ajustes.jsx`, `Auditoria.jsx`, `Catalogos.jsx` | Administración. |
| `pages/pacientes/Pacientes.jsx`, `FichaPaciente.jsx`, `FormularioPaciente.jsx`, `PanelAnamnesis.jsx` | RF-01, RF-02. |
| `pages/agenda/Agenda.jsx`, `NuevaCita.jsx`, `DetalleCita.jsx`, `BandejaMensajes.jsx`, `FilaNotificacion.jsx`, `comun.js` | RF-09, RF-10, RF-11, RN-03. |
| `pages/expediente/Expediente.jsx` | Entrada del menú: buscar paciente y abrir su expediente. |
| `pages/expediente/Odontograma.jsx` | RF-03: SVG interactivo, dentición permanente/temporal, FDI/Universal, historial. |
| `pages/expediente/Periodontograma.jsx` | RF-04: captura por arcada, NIC en vivo, exámenes anteriores con resumen. |
| `pages/expediente/NotasSOEP.jsx`, `ModalFirma.jsx`, `Adendas.jsx` | RF-05: borradores, bloqueo, firma con código MFA, verificación del hash, adendas. |
| `pages/expediente/Procedimientos.jsx` | RN-01: registro firmado e inmutable, adendas. |
| `pages/ModuloPendiente.jsx` | Pantalla honesta de módulos de etapas futuras. |
| `pruebas/` | Vitest + Testing Library. |

## Backend (`sigo-backend/`) — Go 1.24 + Fiber v2

Comandos: `go run .` (API) | `go run . migrar` | `go run . semilla [--reset-usuarios]` | `go run . totp <correo>` | `go run . openapi` | `go test ./...`.
Config: `.env` (copiar de `.env.example`); pruebas: `.env.test` (base `sigo_dental_test`). Docker: `docker compose up -d`.

| Ruta | Responsabilidad |
|---|---|
| `main.go` | DI, servidor con apagado ordenado, goroutine de recordatorios, comandos de consola. |
| `config/` | `Config` desde `.env`; `ConectarDB` (pgxpool); `ConectarRedis` (nil si no responde). |
| `routes/routes.go` | Todas las rutas con **política obligatoria**; helmet, CORS, límite de logins, errores JSON. |
| `routes/openapi.go` | `Docs` (resumen de cada ruta) y generador OpenAPI 3. |
| `routes/frontend.go` | Sirve la build del frontend si `STATIC_DIR` está definido (despliegue en un solo servidor). |
| `middleware/auth.go` | `RequireAuth`, `RequireCompleta`, `RequirePermisos`, `RequireAlguno`, `Actual(c)`. |
| `controllers/controllers.go` | `Controllers{Repo, Cache, Cfg, Despachador, Ahora}`, `leerJSON` estricto, errores, auditoría, ajustes en caché. |
| `controllers/authController.go` | Login, MFA, contraseña, logout, me, heartbeat. |
| `controllers/usuarioController.go`, `sistemaController.go` | Usuarios, ajustes, auditoría, health, panel. |
| `controllers/catalogoController.go` | Catálogos con caché Redis e invalidación. |
| `controllers/pacienteController.go` | Ficha, tutor, anamnesis (lecturas auditadas). |
| `controllers/citaController.go` | Citas, estados, reprogramación, bandeja, despacho. |
| `controllers/clinicaController.go` | Expediente: odontograma, periodontograma, notas SOEP (bloqueo, firma, adendas), procedimientos realizados; re-autenticación MFA y hashes. |
| `repository/` | `DBRepository` (interfaz) + `Postgres`: usuarios/sesiones, auditoría encadenada, ajustes, catálogos, pacientes, citas, notificaciones, expediente clínico (`clinica.go`). |
| `cache/cache.go` | `CacheRepository` + `RedisCache` (nil-safe). |
| `domain/` | Reglas puras: alertas, estados de cita, edad y día local, numeración dental, superficies, hallazgos, estado del odontograma, NIC. |
| `notificaciones/` | Planificador, mensajería (`Simulada`) y despachador. |
| `seguridad/` | bcrypt, JWT, TOTP (RFC 6238), tokens. |
| `permisos/` | Catálogo de permisos y matriz por rol. |
| `validacion/` | Validadores por campo. |
| `models/` | `APIError`, `Pagina[T]`, entidades. |
| `database/*.sql` + `migrar.go` | Migraciones embebidas e idempotentes; `initdb/` crea la base de pruebas. |
| `semilla/semilla.go` | Datos ficticios de demostración. |
| `pruebas/` | Integración contra la API y una base real (auth, permisos, pacientes, agenda, expediente clínico, complementos, OpenAPI). |
| `scripts/prueba-api.ps1` | Prueba de humo en PowerShell. |

## Rutas de la API (todas bajo `/api`)

| Grupo | Rutas | Política |
|---|---|---|
| Público | `GET /health`, `POST /login` | pública (login: 10/min por IP) |
| Pasos del ingreso | `POST /mfa/verificar`, `/mfa/configurar`, `/mfa/activar`, `/password`, `/logout`, `GET /me` | token válido |
| Sesión completa | `POST /heartbeat`, `GET /panel` | MFA verificado |
| Usuarios | `GET/POST /usuarios`, `GET /usuarios/roles`, `PATCH /usuarios/:id`, `POST /usuarios/:id/restablecer-password`, `/restablecer-mfa` | `usuarios.gestionar` |
| Ajustes | `GET /ajustes`, `PUT /ajustes/:clave` | `ajustes.gestionar` |
| Auditoría | `GET /auditoria`, `POST /auditoria/verificar` | `auditoria.leer` |
| Catálogos | `GET /catalogos/estructura`, `/sillones`, `/procedimientos`, `/convenios`, `/odontologos`; `POST /catalogos/:tipo`; `PATCH /catalogos/:tipo/:id` | lectura: varios roles · escritura: `catalogos.gestionar` |
| Pacientes | `GET/POST /pacientes`, `GET/PUT /pacientes/:id`, `GET/POST /pacientes/:id/anamnesis` | `pacientes.*`, `anamnesis.escribir`, `clinica.leer` |
| Agenda | `GET/POST /citas`, `GET /citas/:id`, `PATCH /citas/:id/reprogramar`, `POST /citas/:id/estado`, `GET /notificaciones`, `POST /notificaciones/procesar` | `citas.leer` / `citas.escribir` |
| Expediente | `GET/POST /pacientes/:id/odontograma`, `GET/POST /pacientes/:id/periodontogramas`, `GET/POST /pacientes/:id/notas`, `PUT /notas/:id`, `POST /notas/:id/firmar`, `POST /notas/:id/adendas`, `GET/POST /pacientes/:id/procedimientos`, `POST /procedimientos-realizados/:id/adendas` | leer: `clinica.leer` · registrar: `clinica.escribir` · firmar: `clinica.firmar` |

Detalle de cada una (cuerpos y respuestas): `docs/openapi.json`.

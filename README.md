# SIGO-Dental — Sistema Integral de Gestión Odontológica

Gestión de una clínica dental: ficha única del paciente con anamnesis y alertas médicas, agenda por sillón y profesional con recordatorios, y (en etapas siguientes) expediente clínico, presupuestos, caja, inventario y honorarios. Construido a partir del SRS v1.0 (29/10/2026).

Mismo stack y forma de trabajo que **catalogo-multinivel**: frontend en React + Vite + Tailwind (**JavaScript**, PWA) en la raíz, backend en **Go + Fiber** en `sigo-backend/`, PostgreSQL 16, Redis, JWT con bcrypt.

> **Todos los datos son ficticios** (usuarios `@demo.sigo`, documentos `DEMO…`, teléfonos `+000…`). La mensajería y la facturación usan proveedores **SIMULADOS**: no se envía ningún WhatsApp, SMS ni correo real.

## Estado

| Etapa | Contenido | Estado |
|---|---|---|
| 1. Base | Login + MFA TOTP, sesiones con cierre por inactividad, roles y permisos, auditoría encadenada, usuarios, parámetros, panel por rol | ✅ |
| 2. Pacientes y agenda | Catálogos, ficha única con tutor, anamnesis versionada, alertas críticas, agenda día/semana por sillón, 7 estados, recordatorios 48 h/24 h (simulados), sin superposición por sillón | ✅ |
| 3–7 | Núcleo clínico, presupuestos y consentimientos, caja, inventario y honorarios, calidad y despliegue | Ver `PLAN_IMPLEMENTACION.md` |

El detalle requisito por requisito está en `REQUIREMENTS_CHECKLIST.md`.

## Requisitos

- Node.js 20+ y npm
- Go 1.24+ (probado también con versiones más nuevas)
- Docker Desktop (para PostgreSQL y Redis). Sin Docker: PostgreSQL 16 + Redis/Memurai instalados, ajustando el `.env`.

## Configuración inicial (una sola vez)

En PowerShell, dentro de la carpeta del proyecto:

```powershell
# 1. Base de datos y Redis (PostgreSQL queda en el puerto 5434 y Redis en el 6380)
cd sigo-backend
docker compose up -d

# 2. Configuración del backend
copy .env.example .env          # luego cambie JWT_SECRET por uno propio (mínimo 32 caracteres)

# 3. Dependencias de Go, tablas y datos de demostración
go mod tidy
go run . migrar
go run . semilla

# 4. Dependencias del frontend
cd ..
npm install
```

> ¿Por qué 5434 y 6380? En el mismo equipo suele haber un PostgreSQL instalado en Windows (5432), la versión anterior de este proyecto (5433) y el Redis de catalogo-multinivel (6379). Con puertos propios conviven sin chocar.

## Cómo ejecutar

**Backend (puerto 3000)**, con Docker corriendo:

```powershell
cd sigo-backend
go run .
```

**Frontend (puerto 5173)**, en otra terminal desde la raíz:

```powershell
npm run dev
```

Abrir http://localhost:5173

### Usuarios de demostración

Contraseña de todos: `Demo-Sigo-2026`

| Rol | Correo |
|---|---|
| Administrador TI | `admin@demo.sigo` |
| Recepción / Caja | `recepcion@demo.sigo` |
| Odontólogo | `odontologo@demo.sigo` y `odontologo2@demo.sigo` |
| Asistente dental | `asistente@demo.sigo` |
| Auditor / Contador | `auditor@demo.sigo` |
| Primer ingreso (cambia contraseña y configura MFA) | `nuevo@demo.sigo` |

**Código MFA.** Todos los usuarios (salvo `nuevo`) ya tienen MFA. El código de 6 dígitos se obtiene con:

```powershell
cd sigo-backend
go run . totp recepcion@demo.sigo
```

(sólo funciona fuera de producción). También puede escanear el QR con Google/Microsoft Authenticator al ingresar con `nuevo@demo.sigo`. Cada código se acepta **una sola vez**: si acaba de usarlo, espere el siguiente (30 s).

Para volver a dejar los usuarios como al inicio: `go run . semilla --reset-usuarios`.

## Pruebas

```powershell
cd sigo-backend
go test ./...                                   # unitarias + integración (necesita Docker: usa la base sigo_dental_test)
powershell -ExecutionPolicy Bypass -File scripts\prueba-api.ps1   # prueba de humo con la API corriendo

cd ..
npm test                                        # pruebas del frontend (Vitest)
npm run lint
```

Resultados y cobertura medidos: `docs/05-trazabilidad-y-pruebas.md`.

## Documentación de la API

`docs/openapi.json` (OpenAPI 3) se genera del registro real de rutas, así que no se desactualiza:

```powershell
cd sigo-backend
go run . openapi
```

Puede abrirse en https://editor.swagger.io (File → Import file).

## App instalable (PWA)

```powershell
npm run build
npm run preview      # http://localhost:4173 → menú lateral → Instalar app
```

Por ser datos de salud, el service worker **sólo guarda la aplicación** (HTML, JS, CSS, íconos), nunca respuestas de la API. La sesión vive en `sessionStorage`: al cerrar la pestaña, se cierra.

## Documentos

| Archivo | Contenido |
|---|---|
| `PROJECT_MAP.md` | Mapa del repositorio: qué hay en cada carpeta y archivo. |
| `REQUIREMENTS_CHECKLIST.md` | Estado de cada requisito del SRS (RF, RNF, RN) y de cada etapa. |
| `PLAN_IMPLEMENTACION.md` | Plan por etapas con criterios de aceptación. |
| `docs/01-analisis-requisitos.md` | Inventario, ambigüedades, supuestos y preguntas para el cliente. |
| `docs/02-arquitectura.md` | Decisiones técnicas y qué depende del código o de la infraestructura. |
| `docs/03-modelo-datos.md` | Tablas, restricciones y reglas en la base. |
| `docs/04-matriz-permisos.md` | Rol × permiso. |
| `docs/05-trazabilidad-y-pruebas.md` | Requisito → prueba → estado, con los resultados reales. |
| `docs/capturas/` | Pantallas de las Etapas 1 y 2. |

## Lo que este proyecto **no** afirma

- **Firmas:** las notas y consentimientos (Etapas 3–4) usarán firma **electrónica simple** (re-autenticación + hash). No es una firma digital con certificado ni una firma biométrica.
- **País y facturación:** no se asume ningún país ni autoridad tributaria; la facturación será un adaptador simulado hasta que el cliente lo defina.
- **RNF de rendimiento, disponibilidad y cifrado (TLS 1.3, AES-256):** dependen de la infraestructura y de mediciones que aún no se hicieron. Están marcados como pendientes, no como cumplidos.

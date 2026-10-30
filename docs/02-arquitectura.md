# 2. Arquitectura

Mismo stack y organización que el proyecto del curso **catalogo-multinivel**: frontend React (JavaScript) en la raíz del repositorio y backend Go/Fiber en una subcarpeta, PostgreSQL y Redis en Docker, JWT con bcrypt, PWA.

```
Navegador (React 19 + Vite, PWA) ──fetch /api + Bearer JWT──► Go 1.24 / Fiber v2 (:3000)
   :5173 en desarrollo (Vite reenvía /api)                       │
                                                                  ├── pgx/v5 ──► PostgreSQL 16 (:5434 en Docker)
                                                                  ├── go-redis ─► Redis 7 (:6380 en Docker) — caché
                                                                  └── goroutine de recordatorios ──► proveedor de mensajería
                                                                                                     (SIMULADO en esta etapa)
```

## 2.1 Decisiones

| Decisión | Alternativa descartada | Motivo |
|---|---|---|
| **Monolito modular** en Go (`routes → controllers → repository`, con `domain` puro) | Microservicios | Un equipo pequeño y una sola base; el SRS dice "Microservicio / Modular". Los límites por paquete permiten extraer servicios si alguna vez hiciera falta. |
| **JWT HS256 + tabla `sesiones`** | JWT solo / cookie de sesión | Se conserva el JWT Bearer del curso, pero cada token tiene un `jti` que apunta a una sesión de servidor. Eso permite revocar al instante (logout, desactivar usuario, reset), exigir MFA y cerrar por **inactividad de 15 min** (RNF-05), cosas que un JWT autocontenido no puede hacer. |
| **MFA TOTP propio** (RFC 6238, `seguridad/totp.go`) | Librería externa | ~60 líneas con vectores oficiales del RFC en las pruebas; anti-replay guardando el último paso usado. El token se **rota** al verificar MFA. |
| **bcrypt (costo 12)** | Argon2id | Es el del curso (x/crypto). Política: ≥ 10 caracteres, letra y número, sin el usuario del correo. |
| **Reglas críticas en PostgreSQL** | Sólo validación en la API | RN-03 con `EXCLUDE USING gist`; triggers de sólo inserción para auditoría, anamnesis e historial de citas; sin borrado de pacientes y citas. Resisten concurrencia y accesos directos a la BD. |
| **Redis Cache-Aside** (catálogos 10 min, ajustes 1 min, cabecera `X-Cache`) | Sin caché | Patrón del curso. Si Redis cae, la API sigue (`/api/health` = `degraded`). Nunca se guardan datos de pacientes en Redis. |
| **Proveedores intercambiables** (`notificaciones.Mensajeria`) | Llamar directo a WhatsApp/Twilio | Sin credenciales reales en el proyecto. El proveedor `Simulada` marca cada mensaje `simulado=true` e id `SIM-…`; la interfaz lo muestra como **SIMULADO · no se envió**. |
| **PWA sin caché de API** | Cachear lecturas como en el catálogo | Son datos de salud: el service worker sólo guarda el *app shell*. El token vive en `sessionStorage` (se borra al cerrar la pestaña). |

## 2.2 Backend (`sigo-backend/`)

| Paquete | Responsabilidad |
|---|---|
| `main.go` | Inyección de dependencias, servidor con apagado ordenado, goroutine de recordatorios (`WaitGroup`) y comandos `migrar`, `semilla`, `totp`, `openapi`. |
| `config` | `.env` → `Config`; conexión pgxpool; Redis opcional. |
| `routes` | Registro de rutas con **política obligatoria** (`publica`, `sesion`, `completa`, `permisos:…`, `alguno:…`), helmet, CORS, límite de 10 logins/min, manejador de errores JSON y generador **OpenAPI**. |
| `middleware` | `RequireAuth` (Bearer → JWT → sesión → inactividad), `RequireCompleta` (MFA y contraseña definitiva), `RequirePermisos`, `RequireAlguno`. |
| `controllers` | HTTP: lectura estricta del JSON (campos desconocidos ⇒ 400), validación, auditoría y respuesta. Errores `models.APIError{status, code, message, error, extra}`. |
| `repository` | SQL con pgx detrás de la interfaz `DBRepository`; traduce códigos de PostgreSQL (23505 ⇒ duplicado, 23P01 ⇒ superposición). |
| `domain` | Reglas puras sin E/S: alertas médicas, máquina de estados de la cita, edad, día local. 100 % cubierto por pruebas. |
| `notificaciones` | Planificador (confirmación, −48 h, −24 h; OMITIDO si ya pasó o sin autorización), plantillas sin datos clínicos, despachador. |
| `seguridad` | bcrypt, JWT, TOTP, tokens aleatorios. |
| `permisos` | Catálogo de permisos y matriz por rol (ver 04-matriz-permisos). |
| `validacion` | Validadores reutilizables que acumulan errores por campo (`extra.campos`). |
| `database` | Migraciones SQL embebidas e idempotentes. |
| `semilla` | Datos de demostración **ficticios** (usuarios `@demo.sigo`, documentos `DEMO…`, teléfonos `+000…`). |
| `pruebas` | Pruebas de integración contra una base real (`sigo_dental_test`). |

### Flujo de una petición protegida

```
Authorization: Bearer <jwt>
  → RequireAuth: firma y expiración del JWT → sesión por jti (no revocada, < 12 h)
                 → inactividad > minutosInactividad ⇒ 401 SESION_EXPIRADA (+ auditoría)
                 → actualiza ultima_actividad (máx. cada 15 s)
  → RequireCompleta: sin MFA ⇒ 403 MFA_REQUERIDO; clave temporal ⇒ 403 CAMBIO_PASSWORD_REQUERIDO
  → RequirePermisos("citas.escribir") ⇒ 403 PROHIBIDO si el rol no lo tiene
  → controlador → repository → PostgreSQL (EXCLUDE / triggers) → auditoría encadenada
```

## 2.3 Frontend (raíz)

React 19 en **JavaScript** (`.jsx`), Vite 8, Tailwind 4, React Router 7, `vite-plugin-pwa`. Igual que catalogo-multinivel: `services/http.js` (`apiFetch` único con Bearer y `ApiError`), `services/*Service.js` por módulo, `hooks/useApi.js`, `context/AuthContext.jsx`. Detalle de archivos en `PROJECT_MAP.md`.

- El inicio de sesión es una **máquina de pasos** guiada por `siguientePaso` del backend: contraseña → (cambio de contraseña) → configurar o verificar MFA → aplicación.
- `useInactividad`: aviso 60 s antes del cierre y *heartbeat* (máx. 1/min) mientras el usuario interactúa. El servidor aplica la regla por su cuenta.
- El QR del MFA se dibuja en el navegador (`qrcode`): el secreto no viaja a servicios externos.

## 2.4 Seguridad: qué está en el código y qué depende de la infraestructura

| Control | Dónde | Estado |
|---|---|---|
| RBAC, MFA, inactividad, bloqueo por intentos, límite de logins | Código | ✅ probado |
| Auditoría inmutable y verificable | Código + BD | ✅ probado |
| Cabeceras de seguridad (helmet), CORS restringido, cuerpo máx. 2 MB | Código | ✅ |
| Sin datos clínicos en el navegador ni en Redis | Código | ✅ |
| **TLS 1.3** (RNF-04) | Proxy inverso / plataforma (Render, Caddy) | ⏳ se verifica en el despliegue (Etapa 7) |
| **AES-256 en reposo** (RNF-04) | Disco o BD gestionada (Neon cifra en reposo) y *bucket* de archivos | ⏳ evidencia del proveedor (Etapa 7) |
| **99,9 %** (RNF-02) | Infraestructura y monitoreo | ⏳ |
| Firma de notas y consentimientos | Código (Etapas 3–4) | 🟡 firma **electrónica simple** (re-autenticación + hash). No es firma digital con certificado X.509 ni firma biométrica (ver 01-analisis A-05, A-06). |

# Despliegue de SIGO-Dental (Render + Neon)

Una sola imagen Docker: la API en Go sirve también la PWA compilada (`STATIC_DIR`). PostgreSQL en
**Neon** (gestionado), Redis en **Render Key Value**, la aplicación en un **Web Service** de Render.

> Antes de producción con pacientes reales, lea la sección 8 (lo que falta decidir). Los proveedores
> de mensajería y de facturación siguen **SIMULADOS**.

## 0. Demostración académica (lo que está configurado en `render.yaml`)

`render.yaml` despliega el modo **`APP_ENV=demo`** en el plan **gratuito**: la misma seguridad que
producción (TLS hacia la base, HSTS, CSP, secretos propios), pero al arrancar carga los **datos
ficticios** de la semilla y el paciente de volumen VOL000001, y la PWA muestra el aviso
"DEMOSTRACIÓN · datos ficticios". En la verificación en dos pasos aparece **"Mostrar código de
demostración"** (ruta `/api/demo/mfa`, sólo para usuarios `@demo.sigo` que ya dieron su contraseña;
no existe con `APP_ENV=prod`). Usuarios: los de la semilla (`odontologo@demo.sigo`,
`recepcion@demo.sigo`, `auditor@demo.sigo`, …), contraseña `Demo-Sigo-2026`.

Pasos: §2 (Neon) → §3 (Blueprint; en `WEB_ORIGIN` ponga la URL `https://….onrender.com` que Render
le asigne) → abrir la URL. No hace falta §4: los usuarios demo ya existen.

Limitaciones del plan gratuito: se duerme tras 15 min sin uso (la primera visita tarda ~1 min) y
los archivos subidos se borran al reiniciar. Las mediciones de rendimiento hechas ahí **no** valen
como RNF-01/RNF-03 de producción (CPU compartida); se registran como evidencia del despliegue.

## 1. Probar la imagen en su equipo (Docker Desktop)

```powershell
cd "C:\PROYECTOS FRAMEWORK\sigo-dental-go"
docker build -t sigo-dental .
```

Cree `produccion.env` (no lo suba al repositorio) con valores de prueba:

```
APP_ENV=prod
DATABASE_URL=postgres://usuario:clave@host/base?sslmode=require
JWT_SECRET=<48 caracteres aleatorios>
WEB_ORIGIN=https://localhost
CLINIC_TIMEZONE=America/Guayaquil
CLINIC_NAME=Mi Clínica
REDIS_ADDR=host.docker.internal:6380
```

```powershell
docker run --rm -p 8080:8080 --env-file produccion.env sigo-dental
```

La imagen aplica las migraciones pendientes al arrancar (`./sigo-api migrar`) y luego inicia la API.
En producción la configuración se **rechaza** si la base no usa TLS (`sslmode=require`), si el
`JWT_SECRET` es el de ejemplo o si `WEB_ORIGIN` no es `https://`.

## 2. Base de datos en Neon

1. Cree un proyecto en https://neon.tech (PostgreSQL 16) y una base `sigo`.
2. Copie la cadena de conexión **con** `?sslmode=require`.
3. No hace falta crear tablas: la migración las crea (incluida la extensión `btree_gist`, que Neon
   permite y que usa la regla RN-03 de citas sin superposición).
4. Active los respaldos con restauración a un punto en el tiempo (PITR) y defina su retención.

## 3. Servicio en Render

1. Suba el repositorio a GitHub (ya lo está) y en Render elija **New → Blueprint**: Render lee
   `render.yaml` y crea el Web Service `sigo-dental` y el Key Value `sigo-redis`.
2. Complete las variables marcadas `sync: false`:
   - `DATABASE_URL`: la de Neon (con `sslmode=require`).
   - `WEB_ORIGIN`: `https://sigo-dental.onrender.com` o su dominio.
   - `CLINIC_TIMEZONE`, `CLINIC_NAME`.
   `JWT_SECRET` lo genera Render.
3. El plan `starter` (no el gratuito) permite el **disco persistente** de `render.yaml` para los
   archivos de pacientes y evita que el servicio "duerma".
4. Verifique `https://<servicio>/api/health` → `{"status":"ok","db":"ok","redis":"ok"}`.

## 4. Primer administrador

En Render → el servicio → **Shell**:

```sh
./sigo-api crear-admin admin@suclinica.com "Nombre Apellido"
```

Muestra una contraseña temporal **una sola vez**. Al ingresar se exige cambiarla y configurar el
MFA con una app autenticadora. Desde la aplicación, el administrador crea luego:

- Usuarios (cada uno recibe su contraseña temporal) — Usuarios.
- Sedes, consultorios, sillones, procedimientos con su arancel y convenios — Catálogos.
- Plantillas de consentimiento **aprobadas por su asesoría jurídica** — Consentimientos.

El Auditor crea las ubicaciones de inventario, los insumos, las recetas de consumo y las reglas de
honorarios. La semilla de demostración **no** existe en producción (`semilla`, `totp`, `token`,
`volumen`, `medir` y `carga` se niegan a correr con `APP_ENV=prod`).

## 5. Archivos de pacientes

En desarrollo se guardan en disco (`ARCHIVOS_DIR`). En Render se usa el disco persistente
`/data/archivos` (cifrado en reposo según el proveedor; verifíquelo en su documentación). Para varias
instancias o mayor volumen conviene un almacenamiento de objetos (S3/Azure Blob): el código ya
separa la interfaz `almacenamiento.Almacen`; falta escribir esa implementación.

La imagen corre con un usuario sin privilegios (uid 10001). Si al subir un archivo aparece un error de
permisos en el disco montado, dé permiso de escritura a ese usuario sobre `/data/archivos`.

## 6. Verificaciones después de desplegar (RNF-02, RNF-04)

| Qué | Cómo | Resultado esperado |
|---|---|---|
| TLS 1.3 (RNF-04) | `curl -sv --tlsv1.3 --tls-max 1.3 https://<dominio>/api/health` o https://www.ssllabs.com/ssltest/ | Conexión TLS 1.3; calificación A |
| HSTS | `curl -sI https://<dominio>/` | `Strict-Transport-Security: max-age=31536000; includeSubDomains` |
| CSP y cabeceras | `curl -sI https://<dominio>/` | `Content-Security-Policy` sin `unsafe-inline`, `X-Frame-Options`, `Referrer-Policy: no-referrer` |
| Base con TLS | La API no arranca sin `sslmode=require` | — |
| Cifrado en reposo (AES-256) | Documentación de Neon y de Render (disco) | Evidencia del proveedor, archivada |
| Disponibilidad 99,9 % (RNF-02) | Monitor externo (UptimeRobot, Better Stack…) sobre `/api/health` cada minuto | Informe mensual ≥ 99,9 % |
| Rendimiento | `./sigo-api medir` y `carga` **sólo en un entorno de pruebas** con datos ficticios | p95 < 1,5 s |

Mientras no se completen estas verificaciones, RNF-02 y RNF-04 siguen como ⏳ en
`REQUIREMENTS_CHECKLIST.md`: dependen de la infraestructura, no del código.

## 7. Actualizaciones

1. La integración continua (`.github/workflows/ci.yml`) debe estar en verde.
2. En Render: **Manual Deploy** (el blueprint tiene `autoDeploy: false`).
3. Las migraciones nuevas se aplican solas al arrancar; son idempotentes.

## 8. Lo que falta decidir antes de usar datos reales

- **Q-01 País:** autoridad tributaria, formato del comprobante e impuestos ⇒ proveedor de
  facturación real. Hoy los comprobantes son `SIM-…` sin validez tributaria.
- **Mensajería real** (WhatsApp Business / SMS / correo) con las credenciales de la clínica.
- **Q-02 Firma:** si la normativa exige firma con certificado, integrar un proveedor acreditado.
- Textos legales de consentimientos y recetas, ley de protección de datos del país, contratos de
  tratamiento de datos con Render y Neon, y política de respaldos y retención.

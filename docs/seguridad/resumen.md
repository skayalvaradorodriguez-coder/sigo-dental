# Seguridad — resultados medidos (Etapa 7)

Fecha: 30/10/2026. Entorno: desarrollo, con la build de producción servida por la API.

| Análisis | Herramienta | Resultado |
|---|---|---|
| SAST del backend | gosec 2.21.4 | **0 hallazgos**. Avisos iniciales: se corrigieron los de permisos de archivos (0600/0750) y un error sin manejar; los demás se suprimieron con justificación en el código (6 `#nosec`): HMAC-SHA1 exigido por RFC 6238 (TOTP), conversiones de enteros ya acotadas, contraseña pública de los usuarios ficticios de demostración, ruta de archivo validada por expresión regular, aleatoriedad débil sólo para espaciar la prueba de carga. |
| Dependencias del frontend | npm audit | Producción: **0 vulnerabilidades**. Desarrollo: 4 bajas en la cadena de compilación de la PWA (deepmerge vía workbox-build), sin altas ni críticas. |
| Dependencias de Go | govulncheck | ⏳ No se pudo ejecutar en este entorno (sin acceso a vuln.go.dev). Corre en la integración continua y con `scripts\seguridad.ps1`. |
| DAST | OWASP ZAP 2.16.1 (spider + importación de las 105 rutas del OpenAPI + escaneo activo autenticado) | **0 hallazgos altos, medios o bajos**; 3 informativos (comentarios en el JS minificado de bibliotecas, "aplicación web moderna", fuzzer de User-Agent). Informe: `zap-informe.html`; plan: `zap-plan.yaml`. |

Hallazgos de ZAP corregidos en esta etapa:

1. **CSP con `style-src 'unsafe-inline'`** (medio): la hoja de impresión pasó a `public/imprimir.css`;
   la CSP ya no admite estilos ni scripts en línea. Una prueba de navegador verifica que ninguna
   pantalla (ni la impresión) viola la CSP.
2. **"Archivo oculto encontrado"** (medio, confianza baja): rutas como `/.git` o `/BitKeeper`
   devolvían la app con 200. Ahora: archivos ocultos ⇒ 404, y rutas que no son de la app ⇒ 404.
3. **Respuestas de la API en caché** (informativo): toda respuesta de `/api` lleva `Cache-Control: no-store`.

Otras medidas de esta etapa: HSTS en producción; en producción la API no arranca sin TLS hacia la base
(`sslmode=require`), con el `JWT_SECRET` de ejemplo o con `WEB_ORIGIN` sin `https://`.

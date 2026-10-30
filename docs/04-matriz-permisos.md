# 4. Matriz de permisos (RBAC, RNF-05)

Fuente única: `sigo-backend/permisos/permisos.go` (`PorRol`). Cada ruta declara su permiso al registrarse (`routes/routes.go`) y el middleware `RequirePermisos` / `RequireAlguno` lo verifica en el **backend**. El frontend sólo oculta opciones (`components/modulos.js`); no protege nada.

**Denegar por defecto:** la prueba `TestTodasLasRutasTienenPolitica` recorre todas las rutas de Fiber y falla si alguna no tiene política declarada. Además, toda ruta que no sea pública exige token válido y, salvo `/me`, `/logout`, `/password` y los pasos de MFA, **MFA verificado**.

| Grupo | Permiso | Admin TI | Recepción | Odontólogo | Asistente | Auditor |
|---|---|---|---|---|---|---|
| Administración | `usuarios.gestionar` | ✔ |  |  |  |  |
|  | `ajustes.gestionar` | ✔ |  |  |  |  |
|  | `catalogos.gestionar` | ✔ |  |  |  |  |
|  | `auditoria.leer` | ✔ |  |  |  |  |
| Pacientes (RF-01, RF-02) | `pacientes.leer` |  | ✔ | ✔ | ✔ |  |
|  | `pacientes.escribir` |  | ✔ | ✔ |  |  |
|  | `anamnesis.escribir` |  |  | ✔ | ✔ |  |
| Clínica (RF-03…06, RN-01) | `clinica.leer` |  |  | ✔ | ✔ |  |
|  | `clinica.escribir` |  |  | ✔ | ✔ |  |
|  | `clinica.firmar` |  |  | ✔ |  |  |
|  | `consentimientos.gestionar` |  |  | ✔ |  |  |
|  | `recetas.escribir` |  |  | ✔ |  |  |
| Presupuestos (RF-07, RF-08) | `presupuestos.leer` |  | ✔ | ✔ |  |  |
|  | `presupuestos.escribir` |  |  | ✔ |  |  |
|  | `presupuestos.aprobar` |  | ✔ | ✔ |  |  |
| Agenda (RF-09…11, RN-03) | `citas.leer` |  | ✔ | ✔ | ✔ |  |
|  | `citas.escribir` |  | ✔ | ✔ |  |  |
| Caja (RF-12, RN-02) | `caja.operar` |  | ✔ |  |  |  |
|  | `caja.revisar` |  |  |  |  | ✔ |
|  | `facturacion.emitir` |  | ✔ |  |  |  |
| Inventario (RF-14) | `inventario.leer` |  |  | ✔ | ✔ | ✔ |
|  | `inventario.escribir` |  |  |  | ✔ |  |
|  | `inventario.configurar` |  |  |  |  | ✔ |
| Honorarios (RF-13) | `honorarios.propios` |  |  | ✔ |  |  |
|  | `honorarios.todos` |  |  |  |  | ✔ |
|  | `honorarios.configurar` |  |  |  |  | ✔ |

Decisiones de menor privilegio (ver 01-analisis, supuestos):

- El **Administrador TI** gestiona usuarios, catálogos, parámetros y la bitácora, pero **no ve pacientes ni la agenda** (datos de salud).
- **Recepción** ve la ficha y las **alertas** de la cabecera (necesarias para agendar con seguridad), pero no el detalle de la anamnesis ni notas clínicas.
- El **Asistente** registra anamnesis y (en la Etapa 3) borradores clínicos; sólo el odontólogo firma (`clinica.firmar`).
- El **Auditor/Contador** revisa caja, inventario y honorarios; no accede a la bitácora de seguridad (S-11, pregunta Q-05).
- `honorarios.propios` vs `honorarios.todos`: el odontólogo sólo ve su liquidación.

Uso en el expediente clínico (Etapa 3): leer odontograma, periodontograma, notas y procedimientos ⇒ `clinica.leer`; registrar hallazgos, periodontogramas y borradores de nota ⇒ `clinica.escribir`; firmar notas, agregar adendas y registrar procedimientos realizados ⇒ `clinica.firmar` (además del código MFA en el momento de firmar).

Verificación: `pruebas/permisos_test.go` prueba la matriz rol × endpoint contra la API real (403 donde no corresponde) y `permisos/permisos_test.go` comprueba que cada permiso pertenece al catálogo.

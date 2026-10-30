# 3. Modelo de datos

Base de datos: **PostgreSQL 16**. Identificadores `uuid` (salvo la bitácora, `bigserial` para conservar el orden). Fechas en `timestamptz`; la API las devuelve en ISO-8601 UTC y la interfaz las muestra en hora local. Montos en `NUMERIC(12,2)` (nunca `float`). Nombres de tablas y columnas **en español**, igual que en catalogo-multinivel.

Migraciones: `sigo-backend/database/*.sql`, embebidas en el binario (`//go:embed`) y aplicadas en orden por `go run . migrar`. Cada una es **idempotente** (`IF NOT EXISTS`, bloques `DO`), se registra en `schema_migraciones` y se verificó aplicándola dos veces.

Principios:

1. **Lo clínico no se borra.** Las tablas clínicas son *sólo inserción* o tienen triggers que impiden `DELETE` y la edición de registros cerrados (RN-01). Las correcciones son filas nuevas (versiones o adendas).
2. **Las reglas críticas viven también en la BD**, no sólo en la API: superposición de citas (RN-03), inmutabilidad (RN-01, RNF-06), rangos y montos no negativos.
3. **Historial en lugar de sobrescritura** para anamnesis, estados de cita, odontograma y stock.

## 3.1 Etapa 1 — `001_base.sql`

| Tabla | Propósito | Puntos clave |
|---|---|---|
| `usuarios` | Usuarios del sistema | `rol` (CHECK con los 5 perfiles), `password_hash` **bcrypt**, `mfa_secreto` / `mfa_activo` / `mfa_ultimo_paso` (anti-replay TOTP), `intentos_fallidos` / `bloqueado_hasta`, `debe_cambiar_password`, `registro_profesional`. |
| `sesiones` | Estado de servidor de cada JWT | Clave `jti` (id del token), `mfa_verificado`, `ultima_actividad` (inactividad), `expira_en` (máximo absoluto 12 h), `revocada_en` + `motivo_revocacion`. |
| `ajustes` | Parámetros configurables | `seguridad.minutosInactividad` (5–15), `clinica.horasBloqueoNotas` (1–72), `seguridad.maxIntentosFallidos` (3–10), `seguridad.minutosBloqueo` (5–120). Rango validado en la API; caché Redis de 1 min. |
| `auditoria` | Bitácora (RNF-06) | `ocurrido_en`, `usuario_id`, `accion`, `entidad`, `entidad_id`, `ip`, `user_agent`, `detalle` (jsonb), `hash_anterior`, `hash`. Triggers que impiden `UPDATE`, `DELETE` y `TRUNCATE`. |

**Cadena de hashes.** `hash = SHA-256(hash_anterior + JSON canónico del evento)`. La inserción toma un `pg_advisory_xact_lock` para que dos eventos simultáneos no compartan el mismo anterior. `POST /api/auditoria/verificar` recorre la cadena y devuelve el primer registro alterado.

## 3.2 Etapa 2 — `002_pacientes_agenda.sql`

| Tabla | Propósito | Puntos clave |
|---|---|---|
| `sedes`, `consultorios`, `sillones` | Sede → consultorio → sillón | Nombre único dentro de su padre; `activo` para desactivar sin borrar. |
| `procedimientos` | Catálogo | `codigo` único, `duracion_min` 5–480, `arancel NUMERIC(12,2) ≥ 0`. |
| `convenios` | Seguros y convenios | Nombre único, aseguradora opcional. |
| `pacientes` | Ficha única (RF-01) | `UNIQUE (tipo_documento, numero_documento)`; canal preferido y **autorización de mensajes**; trigger que impide `DELETE`. |
| `tutores` | Tutor legal (uno por paciente) | Obligatorio si el paciente es menor de 18 (validado en la API: `TUTOR_REQUERIDO`). |
| `anamnesis` | Anamnesis versionada (RF-02) | `UNIQUE (paciente_id, version)`; alergias en `jsonb`; trigger **sólo inserción** (rechaza `UPDATE`/`DELETE`). |
| `citas` | Citas (RF-09, RF-10) | `CHECK (fin > inicio)`, estados por `CHECK`, **`EXCLUDE USING gist` por sillón** (RN-03), trigger que impide `DELETE`. |
| `citas_eventos` | Historial de estados | Sólo inserción. |
| `notificaciones` | Confirmación y recordatorios 48 h / 24 h (RF-11) | `estado` (PENDIENTE, ENVIADO, FALLIDO, CANCELADO, OMITIDO), `motivo`, `proveedor`, `simulado`, `id_proveedor`, `intentos`. |

### RN-03 en SQL

```sql
ALTER TABLE citas ADD CONSTRAINT citas_sin_superposicion_por_sillon
  EXCLUDE USING gist (sillon_id WITH =, tstzrange(inicio, fin, '[)') WITH &&)
  WHERE (estado NOT IN ('CANCELADO', 'NO_ASISTIO'));
```

- El intervalo `[)` permite citas consecutivas (una termina 10:30 y otra empieza 10:30).
- Como la regla la evalúa PostgreSQL, **dos peticiones simultáneas no pueden pasar ambas** (prueba: 20 en paralelo ⇒ 1 éxito y 19 rechazos).
- La API consulta antes para dar un mensaje claro y traduce el error `23P01` a **409 `SILLON_OCUPADO`** con la cita en conflicto en `extra.conflicto`.
- Un `INSERT` directo en la base también falla (probado en `pruebas/agenda_test.go`).

### Despacho de recordatorios

Una goroutine (`notificaciones.Despachador`) revisa cada 30 s los mensajes vencidos con `SELECT … FOR UPDATE SKIP LOCKED` (varias instancias de la API no envían el mismo mensaje dos veces), hasta 3 intentos. Al cancelar una cita, sus mensajes pendientes pasan a CANCELADO; al reprogramarla, se cancelan y se crean de nuevo.

## 3.3 Etapa 3 — `003_clinica.sql`

| Tabla | Propósito | Puntos clave |
|---|---|---|
| `odontograma_eventos` | Odontograma (RF-03) | Cada marca es un evento `AGREGAR` / `QUITAR` (sólo inserción). Diente en **FDI** con `CHECK` de permanentes (11–48) y temporales (51–85); superficie canónica `M, D, O, V, L` (O = oclusal/incisal, L = lingual/palatino). El estado vigente se calcula en `domain.EstadoActual`. |
| `periodontogramas`, `perio_dientes`, `perio_sitios` | Periodontograma (RF-04) | 6 sitios por diente (`MV, V, DV, ML, L, DL`), PS 0–15, MG −10…15, **`nic` columna generada = PS + MG**; movilidad y furca 0–3. Sólo inserción. |
| `notas_soep` | Evolución SOEP (RF-05) | `bloqueo_en` = creación + `clinica.horasBloqueoNotas`. Firma: `firmada_en`, `firmada_por`, `hash_firma` (los tres o ninguno, por `CHECK`). |
| `notas_adendas` | Correcciones de notas | Sólo inserción, cada una con su hash. |
| `procedimientos_realizados` | RN-01 | Firmado al registrarse (`hash_firma`); **sin `UPDATE` ni `DELETE`** (trigger). |
| `procedimientos_adendas` | Correcciones (RN-01) | Sólo inserción, con hash. |

Guardia de las notas en la BD (`sigo_nota_soep_guardia`):

- `DELETE` siempre rechazado.
- Nota firmada ⇒ ningún cambio.
- Paciente, autor, cita y fechas nunca cambian.
- Vencido `bloqueo_en` ⇒ el texto S/O/E/P no cambia (sólo se admite registrar la firma).

Así, aunque alguien escriba SQL directo, no puede editar una nota fuera de plazo. Y si un administrador de base de datos desactiva el trigger y altera una nota firmada, el hash ya no coincide y la interfaz la marca como **posible alteración** (probado en `TestNotasSOEPFirmaYAdendas`).

## 3.4 Etapa 4 — `004_planes.sql`

| Tabla | Propósito | Puntos clave |
|---|---|---|
| `presupuestos` | RF-07 | `estado` BORRADOR → PRESENTADO → APROBADO / APROBADO_PARCIAL / RECHAZADO; `hash_presentado` (SHA-256 de ítems y montos); quién y cuándo registró la decisión. |
| `presupuesto_items` | Fases y sesiones | `fase` 1–20, `sesion` 1–50, diente FDI y superficies opcionales; `honorario`, `costo_laboratorio`, `descuento` en `NUMERIC(12,2)` con `descuento ≤ honorario`; `descuento_autorizado_por/en` (S-25); `decision` por ítem. |
| `tareas_tratamiento` | RF-08 | Una por ítem aprobado (`presupuesto_item_id UNIQUE`). Sólo inserción: el estado se **deriva** de `citas.tarea_id` (índice único parcial sobre citas activas) y `procedimientos_realizados.tarea_id` (índice único). |
| `consentimiento_plantillas` | Plantillas | `(codigo, version) UNIQUE`; sólo inserción (una edición es una versión nueva). |
| `consentimientos` | RF-06 | Texto final y `hash_documento`; firmante (PACIENTE/TUTOR); `metodo` MANUSCRITA (`firma_imagen` PNG + SHA-256) u OTP (`otp_hash`, vencimiento, intentos, canal, destinatario enmascarado, proveedor y `otp_simulado`); `hash_firma`. `CHECK` de coherencia entre método, estado y datos de firma. |
| `recetas`, `receta_items` | P-01 | Alertas confirmadas (`jsonb`), `hash_firma`; sólo inserción. |
| `archivos_paciente` | Adjuntos | Tipo `image/jpeg`, `image/png`, `application/pdf`; `tamano ≤ 10 MB`; `sha256`; `clave` única en el almacén; sólo inserción. |

Guardias en la BD: presupuesto en borrador editable; **presentado ⇒ sólo se registra la decisión** (ni título, ni ítems, ni montos); decidido ⇒ nada; nunca se borra. Consentimiento: nunca se borra, el texto y su hash nunca cambian y, firmado, nada cambia.

## 3.5 Etapa 5 — `005_caja.sql`

| Tabla | Propósito | Puntos clave |
|---|---|---|
| `caja_sesiones` | Caja del recepcionista | Índice único parcial: una ABIERTA por cajero. Al cerrar: `declarado_*` y `teorico_*` (efectivo con fondo, tarjeta, transferencia) guardados en la misma transacción. `CHECK` de coherencia por estado. |
| `cobros` | RF-12 | `numero` de recibo interno (secuencia); `total > 0`. Trigger: sólo en caja ABIERTA (`FOR SHARE`). Trigger **diferido**: conceptos = pagos = total al confirmar. |
| `cobro_conceptos` | Qué se cobra | Procedimiento realizado (con su precio) o procedimiento del catálogo. Trigger: un realizado no se cobra dos veces (con `FOR UPDATE` del realizado), salvo anulación. |
| `cobro_pagos` | Pago mixto | `medio` EFECTIVO/TARJETA/TRANSFERENCIA/SEGURO; tarjeta y transferencia exigen referencia; seguro exige convenio (cuenta por cobrar). |
| `cobros_anulaciones` | Anulación | Una por cobro, con motivo; sólo en caja abierta. |
| `comprobantes` | Facturación | Cada intento es una fila (AUTORIZADO, RECHAZADO o ERROR) con el documento enviado y su hash; a lo sumo una FACTURA y una NOTA_CREDITO autorizadas por cobro. |

Todo es sólo inserción salvo `caja_sesiones`, cuya guardia permite ABIERTA → CERRADA (sólo datos de cierre) → REVISADA (sólo datos de revisión).

## 3.6 Etapa 6 — `006_inventario_honorarios.sql`

| Tabla | Propósito | Puntos clave |
|---|---|---|
| `inventario_ubicaciones` | Bodega y consultorios | Una por consultorio (`consultorio_id UNIQUE`); una sola `por_defecto`. |
| `insumos` | Catálogo | `costo_unitario NUMERIC(12,4)` (último costo), `stock_minimo NUMERIC(12,3)`; sin borrado. |
| `recetas_consumo` | Consumo estándar | (procedimiento, insumo) → cantidad. Configuración: los cambios se auditan y no tocan descargos pasados. |
| `movimientos_stock` | Kardex | Cantidad con signo; ENTRADA > 0, SALIDA_PROCEDIMIENTO < 0 y ligada al realizado, AJUSTE con motivo, TRANSFERENCIA con dos patas que suman cero (trigger diferido). Guarda el costo vigente. Sólo inserción. |
| `reglas_honorario` | RF-13 | Alcance odontólogo y/o procedimiento (NULL = todos), tipo, valor (% ≤ 100 o monto), vigencia. Sólo inserción. |
| `liquidaciones_honorarios`, `liquidacion_lineas` | Liquidación | Copia del cálculo por línea; `cobro_concepto_id UNIQUE` ⇒ un cobro se liquida una vez. Sólo inserción. |

Descargo (`RegistrarRealizado`): `INSERT` del procedimiento y `INSERT … SELECT` de la receta en la misma transacción.

## 3.7 Convenciones clínicas

### Numeración dental

| Dentición | FDI | Universal |
|---|---|---|
| Permanente (32) | Cuadrantes 1–4, dientes 1–8: 18…11, 21…28, 38…31, 41…48 | 1 (=18) … 8 (=11), 9 (=21) … 16 (=28), 17 (=38) … 24 (=31), 25 (=41) … 32 (=48) |
| Temporal (20) | Cuadrantes 5–8, dientes 1–5: 55…51, 61…65, 75…71, 81…85 | A (=55) … E (=51), F (=61) … J (=65), K (=75) … O (=71), P (=81) … T (=85) |

Se **almacena siempre FDI**; Universal es sólo una forma de mostrar (preferencia del usuario). Así nunca hay dos códigos para el mismo diente en la BD.

Superficies: **M** (mesial), **D** (distal), **O/I** (oclusal en posteriores, incisal en anteriores), **V** (vestibular), **L/P** (lingual en inferiores, palatino en superiores).

### Periodontograma: cálculo del NIC

Seis sitios por diente: mesio-vestibular, vestibular, disto-vestibular, mesio-lingual/palatino, lingual/palatino, disto-lingual/palatino.

- **PS** (profundidad de sondaje): del margen gingival al fondo del surco/bolsa, en mm (≥ 0).
- **MG** (margen gingival): distancia del **límite amelocementario (LAC)** al margen gingival, con esta convención de signo:
  - **positivo** = el margen está **apical** a la LAC (**recesión**);
  - **negativo** = el margen está **coronal** a la LAC (agrandamiento o encía que cubre la LAC);
  - 0 = coincide con la LAC.
- **NIC = PS + MG**

Ejemplos: PS 4 mm con recesión de 2 mm (MG = +2) ⇒ NIC 6 mm. PS 5 mm con margen 2 mm coronal a la LAC (MG = −2) ⇒ NIC 3 mm.

Algunos formularios usan el signo opuesto para el margen (y entonces calculan NIC = PS − MG). El resultado clínico es el mismo; lo importante es que la interfaz rotule la convención junto al campo y que el NIC se calcule en el servidor (columna generada), no a mano.

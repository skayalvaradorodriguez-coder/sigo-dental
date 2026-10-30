-- 004_planes.sql — Etapa 4: presupuestos y tareas (RF-07, RF-08), consentimientos (RF-06),
-- recetas (propuesta P-01) y archivos del paciente. Idempotente.

-- --------------------------------------------------------------- Presupuestos (RF-07)
CREATE TABLE IF NOT EXISTS presupuestos (
  id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  paciente_id             UUID NOT NULL REFERENCES pacientes(id),
  odontologo_id           UUID NOT NULL REFERENCES usuarios(id),   -- quien lo elabora
  titulo                  TEXT NOT NULL,
  observaciones           TEXT,
  estado                  TEXT NOT NULL DEFAULT 'BORRADOR'
                          CHECK (estado IN ('BORRADOR','PRESENTADO','APROBADO','APROBADO_PARCIAL','RECHAZADO')),
  creado_en               TIMESTAMPTZ NOT NULL DEFAULT now(),
  actualizado_en          TIMESTAMPTZ NOT NULL DEFAULT now(),
  presentado_en           TIMESTAMPTZ,
  hash_presentado         TEXT,          -- SHA-256 de los ítems y montos al presentarlo
  decidido_en             TIMESTAMPTZ,
  decision_registrada_por UUID REFERENCES usuarios(id),
  decision_nota           TEXT
);
CREATE INDEX IF NOT EXISTS presupuestos_paciente ON presupuestos (paciente_id, creado_en DESC);
CREATE INDEX IF NOT EXISTS presupuestos_estado ON presupuestos (estado, actualizado_en DESC);

-- Montos en NUMERIC (nunca float). descuento ≤ honorario. Un descuento por encima del tope
-- configurado (presupuestos.topeDescuento) debe autorizarlo OTRA persona (supuesto S-25).
CREATE TABLE IF NOT EXISTS presupuesto_items (
  id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  presupuesto_id          UUID NOT NULL REFERENCES presupuestos(id),
  orden                   SMALLINT NOT NULL,
  fase                    SMALLINT NOT NULL CHECK (fase BETWEEN 1 AND 20),
  fase_nombre             TEXT NOT NULL,
  sesion                  SMALLINT NOT NULL CHECK (sesion BETWEEN 1 AND 50),
  procedimiento_id        UUID NOT NULL REFERENCES procedimientos(id),
  diente                  SMALLINT,
  superficies             TEXT[] NOT NULL DEFAULT '{}',
  honorario               NUMERIC(12,2) NOT NULL CHECK (honorario >= 0),
  costo_laboratorio       NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (costo_laboratorio >= 0),
  descuento               NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (descuento >= 0),
  descuento_autorizado_por UUID REFERENCES usuarios(id),
  descuento_autorizado_en TIMESTAMPTZ,
  decision                TEXT NOT NULL DEFAULT 'PENDIENTE' CHECK (decision IN ('PENDIENTE','APROBADO','RECHAZADO')),
  CONSTRAINT presupuesto_items_descuento_max CHECK (descuento <= honorario),
  UNIQUE (presupuesto_id, orden)
);

-- Tareas agendables: una por ítem aprobado (RF-08). Su estado (pendiente, agendada, realizada)
-- se DERIVA de las citas y procedimientos que la referencian; la fila no cambia.
CREATE TABLE IF NOT EXISTS tareas_tratamiento (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  presupuesto_item_id UUID NOT NULL UNIQUE REFERENCES presupuesto_items(id),
  paciente_id         UUID NOT NULL REFERENCES pacientes(id),
  procedimiento_id    UUID NOT NULL REFERENCES procedimientos(id),
  diente              SMALLINT,
  superficies         TEXT[] NOT NULL DEFAULT '{}',
  fase                SMALLINT NOT NULL,
  sesion              SMALLINT NOT NULL,
  creado_en           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS tareas_paciente ON tareas_tratamiento (paciente_id, fase, sesion);

ALTER TABLE citas ADD COLUMN IF NOT EXISTS tarea_id UUID REFERENCES tareas_tratamiento(id);
-- Una tarea no puede tener dos citas activas a la vez.
CREATE UNIQUE INDEX IF NOT EXISTS citas_tarea_activa ON citas (tarea_id)
  WHERE tarea_id IS NOT NULL AND estado NOT IN ('CANCELADO','NO_ASISTIO');
ALTER TABLE procedimientos_realizados ADD COLUMN IF NOT EXISTS tarea_id UUID REFERENCES tareas_tratamiento(id);
CREATE UNIQUE INDEX IF NOT EXISTS realizados_tarea ON procedimientos_realizados (tarea_id) WHERE tarea_id IS NOT NULL;

-- --------------------------------------------------------------- Consentimientos (RF-06)
-- Plantillas versionadas: "editar" una plantilla crea una versión nueva; los consentimientos
-- firmados conservan el texto exacto que se firmó.
CREATE TABLE IF NOT EXISTS consentimiento_plantillas (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  codigo            TEXT NOT NULL,
  version           INTEGER NOT NULL,
  titulo            TEXT NOT NULL,
  texto             TEXT NOT NULL,
  procedimiento_id  UUID REFERENCES procedimientos(id),
  creado_por        UUID REFERENCES usuarios(id),
  creado_en         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (codigo, version)
);

-- Firma: trazo manuscrito digitalizado (imagen PNG, NO biométrica) u OTP enviado al paciente.
CREATE TABLE IF NOT EXISTS consentimientos (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  paciente_id         UUID NOT NULL REFERENCES pacientes(id),
  plantilla_id        UUID NOT NULL REFERENCES consentimiento_plantillas(id),
  procedimiento_id    UUID REFERENCES procedimientos(id),
  odontologo_id       UUID NOT NULL REFERENCES usuarios(id),
  texto               TEXT NOT NULL,          -- documento ya completado con los datos
  hash_documento      TEXT NOT NULL,          -- SHA-256 del texto
  firmante_relacion   TEXT NOT NULL CHECK (firmante_relacion IN ('PACIENTE','TUTOR')),
  firmante_nombre     TEXT NOT NULL,
  firmante_documento  TEXT NOT NULL,
  metodo              TEXT NOT NULL CHECK (metodo IN ('MANUSCRITA','OTP')),
  estado              TEXT NOT NULL CHECK (estado IN ('PENDIENTE_OTP','FIRMADO')),
  creado_en           TIMESTAMPTZ NOT NULL DEFAULT now(),
  firma_imagen        BYTEA,
  firma_imagen_sha256 TEXT,
  otp_hash            TEXT,
  otp_expira          TIMESTAMPTZ,
  otp_intentos        SMALLINT NOT NULL DEFAULT 0,
  otp_canal           TEXT,
  otp_destinatario    TEXT,
  otp_proveedor       TEXT,
  otp_simulado        BOOLEAN,
  otp_id_proveedor    TEXT,
  firmado_en          TIMESTAMPTZ,
  hash_firma          TEXT,
  CONSTRAINT consentimiento_firma CHECK (
    (estado = 'FIRMADO') = (firmado_en IS NOT NULL AND hash_firma IS NOT NULL)),
  CONSTRAINT consentimiento_metodo CHECK (
    (metodo = 'MANUSCRITA' AND firma_imagen IS NOT NULL) OR (metodo = 'OTP' AND otp_hash IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS consentimientos_paciente ON consentimientos (paciente_id, creado_en DESC);

-- --------------------------------------------------------------- Recetas (propuesta P-01)
CREATE TABLE IF NOT EXISTS recetas (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  paciente_id          UUID NOT NULL REFERENCES pacientes(id),
  odontologo_id        UUID NOT NULL REFERENCES usuarios(id),
  indicaciones         TEXT,
  alertas_confirmadas  JSONB NOT NULL DEFAULT '[]',   -- avisos de alergia que el profesional aceptó
  emitida_en           TIMESTAMPTZ NOT NULL DEFAULT now(),
  hash_firma           TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS recetas_paciente ON recetas (paciente_id, emitida_en DESC);

CREATE TABLE IF NOT EXISTS receta_items (
  receta_id     UUID NOT NULL REFERENCES recetas(id),
  orden         SMALLINT NOT NULL,
  medicamento   TEXT NOT NULL,
  presentacion  TEXT NOT NULL,
  dosis         TEXT NOT NULL,
  via           TEXT NOT NULL,
  frecuencia    TEXT NOT NULL,
  duracion      TEXT NOT NULL,
  indicaciones  TEXT,
  PRIMARY KEY (receta_id, orden)
);

-- --------------------------------------------------------------- Archivos del paciente
-- El contenido vive en el almacenamiento (disco local en desarrollo; S3/Azure en producción);
-- aquí queda el registro con su SHA-256 para verificar que no cambió.
CREATE TABLE IF NOT EXISTS archivos_paciente (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  paciente_id  UUID NOT NULL REFERENCES pacientes(id),
  categoria    TEXT NOT NULL CHECK (categoria IN ('RADIOGRAFIA','FOTOGRAFIA','DOCUMENTO','OTRO')),
  nombre       TEXT NOT NULL,
  tipo_mime    TEXT NOT NULL CHECK (tipo_mime IN ('image/jpeg','image/png','application/pdf')),
  tamano       INTEGER NOT NULL CHECK (tamano BETWEEN 1 AND 10485760),
  sha256       TEXT NOT NULL,
  clave        TEXT NOT NULL UNIQUE,     -- ubicación en el almacenamiento
  descripcion  TEXT,
  subido_por   UUID NOT NULL REFERENCES usuarios(id),
  subido_en    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS archivos_paciente_idx ON archivos_paciente (paciente_id, subido_en DESC);

-- --------------------------------------------------------------- Reglas en la BD
-- Presupuesto: no se borra; decidido ⇒ no cambia.
CREATE OR REPLACE FUNCTION sigo_presupuesto_guardia() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'presupuestos no admite borrado' USING ERRCODE = 'P0001';
  END IF;
  IF OLD.estado IN ('APROBADO','APROBADO_PARCIAL','RECHAZADO') THEN
    RAISE EXCEPTION 'presupuesto decidido: no se modifica' USING ERRCODE = 'P0001';
  END IF;
  IF NEW.paciente_id <> OLD.paciente_id OR NEW.odontologo_id <> OLD.odontologo_id OR NEW.creado_en <> OLD.creado_en THEN
    RAISE EXCEPTION 'presupuesto: paciente, autor y fecha no cambian' USING ERRCODE = 'P0001';
  END IF;
  -- Presentado: congelado; sólo se registra la decisión del paciente.
  IF OLD.estado = 'PRESENTADO' AND (NEW.estado = 'BORRADOR'
     OR NEW.titulo IS DISTINCT FROM OLD.titulo OR NEW.observaciones IS DISTINCT FROM OLD.observaciones
     OR NEW.presentado_en IS DISTINCT FROM OLD.presentado_en OR NEW.hash_presentado IS DISTINCT FROM OLD.hash_presentado) THEN
    RAISE EXCEPTION 'presupuesto presentado: sólo se registra la decisión' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS presupuestos_guardia ON presupuestos;
CREATE TRIGGER presupuestos_guardia BEFORE UPDATE OR DELETE ON presupuestos
  FOR EACH ROW EXECUTE FUNCTION sigo_presupuesto_guardia();

-- Ítems: sólo se agregan, cambian o quitan en BORRADOR; una vez PRESENTADO sólo se registra la
-- decisión del paciente; decidido ⇒ nada.
CREATE OR REPLACE FUNCTION sigo_presupuesto_item_guardia() RETURNS trigger AS $$
DECLARE est TEXT;
BEGIN
  SELECT estado INTO est FROM presupuestos WHERE id = COALESCE(NEW.presupuesto_id, OLD.presupuesto_id);
  IF est = 'BORRADOR' THEN
    RETURN COALESCE(NEW, OLD);
  END IF;
  IF TG_OP = 'UPDATE' AND est = 'PRESENTADO'
     AND (NEW.fase, NEW.sesion, NEW.procedimiento_id, NEW.honorario, NEW.costo_laboratorio, NEW.descuento, NEW.orden)
         IS NOT DISTINCT FROM (OLD.fase, OLD.sesion, OLD.procedimiento_id, OLD.honorario, OLD.costo_laboratorio, OLD.descuento, OLD.orden)
     AND NEW.diente IS NOT DISTINCT FROM OLD.diente AND NEW.superficies = OLD.superficies
     AND NEW.descuento_autorizado_por IS NOT DISTINCT FROM OLD.descuento_autorizado_por THEN
    RETURN NEW;  -- sólo cambia la decisión
  END IF;
  RAISE EXCEPTION 'presupuesto % (%): los items ya no se modifican', COALESCE(NEW.presupuesto_id, OLD.presupuesto_id), est USING ERRCODE = 'P0001';
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS presupuesto_items_guardia ON presupuesto_items;
CREATE TRIGGER presupuesto_items_guardia BEFORE INSERT OR UPDATE OR DELETE ON presupuesto_items
  FOR EACH ROW EXECUTE FUNCTION sigo_presupuesto_item_guardia();

-- Consentimiento: no se borra; firmado ⇒ no cambia; el documento (texto y hash) nunca cambia.
CREATE OR REPLACE FUNCTION sigo_consentimiento_guardia() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'consentimientos no admite borrado' USING ERRCODE = 'P0001';
  END IF;
  IF OLD.estado = 'FIRMADO' THEN
    RAISE EXCEPTION 'consentimiento firmado: no se modifica' USING ERRCODE = 'P0001';
  END IF;
  IF NEW.texto <> OLD.texto OR NEW.hash_documento <> OLD.hash_documento OR NEW.paciente_id <> OLD.paciente_id
     OR NEW.firmante_nombre <> OLD.firmante_nombre OR NEW.metodo <> OLD.metodo THEN
    RAISE EXCEPTION 'consentimiento: el documento y el firmante no cambian' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS consentimientos_guardia ON consentimientos;
CREATE TRIGGER consentimientos_guardia BEFORE UPDATE OR DELETE ON consentimientos
  FOR EACH ROW EXECUTE FUNCTION sigo_consentimiento_guardia();

DO $$
DECLARE t TEXT;
BEGIN
  FOREACH t IN ARRAY ARRAY['tareas_tratamiento','consentimiento_plantillas','recetas','receta_items','archivos_paciente'] LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS %I_solo_insercion ON %I', t, t);
    EXECUTE format('CREATE TRIGGER %I_solo_insercion BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION sigo_solo_insercion()', t, t);
  END LOOP;
END $$;

-- 003_clinica.sql — Etapa 3: núcleo clínico (RF-03, RF-04, RF-05, RN-01).
-- Idempotente. Principio: lo clínico no se borra ni se sobrescribe; las correcciones son filas nuevas.

-- --------------------------------------------------------------- Odontograma (RF-03)
-- Cada marca es un EVENTO (agregar o quitar un hallazgo). El estado actual se obtiene del último
-- evento por (diente, superficie, hallazgo); el historial completo se conserva.
-- Los dientes se guardan SIEMPRE en notación FDI; Universal es sólo una forma de mostrarlos.
CREATE TABLE IF NOT EXISTS odontograma_eventos (
  id              BIGSERIAL PRIMARY KEY,
  paciente_id     UUID NOT NULL REFERENCES pacientes(id),
  diente          SMALLINT NOT NULL CHECK (
                    (diente / 10 BETWEEN 1 AND 4 AND diente % 10 BETWEEN 1 AND 8) OR
                    (diente / 10 BETWEEN 5 AND 8 AND diente % 10 BETWEEN 1 AND 5)),
  superficie      TEXT CHECK (superficie IN ('M','D','O','V','L')),  -- NULL = hallazgo de todo el diente
  hallazgo        TEXT NOT NULL,
  accion          TEXT NOT NULL CHECK (accion IN ('AGREGAR','QUITAR')),
  nota            TEXT,
  registrado_por  UUID NOT NULL REFERENCES usuarios(id),
  registrado_en   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS odontograma_eventos_paciente ON odontograma_eventos (paciente_id, id);

-- --------------------------------------------------------------- Periodontograma (RF-04)
-- Un examen completo por visita; una vez guardado no se modifica (sólo inserción).
CREATE TABLE IF NOT EXISTS periodontogramas (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  paciente_id     UUID NOT NULL REFERENCES pacientes(id),
  observaciones   TEXT,
  registrado_por  UUID NOT NULL REFERENCES usuarios(id),
  registrado_en   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS periodontogramas_paciente ON periodontogramas (paciente_id, registrado_en DESC);

CREATE TABLE IF NOT EXISTS perio_dientes (
  periodontograma_id UUID NOT NULL REFERENCES periodontogramas(id),
  diente             SMALLINT NOT NULL CHECK (diente / 10 BETWEEN 1 AND 4 AND diente % 10 BETWEEN 1 AND 8),
  ausente            BOOLEAN NOT NULL DEFAULT FALSE,
  movilidad          SMALLINT CHECK (movilidad BETWEEN 0 AND 3),   -- grados de Miller
  furca              SMALLINT CHECK (furca BETWEEN 0 AND 3),       -- clases de Glickman simplificadas
  PRIMARY KEY (periodontograma_id, diente)
);

-- Seis sitios por diente. Convención del margen gingival (MG), rotulada también en la interfaz:
--   positivo = margen APICAL a la línea amelocementaria (recesión); negativo = CORONAL.
-- NIC (nivel de inserción clínica) = profundidad de sondaje + margen gingival. Lo calcula la BD.
CREATE TABLE IF NOT EXISTS perio_sitios (
  periodontograma_id UUID NOT NULL REFERENCES periodontogramas(id),
  diente             SMALLINT NOT NULL,
  sitio              TEXT NOT NULL CHECK (sitio IN ('MV','V','DV','ML','L','DL')),
  profundidad        SMALLINT NOT NULL CHECK (profundidad BETWEEN 0 AND 15),
  margen             SMALLINT NOT NULL CHECK (margen BETWEEN -10 AND 15),
  nic                SMALLINT GENERATED ALWAYS AS (profundidad + margen) STORED,
  sangrado           BOOLEAN NOT NULL DEFAULT FALSE,
  supuracion         BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (periodontograma_id, diente, sitio),
  FOREIGN KEY (periodontograma_id, diente) REFERENCES perio_dientes (periodontograma_id, diente)
);

-- --------------------------------------------------------------- Notas SOEP (RF-05)
-- bloqueo_en se calcula al crear la nota con el parámetro vigente (clinica.horasBloqueoNotas):
-- cambiar el parámetro después no reabre notas antiguas.
CREATE TABLE IF NOT EXISTS notas_soep (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  paciente_id      UUID NOT NULL REFERENCES pacientes(id),
  cita_id          UUID REFERENCES citas(id),
  autor_id         UUID NOT NULL REFERENCES usuarios(id),
  subjetivo        TEXT NOT NULL,
  objetivo         TEXT NOT NULL,
  evaluacion       TEXT NOT NULL,
  plan             TEXT NOT NULL,
  creado_en        TIMESTAMPTZ NOT NULL DEFAULT now(),
  actualizado_en   TIMESTAMPTZ NOT NULL DEFAULT now(),
  bloqueo_en       TIMESTAMPTZ NOT NULL,
  -- Firma electrónica simple: re-autenticación (código MFA) + hash SHA-256 del contenido.
  -- NO es una firma digital con certificado X.509.
  firmada_en       TIMESTAMPTZ,
  firmada_por      UUID REFERENCES usuarios(id),
  hash_firma       TEXT,
  CONSTRAINT notas_firma_completa CHECK ((firmada_en IS NULL) = (firmada_por IS NULL) AND (firmada_en IS NULL) = (hash_firma IS NULL)),
  CONSTRAINT notas_bloqueo_posterior CHECK (bloqueo_en > creado_en)
);
CREATE INDEX IF NOT EXISTS notas_soep_paciente ON notas_soep (paciente_id, creado_en DESC);

-- Adendas: la única forma de corregir o completar una nota firmada o bloqueada (sólo inserción).
CREATE TABLE IF NOT EXISTS notas_adendas (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  nota_id      UUID NOT NULL REFERENCES notas_soep(id),
  autor_id     UUID NOT NULL REFERENCES usuarios(id),
  texto        TEXT NOT NULL,
  creado_en    TIMESTAMPTZ NOT NULL DEFAULT now(),
  hash_firma   TEXT NOT NULL
);

-- --------------------------------------------------------------- Procedimientos realizados (RN-01)
CREATE TABLE IF NOT EXISTS procedimientos_realizados (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  paciente_id       UUID NOT NULL REFERENCES pacientes(id),
  procedimiento_id  UUID NOT NULL REFERENCES procedimientos(id),
  cita_id           UUID REFERENCES citas(id),
  diente            SMALLINT,
  superficies       TEXT[] NOT NULL DEFAULT '{}',
  estado            TEXT NOT NULL DEFAULT 'REALIZADO' CHECK (estado = 'REALIZADO'),
  observaciones     TEXT,
  odontologo_id     UUID NOT NULL REFERENCES usuarios(id),
  realizado_en      TIMESTAMPTZ NOT NULL DEFAULT now(),
  hash_firma        TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS procedimientos_realizados_paciente ON procedimientos_realizados (paciente_id, realizado_en DESC);

CREATE TABLE IF NOT EXISTS procedimientos_adendas (
  id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  procedimiento_realizado_id  UUID NOT NULL REFERENCES procedimientos_realizados(id),
  autor_id                    UUID NOT NULL REFERENCES usuarios(id),
  texto                       TEXT NOT NULL,
  creado_en                   TIMESTAMPTZ NOT NULL DEFAULT now(),
  hash_firma                  TEXT NOT NULL
);

-- --------------------------------------------------------------- Reglas en la BD
-- Nota SOEP: nunca se borra. Firmada ⇒ no cambia nada. Vencido el plazo ⇒ el contenido no cambia;
-- sólo se admite registrar la firma (sin tocar el texto).
CREATE OR REPLACE FUNCTION sigo_nota_soep_guardia() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'notas_soep no admite borrado; use una adenda' USING ERRCODE = 'P0001';
  END IF;
  IF OLD.firmada_en IS NOT NULL THEN
    RAISE EXCEPTION 'nota firmada: no se puede modificar; use una adenda' USING ERRCODE = 'P0001';
  END IF;
  IF NEW.paciente_id <> OLD.paciente_id OR NEW.autor_id <> OLD.autor_id OR NEW.creado_en <> OLD.creado_en
     OR NEW.bloqueo_en <> OLD.bloqueo_en OR NEW.cita_id IS DISTINCT FROM OLD.cita_id THEN
    RAISE EXCEPTION 'nota: paciente, autor, fechas y cita no se modifican' USING ERRCODE = 'P0001';
  END IF;
  IF now() >= OLD.bloqueo_en AND (NEW.subjetivo <> OLD.subjetivo OR NEW.objetivo <> OLD.objetivo
     OR NEW.evaluacion <> OLD.evaluacion OR NEW.plan <> OLD.plan) THEN
    RAISE EXCEPTION 'nota bloqueada: vencio el plazo de edicion; use una adenda' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS notas_soep_guardia ON notas_soep;
CREATE TRIGGER notas_soep_guardia BEFORE UPDATE OR DELETE ON notas_soep
  FOR EACH ROW EXECUTE FUNCTION sigo_nota_soep_guardia();

-- RN-01: un procedimiento realizado no se borra ni se edita (correcciones = adendas firmadas).
DROP TRIGGER IF EXISTS procedimientos_realizados_inmutables ON procedimientos_realizados;
CREATE TRIGGER procedimientos_realizados_inmutables BEFORE UPDATE OR DELETE ON procedimientos_realizados
  FOR EACH ROW EXECUTE FUNCTION sigo_solo_insercion();

DO $$
DECLARE t TEXT;
BEGIN
  FOREACH t IN ARRAY ARRAY['odontograma_eventos','periodontogramas','perio_dientes','perio_sitios','notas_adendas','procedimientos_adendas'] LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS %I_solo_insercion ON %I', t, t);
    EXECUTE format('CREATE TRIGGER %I_solo_insercion BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION sigo_solo_insercion()', t, t);
  END LOOP;
END $$;

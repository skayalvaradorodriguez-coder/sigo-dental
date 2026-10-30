-- ============================================================================
-- SIGO-Dental · Migración 002 — Etapa 2: catálogos, pacientes, anamnesis, agenda
-- y notificaciones. Idempotente.
-- ============================================================================

-- Necesaria para la restricción EXCLUDE que impide citas superpuestas por sillón (RN-03).
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- --------------------------------------------------------------- Catálogos
CREATE TABLE IF NOT EXISTS sedes (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  nombre     TEXT NOT NULL UNIQUE,
  direccion  TEXT,
  activo     BOOLEAN NOT NULL DEFAULT TRUE,
  creado_en  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS consultorios (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sede_id    UUID NOT NULL REFERENCES sedes(id),
  nombre     TEXT NOT NULL,
  activo     BOOLEAN NOT NULL DEFAULT TRUE,
  creado_en  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (sede_id, nombre)
);

-- Sillón dental físico: la unidad que no puede tener dos citas a la vez.
CREATE TABLE IF NOT EXISTS sillones (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  consultorio_id  UUID NOT NULL REFERENCES consultorios(id),
  nombre          TEXT NOT NULL,
  activo          BOOLEAN NOT NULL DEFAULT TRUE,
  creado_en       TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (consultorio_id, nombre)
);

CREATE TABLE IF NOT EXISTS procedimientos (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  codigo            TEXT NOT NULL UNIQUE,
  nombre            TEXT NOT NULL,
  duracion_min      SMALLINT NOT NULL CHECK (duracion_min BETWEEN 5 AND 480),
  arancel           NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (arancel >= 0), -- NUMERIC: sin errores de redondeo
  activo            BOOLEAN NOT NULL DEFAULT TRUE,
  creado_en         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS convenios (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  nombre       TEXT NOT NULL UNIQUE,
  aseguradora  TEXT,
  activo       BOOLEAN NOT NULL DEFAULT TRUE,
  creado_en    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- --------------------------------------------------------------- Pacientes (RF-01)
CREATE TABLE IF NOT EXISTS pacientes (
  id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tipo_documento             TEXT NOT NULL CHECK (tipo_documento IN ('CEDULA','PASAPORTE','OTRO')),
  numero_documento           TEXT NOT NULL,
  nombres                    TEXT NOT NULL,
  apellidos                  TEXT NOT NULL,
  fecha_nacimiento           DATE NOT NULL,
  sexo                       TEXT NOT NULL CHECK (sexo IN ('FEMENINO','MASCULINO','OTRO','NO_ESPECIFICA')),
  telefono                   TEXT,
  email                      TEXT,
  direccion                  TEXT,
  emergencia_nombre          TEXT NOT NULL,
  emergencia_telefono        TEXT NOT NULL,
  emergencia_parentesco      TEXT,
  canal_preferido            TEXT NOT NULL DEFAULT 'WHATSAPP' CHECK (canal_preferido IN ('WHATSAPP','SMS','EMAIL')),
  autoriza_mensajes          BOOLEAN NOT NULL DEFAULT FALSE,  -- requisito para recordatorios (S-08)
  convenio_id                UUID REFERENCES convenios(id),
  numero_afiliado            TEXT,
  creado_por                 UUID REFERENCES usuarios(id),
  creado_en                  TIMESTAMPTZ NOT NULL DEFAULT now(),
  actualizado_en             TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tipo_documento, numero_documento)
);
CREATE INDEX IF NOT EXISTS pacientes_apellidos_idx ON pacientes(apellidos);

-- Tutor legal: obligatorio cuando el paciente es menor de edad (validado en la API).
CREATE TABLE IF NOT EXISTS tutores (
  paciente_id       UUID PRIMARY KEY REFERENCES pacientes(id),
  nombre            TEXT NOT NULL,
  tipo_documento    TEXT NOT NULL CHECK (tipo_documento IN ('CEDULA','PASAPORTE','OTRO')),
  numero_documento  TEXT NOT NULL,
  parentesco        TEXT NOT NULL,
  telefono          TEXT NOT NULL,
  email             TEXT,
  actualizado_en    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Anamnesis versionada (RF-02): cada cambio es una versión nueva; las anteriores no cambian.
CREATE TABLE IF NOT EXISTS anamnesis (
  id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  paciente_id                UUID NOT NULL REFERENCES pacientes(id),
  version                    INTEGER NOT NULL,
  enfermedades_sistemicas    JSONB NOT NULL,   -- ["Hipertensión", ...]; vacío = niega
  alergias                   JSONB NOT NULL,   -- [{sustancia, tipo, reaccion}]; vacío = niega
  antecedentes_quirurgicos   TEXT,
  coagulopatia               BOOLEAN NOT NULL,
  anticoagulado              BOOLEAN NOT NULL,
  medicacion_actual          TEXT,
  embarazo                   BOOLEAN,
  otra_condicion_critica     TEXT,
  observaciones              TEXT,
  registrado_por             UUID NOT NULL REFERENCES usuarios(id),
  registrado_en              TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (paciente_id, version)
);

-- --------------------------------------------------------------- Agenda (RF-09, RF-10)
CREATE TABLE IF NOT EXISTS citas (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  paciente_id       UUID NOT NULL REFERENCES pacientes(id),
  odontologo_id     UUID NOT NULL REFERENCES usuarios(id),
  sillon_id         UUID NOT NULL REFERENCES sillones(id),
  procedimiento_id  UUID REFERENCES procedimientos(id),
  inicio            TIMESTAMPTZ NOT NULL,
  fin               TIMESTAMPTZ NOT NULL,
  estado            TEXT NOT NULL DEFAULT 'PROGRAMADO'
                    CHECK (estado IN ('PROGRAMADO','CONFIRMADO','EN_SALA_ESPERA','EN_ATENCION','ATENDIDO','CANCELADO','NO_ASISTIO')),
  motivo            TEXT,
  motivo_cancelacion TEXT,
  creado_por        UUID NOT NULL REFERENCES usuarios(id),
  creado_en         TIMESTAMPTZ NOT NULL DEFAULT now(),
  actualizado_en    TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT citas_rango_valido CHECK (fin > inicio)
);
CREATE INDEX IF NOT EXISTS citas_inicio_idx ON citas(inicio);
CREATE INDEX IF NOT EXISTS citas_sillon_idx ON citas(sillon_id, inicio);
CREATE INDEX IF NOT EXISTS citas_odontologo_idx ON citas(odontologo_id, inicio);
CREATE INDEX IF NOT EXISTS citas_paciente_idx ON citas(paciente_id);

-- RN-03: dos citas activas no pueden superponerse en el mismo sillón, aunque los odontólogos
-- sean distintos. La evalúa PostgreSQL: también resuelve peticiones simultáneas (error 23P01).
-- '[)' permite citas consecutivas (10:00-10:30 y 10:30-11:00). Cancelado/No asistió liberan el sillón.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'citas_sin_superposicion_por_sillon') THEN
    ALTER TABLE citas ADD CONSTRAINT citas_sin_superposicion_por_sillon
      EXCLUDE USING gist (sillon_id WITH =, tstzrange(inicio, fin, '[)') WITH &&)
      WHERE (estado NOT IN ('CANCELADO', 'NO_ASISTIO'));
  END IF;
END $$;

-- Historial de estados de cada cita (sólo inserción).
CREATE TABLE IF NOT EXISTS citas_eventos (
  id               BIGSERIAL PRIMARY KEY,
  cita_id          UUID NOT NULL REFERENCES citas(id),
  estado_anterior  TEXT,
  estado_nuevo     TEXT NOT NULL,
  nota             TEXT,
  usuario_id       UUID NOT NULL REFERENCES usuarios(id),
  ocurrido_en      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS citas_eventos_cita_idx ON citas_eventos(cita_id);

-- --------------------------------------------------------------- Notificaciones (RF-11)
-- `simulado` marca lo que NO salió a un proveedor real.
CREATE TABLE IF NOT EXISTS notificaciones (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cita_id             UUID NOT NULL REFERENCES citas(id),
  tipo                TEXT NOT NULL CHECK (tipo IN ('CONFIRMACION','RECORDATORIO_48H','RECORDATORIO_24H')),
  canal               TEXT NOT NULL CHECK (canal IN ('WHATSAPP','SMS','EMAIL')),
  destinatario        TEXT,
  programada_para     TIMESTAMPTZ NOT NULL,
  estado              TEXT NOT NULL DEFAULT 'PENDIENTE' CHECK (estado IN ('PENDIENTE','ENVIADO','FALLIDO','CANCELADO','OMITIDO')),
  motivo              TEXT,
  proveedor           TEXT,
  simulado            BOOLEAN,
  mensaje             TEXT,
  id_proveedor        TEXT,
  intentos            SMALLINT NOT NULL DEFAULT 0,
  enviada_en          TIMESTAMPTZ,
  creada_en           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS notificaciones_pendientes_idx ON notificaciones(estado, programada_para);
CREATE INDEX IF NOT EXISTS notificaciones_cita_idx ON notificaciones(cita_id);

-- --------------------------------------------------------------- Reglas en la BD
CREATE OR REPLACE FUNCTION sigo_solo_insercion() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION '% es de solo insercion: operacion % no permitida', TG_TABLE_NAME, TG_OP USING ERRCODE = 'P0001';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION sigo_sin_borrado() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION '% no admite borrado; use el cambio de estado', TG_TABLE_NAME USING ERRCODE = 'P0001';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS anamnesis_solo_insercion ON anamnesis;
CREATE TRIGGER anamnesis_solo_insercion BEFORE UPDATE OR DELETE ON anamnesis
  FOR EACH ROW EXECUTE FUNCTION sigo_solo_insercion();
DROP TRIGGER IF EXISTS citas_eventos_solo_insercion ON citas_eventos;
CREATE TRIGGER citas_eventos_solo_insercion BEFORE UPDATE OR DELETE ON citas_eventos
  FOR EACH ROW EXECUTE FUNCTION sigo_solo_insercion();
DROP TRIGGER IF EXISTS citas_sin_borrado ON citas;
CREATE TRIGGER citas_sin_borrado BEFORE DELETE ON citas
  FOR EACH ROW EXECUTE FUNCTION sigo_sin_borrado();
DROP TRIGGER IF EXISTS pacientes_sin_borrado ON pacientes;
CREATE TRIGGER pacientes_sin_borrado BEFORE DELETE ON pacientes
  FOR EACH ROW EXECUTE FUNCTION sigo_sin_borrado();

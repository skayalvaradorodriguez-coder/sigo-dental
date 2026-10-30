-- ============================================================================
-- SIGO-Dental · Migración 001 — Etapa 1: identidad, sesiones, ajustes y auditoría
-- Idempotente: se puede ejecutar varias veces sin error.
-- ============================================================================

CREATE TABLE IF NOT EXISTS usuarios (
  id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email                  TEXT NOT NULL UNIQUE,
  nombre                 TEXT NOT NULL,
  rol                    TEXT NOT NULL CHECK (rol IN ('ADMIN_TI','RECEPCIONISTA','ODONTOLOGO','ASISTENTE','AUDITOR')),
  registro_profesional   TEXT,                       -- odontólogos: se muestra al firmar notas
  password_hash          TEXT NOT NULL,              -- bcrypt
  debe_cambiar_password  BOOLEAN NOT NULL DEFAULT TRUE,
  activo                 BOOLEAN NOT NULL DEFAULT TRUE,
  mfa_secreto            TEXT,                       -- TOTP Base32 (en producción: cifrar con KMS)
  mfa_activo             BOOLEAN NOT NULL DEFAULT FALSE,
  mfa_ultimo_paso        BIGINT,                     -- anti-replay del código TOTP
  intentos_fallidos      INTEGER NOT NULL DEFAULT 0,
  bloqueado_hasta        TIMESTAMPTZ,
  ultimo_acceso          TIMESTAMPTZ,
  creado_en              TIMESTAMPTZ NOT NULL DEFAULT now(),
  actualizado_en         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sesión de servidor asociada a cada JWT (por su `jti`). Permite revocar al instante,
-- exigir MFA y cerrar por inactividad (RNF-05) aunque el token aún no haya vencido.
CREATE TABLE IF NOT EXISTS sesiones (
  jti                TEXT PRIMARY KEY,
  usuario_id         UUID NOT NULL REFERENCES usuarios(id),
  mfa_verificado     BOOLEAN NOT NULL DEFAULT FALSE,
  ip                 TEXT,
  user_agent         TEXT,
  creada_en          TIMESTAMPTZ NOT NULL DEFAULT now(),
  ultima_actividad   TIMESTAMPTZ NOT NULL DEFAULT now(),
  expira_en          TIMESTAMPTZ NOT NULL,
  revocada_en        TIMESTAMPTZ,
  motivo_revocacion  TEXT
);
CREATE INDEX IF NOT EXISTS sesiones_usuario_idx ON sesiones(usuario_id);

-- Parámetros configurables (p. ej. minutos de inactividad, horas de bloqueo de notas SOEP).
CREATE TABLE IF NOT EXISTS ajustes (
  clave            TEXT PRIMARY KEY,
  valor            JSONB NOT NULL,
  actualizado_en   TIMESTAMPTZ NOT NULL DEFAULT now(),
  actualizado_por  UUID REFERENCES usuarios(id)
);

-- Bitácora de auditoría (RNF-06): quién, qué, cuándo (ISO-8601) y desde qué IP.
-- Cada fila encadena el hash de la anterior para detectar manipulación.
CREATE TABLE IF NOT EXISTS auditoria (
  id             BIGSERIAL PRIMARY KEY,
  ocurrido_en    TIMESTAMPTZ NOT NULL,
  usuario_id     UUID REFERENCES usuarios(id),
  accion         TEXT NOT NULL,
  entidad        TEXT NOT NULL,
  entidad_id     TEXT,
  ip             TEXT,
  user_agent     TEXT,
  detalle        JSONB,
  hash_anterior  TEXT NOT NULL,
  hash           TEXT NOT NULL UNIQUE
);
CREATE INDEX IF NOT EXISTS auditoria_entidad_idx ON auditoria(entidad, entidad_id);
CREATE INDEX IF NOT EXISTS auditoria_usuario_idx ON auditoria(usuario_id);
CREATE INDEX IF NOT EXISTS auditoria_fecha_idx ON auditoria(ocurrido_en);

-- La bitácora sólo admite INSERT: UPDATE, DELETE y TRUNCATE se rechazan en la propia BD.
CREATE OR REPLACE FUNCTION sigo_auditoria_inmutable() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'auditoria es inmutable: operacion % no permitida', TG_OP USING ERRCODE = 'P0001';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS auditoria_sin_update ON auditoria;
CREATE TRIGGER auditoria_sin_update BEFORE UPDATE OR DELETE ON auditoria
  FOR EACH ROW EXECUTE FUNCTION sigo_auditoria_inmutable();
DROP TRIGGER IF EXISTS auditoria_sin_truncate ON auditoria;
CREATE TRIGGER auditoria_sin_truncate BEFORE TRUNCATE ON auditoria
  FOR EACH STATEMENT EXECUTE FUNCTION sigo_auditoria_inmutable();

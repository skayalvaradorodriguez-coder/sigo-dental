-- Etapa 5 — Caja y facturación (RF-12, RN-02). Montos en NUMERIC(12,2), nunca en float.
-- Idempotente: puede ejecutarse varias veces.

-- Sesión de caja de un recepcionista. Al cerrar se guardan el conteo declarado y, en la misma
-- transacción, el teórico calculado por el sistema. El recepcionista nunca recibe el teórico (RN-02).
CREATE TABLE IF NOT EXISTS caja_sesiones (
  id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cajero_id               UUID NOT NULL REFERENCES usuarios(id),
  abierta_en              TIMESTAMPTZ NOT NULL DEFAULT now(),
  fondo_inicial           NUMERIC(12,2) NOT NULL CHECK (fondo_inicial >= 0),
  estado                  TEXT NOT NULL DEFAULT 'ABIERTA' CHECK (estado IN ('ABIERTA','CERRADA','REVISADA')),
  cerrada_en              TIMESTAMPTZ,
  declarado_efectivo      NUMERIC(12,2) CHECK (declarado_efectivo >= 0),
  declarado_tarjeta       NUMERIC(12,2) CHECK (declarado_tarjeta >= 0),
  declarado_transferencia NUMERIC(12,2) CHECK (declarado_transferencia >= 0),
  teorico_efectivo        NUMERIC(12,2),
  teorico_tarjeta         NUMERIC(12,2),
  teorico_transferencia   NUMERIC(12,2),
  nota_cierre             TEXT,
  revisada_por            UUID REFERENCES usuarios(id),
  revisada_en             TIMESTAMPTZ,
  nota_revision           TEXT,
  CONSTRAINT caja_cierre_completo CHECK (
    (estado = 'ABIERTA' AND cerrada_en IS NULL AND declarado_efectivo IS NULL AND teorico_efectivo IS NULL)
    OR (estado <> 'ABIERTA' AND cerrada_en IS NOT NULL AND declarado_efectivo IS NOT NULL AND declarado_tarjeta IS NOT NULL
        AND declarado_transferencia IS NOT NULL AND teorico_efectivo IS NOT NULL AND teorico_tarjeta IS NOT NULL
        AND teorico_transferencia IS NOT NULL)),
  CONSTRAINT caja_revision_completa CHECK ((estado = 'REVISADA') = (revisada_por IS NOT NULL AND revisada_en IS NOT NULL))
);
-- Una sola caja abierta por recepcionista.
CREATE UNIQUE INDEX IF NOT EXISTS caja_una_abierta ON caja_sesiones (cajero_id) WHERE estado = 'ABIERTA';
CREATE INDEX IF NOT EXISTS caja_sesiones_estado ON caja_sesiones (estado, cerrada_en DESC);

-- Numeración interna de recibos y secuencias de comprobantes (por tipo).
CREATE SEQUENCE IF NOT EXISTS cobros_numero;
CREATE SEQUENCE IF NOT EXISTS comprobantes_factura;
CREATE SEQUENCE IF NOT EXISTS comprobantes_nota_credito;

CREATE TABLE IF NOT EXISTS cobros (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  numero          BIGINT NOT NULL UNIQUE DEFAULT nextval('cobros_numero'),
  sesion_id       UUID NOT NULL REFERENCES caja_sesiones(id),
  paciente_id     UUID NOT NULL REFERENCES pacientes(id),
  total           NUMERIC(12,2) NOT NULL CHECK (total > 0),
  registrado_por  UUID NOT NULL REFERENCES usuarios(id),
  registrado_en   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS cobros_sesion ON cobros (sesion_id, registrado_en);
CREATE INDEX IF NOT EXISTS cobros_paciente ON cobros (paciente_id, registrado_en DESC);

-- Qué se cobra: un procedimiento realizado (con el monto del presupuesto aprobado o el arancel)
-- o un procedimiento del catálogo a su arancel (p. ej. una consulta).
CREATE TABLE IF NOT EXISTS cobro_conceptos (
  id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cobro_id                    UUID NOT NULL REFERENCES cobros(id),
  procedimiento_realizado_id  UUID REFERENCES procedimientos_realizados(id),
  procedimiento_id            UUID NOT NULL REFERENCES procedimientos(id),
  descripcion                 TEXT NOT NULL,
  monto                       NUMERIC(12,2) NOT NULL CHECK (monto > 0)
);
CREATE INDEX IF NOT EXISTS cobro_conceptos_cobro ON cobro_conceptos (cobro_id);
CREATE INDEX IF NOT EXISTS cobro_conceptos_realizado ON cobro_conceptos (procedimiento_realizado_id);

-- Pago mixto (RF-12): una línea por medio. El seguro genera una cuenta por cobrar a la aseguradora (S-06).
CREATE TABLE IF NOT EXISTS cobro_pagos (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cobro_id     UUID NOT NULL REFERENCES cobros(id),
  medio        TEXT NOT NULL CHECK (medio IN ('EFECTIVO','TARJETA','TRANSFERENCIA','SEGURO')),
  monto        NUMERIC(12,2) NOT NULL CHECK (monto > 0),
  referencia   TEXT,
  convenio_id  UUID REFERENCES convenios(id),
  CONSTRAINT cobro_pago_seguro CHECK ((medio = 'SEGURO') = (convenio_id IS NOT NULL)),
  CONSTRAINT cobro_pago_referencia CHECK (medio NOT IN ('TARJETA','TRANSFERENCIA') OR length(trim(referencia)) > 0)
);
CREATE INDEX IF NOT EXISTS cobro_pagos_cobro ON cobro_pagos (cobro_id);

-- Anulación: el cobro nunca se borra; queda esta marca con su motivo.
CREATE TABLE IF NOT EXISTS cobros_anulaciones (
  cobro_id     UUID PRIMARY KEY REFERENCES cobros(id),
  motivo       TEXT NOT NULL CHECK (length(trim(motivo)) >= 5),
  anulado_por  UUID NOT NULL REFERENCES usuarios(id),
  anulado_en   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Cada intento de emisión es una fila (sólo inserción). Vigente = el último por cobro y tipo.
CREATE TABLE IF NOT EXISTS comprobantes (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cobro_id       UUID NOT NULL REFERENCES cobros(id),
  tipo           TEXT NOT NULL CHECK (tipo IN ('FACTURA','NOTA_CREDITO')),
  estado         TEXT NOT NULL CHECK (estado IN ('AUTORIZADO','RECHAZADO','ERROR')),
  secuencial     BIGINT,
  numero         TEXT,
  autorizacion   TEXT,
  proveedor      TEXT NOT NULL,
  simulado       BOOLEAN NOT NULL,
  mensaje        TEXT,
  documento      JSONB NOT NULL,          -- lo que se envió al proveedor
  hash_documento TEXT NOT NULL,
  emitido_en     TIMESTAMPTZ NOT NULL DEFAULT now(),
  emitido_por    UUID NOT NULL REFERENCES usuarios(id),
  CONSTRAINT comprobante_autorizado CHECK (estado <> 'AUTORIZADO' OR (numero IS NOT NULL AND autorizacion IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS comprobantes_un_autorizado ON comprobantes (cobro_id, tipo) WHERE estado = 'AUTORIZADO';
CREATE INDEX IF NOT EXISTS comprobantes_cobro ON comprobantes (cobro_id, emitido_en);

-- ------------------------------------------------------------------------------- Guardias

-- Sesión: ABIERTA → CERRADA (sólo datos de cierre) → REVISADA (sólo datos de revisión); nunca se borra.
CREATE OR REPLACE FUNCTION sigo_caja_sesion_guardia() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'caja_sesiones no admite borrado' USING ERRCODE = 'P0001';
  END IF;
  IF NEW.cajero_id <> OLD.cajero_id OR NEW.abierta_en <> OLD.abierta_en OR NEW.fondo_inicial <> OLD.fondo_inicial THEN
    RAISE EXCEPTION 'caja: cajero, apertura y fondo no cambian' USING ERRCODE = 'P0001';
  END IF;
  IF OLD.estado = 'REVISADA' THEN
    RAISE EXCEPTION 'caja revisada: no se modifica' USING ERRCODE = 'P0001';
  END IF;
  IF OLD.estado = 'CERRADA' AND (NEW.estado <> 'REVISADA'
     OR (NEW.cerrada_en, NEW.declarado_efectivo, NEW.declarado_tarjeta, NEW.declarado_transferencia,
         NEW.teorico_efectivo, NEW.teorico_tarjeta, NEW.teorico_transferencia, NEW.nota_cierre)
        IS DISTINCT FROM
        (OLD.cerrada_en, OLD.declarado_efectivo, OLD.declarado_tarjeta, OLD.declarado_transferencia,
         OLD.teorico_efectivo, OLD.teorico_tarjeta, OLD.teorico_transferencia, OLD.nota_cierre)) THEN
    RAISE EXCEPTION 'caja cerrada: sólo se registra la revisión' USING ERRCODE = 'P0001';
  END IF;
  IF OLD.estado = 'ABIERTA' AND NEW.estado NOT IN ('ABIERTA','CERRADA') THEN
    RAISE EXCEPTION 'caja abierta: primero se cierra' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS caja_sesiones_guardia ON caja_sesiones;
CREATE TRIGGER caja_sesiones_guardia BEFORE UPDATE OR DELETE ON caja_sesiones
  FOR EACH ROW EXECUTE FUNCTION sigo_caja_sesion_guardia();

-- Un cobro (o una anulación) sólo entra en una caja ABIERTA. FOR SHARE espera a un cierre en curso
-- (que toma FOR UPDATE): así ningún cobro queda fuera del teórico calculado al cerrar.
CREATE OR REPLACE FUNCTION sigo_cobro_en_caja_abierta() RETURNS trigger AS $$
DECLARE est TEXT;
BEGIN
  IF TG_TABLE_NAME = 'cobros' THEN
    SELECT estado INTO est FROM caja_sesiones WHERE id = NEW.sesion_id FOR SHARE;
  ELSE
    SELECT s.estado INTO est FROM caja_sesiones s JOIN cobros c ON c.sesion_id = s.id WHERE c.id = NEW.cobro_id FOR SHARE OF s;
  END IF;
  IF est IS DISTINCT FROM 'ABIERTA' THEN
    RAISE EXCEPTION 'la caja no está abierta' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS cobros_caja_abierta ON cobros;
CREATE TRIGGER cobros_caja_abierta BEFORE INSERT ON cobros FOR EACH ROW EXECUTE FUNCTION sigo_cobro_en_caja_abierta();
DROP TRIGGER IF EXISTS cobros_anulaciones_caja_abierta ON cobros_anulaciones;
CREATE TRIGGER cobros_anulaciones_caja_abierta BEFORE INSERT ON cobros_anulaciones FOR EACH ROW EXECUTE FUNCTION sigo_cobro_en_caja_abierta();

-- Al confirmar la transacción: conceptos = pagos = total del cobro (al centavo).
CREATE OR REPLACE FUNCTION sigo_cobro_cuadra() RETURNS trigger AS $$
DECLARE c NUMERIC; p NUMERIC;
BEGIN
  SELECT COALESCE(sum(monto), 0) INTO c FROM cobro_conceptos WHERE cobro_id = NEW.id;
  SELECT COALESCE(sum(monto), 0) INTO p FROM cobro_pagos WHERE cobro_id = NEW.id;
  IF c <> NEW.total OR p <> NEW.total THEN
    RAISE EXCEPTION 'cobro %: conceptos (%) y pagos (%) deben sumar el total (%)', NEW.numero, c, p, NEW.total USING ERRCODE = 'P0001';
  END IF;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS cobros_cuadra ON cobros;
CREATE CONSTRAINT TRIGGER cobros_cuadra AFTER INSERT ON cobros DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION sigo_cobro_cuadra();

-- Un procedimiento realizado se cobra una sola vez (salvo que el cobro anterior se haya anulado).
CREATE OR REPLACE FUNCTION sigo_realizado_sin_doble_cobro() RETURNS trigger AS $$
BEGIN
  IF NEW.procedimiento_realizado_id IS NOT NULL THEN
    PERFORM 1 FROM procedimientos_realizados WHERE id = NEW.procedimiento_realizado_id FOR UPDATE;
    IF EXISTS (SELECT 1 FROM cobro_conceptos cc
               WHERE cc.procedimiento_realizado_id = NEW.procedimiento_realizado_id AND cc.id <> NEW.id
                 AND NOT EXISTS (SELECT 1 FROM cobros_anulaciones a WHERE a.cobro_id = cc.cobro_id)) THEN
      RAISE EXCEPTION 'el procedimiento realizado % ya fue cobrado', NEW.procedimiento_realizado_id USING ERRCODE = '23505';
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS cobro_conceptos_sin_doble ON cobro_conceptos;
CREATE TRIGGER cobro_conceptos_sin_doble BEFORE INSERT ON cobro_conceptos
  FOR EACH ROW EXECUTE FUNCTION sigo_realizado_sin_doble_cobro();

DO $$
DECLARE t TEXT;
BEGIN
  FOREACH t IN ARRAY ARRAY['cobros','cobro_conceptos','cobro_pagos','cobros_anulaciones','comprobantes'] LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS %I_solo_insercion ON %I', t, t);
    EXECUTE format('CREATE TRIGGER %I_solo_insercion BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION sigo_solo_insercion()', t, t);
  END LOOP;
END $$;

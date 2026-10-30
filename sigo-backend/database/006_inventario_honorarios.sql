-- Etapa 6 — Inventario (RF-14) y honorarios (RF-13). Idempotente.

-- ------------------------------------------------------------------------- Inventario

-- Ubicaciones de stock: bodega o consultorio (A-14). Una es la de por defecto.
CREATE TABLE IF NOT EXISTS inventario_ubicaciones (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  nombre          TEXT NOT NULL UNIQUE,
  consultorio_id  UUID UNIQUE REFERENCES consultorios(id),
  por_defecto     BOOLEAN NOT NULL DEFAULT FALSE,
  activo          BOOLEAN NOT NULL DEFAULT TRUE,
  creado_en       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS inventario_una_por_defecto ON inventario_ubicaciones (por_defecto) WHERE por_defecto;

-- Insumos: el costo unitario admite 4 decimales (p. ej. costo por ml); stock mínimo para alertas.
CREATE TABLE IF NOT EXISTS insumos (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  codigo          TEXT NOT NULL UNIQUE,
  nombre          TEXT NOT NULL,
  unidad          TEXT NOT NULL,
  costo_unitario  NUMERIC(12,4) NOT NULL CHECK (costo_unitario >= 0),
  stock_minimo    NUMERIC(12,3) NOT NULL DEFAULT 0 CHECK (stock_minimo >= 0),
  activo          BOOLEAN NOT NULL DEFAULT TRUE,
  creado_en       TIMESTAMPTZ NOT NULL DEFAULT now(),
  actualizado_en  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Receta de consumo estándar por procedimiento (configuración; los cambios se auditan y no
-- alteran descargos pasados, que guardan su propia cantidad y costo).
CREATE TABLE IF NOT EXISTS recetas_consumo (
  procedimiento_id  UUID NOT NULL REFERENCES procedimientos(id),
  insumo_id         UUID NOT NULL REFERENCES insumos(id),
  cantidad          NUMERIC(12,3) NOT NULL CHECK (cantidad > 0),
  PRIMARY KEY (procedimiento_id, insumo_id)
);

-- Kardex: el stock es la suma de los movimientos (sólo inserción). Cantidad con signo.
CREATE TABLE IF NOT EXISTS movimientos_stock (
  id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  insumo_id                   UUID NOT NULL REFERENCES insumos(id),
  ubicacion_id                UUID NOT NULL REFERENCES inventario_ubicaciones(id),
  tipo                        TEXT NOT NULL CHECK (tipo IN ('ENTRADA','SALIDA_PROCEDIMIENTO','AJUSTE','TRANSFERENCIA')),
  cantidad                    NUMERIC(12,3) NOT NULL CHECK (cantidad <> 0),
  costo_unitario              NUMERIC(12,4) NOT NULL CHECK (costo_unitario >= 0),
  procedimiento_realizado_id  UUID REFERENCES procedimientos_realizados(id),
  transferencia_id            UUID,
  motivo                      TEXT,
  registrado_por              UUID NOT NULL REFERENCES usuarios(id),
  registrado_en               TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT movimiento_signo CHECK (
    (tipo = 'ENTRADA' AND cantidad > 0) OR (tipo = 'SALIDA_PROCEDIMIENTO' AND cantidad < 0) OR tipo IN ('AJUSTE','TRANSFERENCIA')),
  CONSTRAINT movimiento_procedimiento CHECK ((tipo = 'SALIDA_PROCEDIMIENTO') = (procedimiento_realizado_id IS NOT NULL)),
  CONSTRAINT movimiento_motivo CHECK (tipo NOT IN ('AJUSTE') OR length(trim(motivo)) >= 5),
  CONSTRAINT movimiento_transferencia CHECK ((tipo = 'TRANSFERENCIA') = (transferencia_id IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS movimientos_insumo ON movimientos_stock (insumo_id, registrado_en DESC);
CREATE INDEX IF NOT EXISTS movimientos_realizado ON movimientos_stock (procedimiento_realizado_id);

-- Las dos patas de una transferencia suman cero (al confirmar la transacción).
CREATE OR REPLACE FUNCTION sigo_transferencia_cuadra() RETURNS trigger AS $$
DECLARE s NUMERIC; n INT;
BEGIN
  IF NEW.transferencia_id IS NULL THEN RETURN NULL; END IF;
  SELECT sum(cantidad), count(*) INTO s, n FROM movimientos_stock WHERE transferencia_id = NEW.transferencia_id;
  IF s <> 0 OR n <> 2 THEN
    RAISE EXCEPTION 'transferencia %: debe tener una salida y una entrada por la misma cantidad', NEW.transferencia_id USING ERRCODE = 'P0001';
  END IF;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS movimientos_transferencia_cuadra ON movimientos_stock;
CREATE CONSTRAINT TRIGGER movimientos_transferencia_cuadra AFTER INSERT ON movimientos_stock DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION sigo_transferencia_cuadra();

DROP TRIGGER IF EXISTS movimientos_stock_solo_insercion ON movimientos_stock;
CREATE TRIGGER movimientos_stock_solo_insercion BEFORE UPDATE OR DELETE ON movimientos_stock
  FOR EACH ROW EXECUTE FUNCTION sigo_solo_insercion();
DROP TRIGGER IF EXISTS insumos_sin_borrado ON insumos;
CREATE TRIGGER insumos_sin_borrado BEFORE DELETE ON insumos FOR EACH ROW EXECUTE FUNCTION sigo_sin_borrado();

-- ------------------------------------------------------------------------- Honorarios

-- Regla de comisión (RF-13). Alcance: odontólogo y/o procedimiento (NULL = todos). Se aplica la más
-- específica vigente a la fecha del cobro. Sólo inserción: una regla se reemplaza con otra más nueva.
CREATE TABLE IF NOT EXISTS reglas_honorario (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  odontologo_id     UUID REFERENCES usuarios(id),
  procedimiento_id  UUID REFERENCES procedimientos(id),
  tipo              TEXT NOT NULL CHECK (tipo IN ('PORCENTAJE','MONTO_FIJO','PORCENTAJE_NETO')),
  valor             NUMERIC(12,2) NOT NULL CHECK (valor >= 0),
  vigente_desde     DATE NOT NULL,
  creado_por        UUID NOT NULL REFERENCES usuarios(id),
  creado_en         TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT regla_porcentaje CHECK (tipo = 'MONTO_FIJO' OR valor <= 100)
);
CREATE INDEX IF NOT EXISTS reglas_honorario_alcance ON reglas_honorario (odontologo_id, procedimiento_id, vigente_desde DESC);

-- Liquidación cerrada de un periodo: copia de cada línea con su cálculo. Un concepto cobrado se
-- liquida una sola vez.
CREATE TABLE IF NOT EXISTS liquidaciones_honorarios (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  odontologo_id  UUID NOT NULL REFERENCES usuarios(id),
  desde          DATE NOT NULL,
  hasta          DATE NOT NULL CHECK (hasta >= desde),
  total          NUMERIC(12,2) NOT NULL,
  creado_por     UUID NOT NULL REFERENCES usuarios(id),
  creado_en      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS liquidacion_lineas (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  liquidacion_id    UUID NOT NULL REFERENCES liquidaciones_honorarios(id),
  cobro_concepto_id UUID NOT NULL UNIQUE REFERENCES cobro_conceptos(id),
  procedimiento_realizado_id UUID NOT NULL REFERENCES procedimientos_realizados(id),
  descripcion       TEXT NOT NULL,
  cobrado_en        TIMESTAMPTZ NOT NULL,
  monto             NUMERIC(12,2) NOT NULL,
  laboratorio       NUMERIC(12,2) NOT NULL,
  materiales        NUMERIC(12,2) NOT NULL,
  regla_id          UUID REFERENCES reglas_honorario(id),
  regla             TEXT NOT NULL,
  honorario         NUMERIC(12,2) NOT NULL
);
CREATE INDEX IF NOT EXISTS liquidacion_lineas_liq ON liquidacion_lineas (liquidacion_id);

DO $$
DECLARE t TEXT;
BEGIN
  FOREACH t IN ARRAY ARRAY['reglas_honorario','liquidaciones_honorarios','liquidacion_lineas'] LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS %I_solo_insercion ON %I', t, t);
    EXECUTE format('CREATE TRIGGER %I_solo_insercion BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION sigo_solo_insercion()', t, t);
  END LOOP;
END $$;

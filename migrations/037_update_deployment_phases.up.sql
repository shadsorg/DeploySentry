-- Migration 029 (platform_redesign) dropped deployment_phases as part of the
-- platform redesign, then migration 059 recreates it with the final schema
-- this migration was originally meant to produce. On a fresh database the
-- table doesn't exist when this migration runs, so guard with a table check
-- to keep the migration idempotent across both upgrade paths:
--
--   * Existing DBs that ran 037 before 029 dropped the table → already applied.
--   * Fresh DBs running migrations from scratch → no-op; 059 builds the
--     final schema directly.
--
-- The schema_migrations row is still recorded so subsequent migrations
-- proceed in order.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = current_schema() AND table_name = 'deployment_phases'
  ) THEN
    ALTER TABLE deployment_phases
      ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '',
      ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT 0,
      ADD COLUMN IF NOT EXISTS auto_promote BOOLEAN NOT NULL DEFAULT true;

    -- Backfill sort_order from phase_number when present.
    IF EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = current_schema()
        AND table_name = 'deployment_phases'
        AND column_name = 'phase_number'
    ) THEN
      UPDATE deployment_phases SET sort_order = phase_number, name = 'phase-' || phase_number;
    END IF;

    -- Rename traffic_pct → traffic_percent if old column still present.
    IF EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = current_schema()
        AND table_name = 'deployment_phases'
        AND column_name = 'traffic_pct'
    ) THEN
      ALTER TABLE deployment_phases RENAME COLUMN traffic_pct TO traffic_percent;
    END IF;

    -- Rename duration_secs → duration_seconds if old column still present.
    IF EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = current_schema()
        AND table_name = 'deployment_phases'
        AND column_name = 'duration_secs'
    ) THEN
      ALTER TABLE deployment_phases RENAME COLUMN duration_secs TO duration_seconds;
    END IF;

    ALTER TABLE deployment_phases
      ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
      ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
  END IF;
END $$;

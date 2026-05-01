-- Guarded mirror of the up migration. By the time this down migration runs
-- in a full rollback, migration 059's down has already dropped the table,
-- so a no-op when the table is absent.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = current_schema() AND table_name = 'deployment_phases'
  ) THEN
    ALTER TABLE deployment_phases DROP COLUMN IF EXISTS created_at;
    ALTER TABLE deployment_phases DROP COLUMN IF EXISTS updated_at;

    IF EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = current_schema()
        AND table_name = 'deployment_phases'
        AND column_name = 'duration_seconds'
    ) THEN
      ALTER TABLE deployment_phases RENAME COLUMN duration_seconds TO duration_secs;
    END IF;
    IF EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = current_schema()
        AND table_name = 'deployment_phases'
        AND column_name = 'traffic_percent'
    ) THEN
      ALTER TABLE deployment_phases RENAME COLUMN traffic_percent TO traffic_pct;
    END IF;
    ALTER TABLE deployment_phases DROP COLUMN IF EXISTS auto_promote;
    ALTER TABLE deployment_phases DROP COLUMN IF EXISTS sort_order;
    ALTER TABLE deployment_phases DROP COLUMN IF EXISTS name;
  END IF;
END $$;

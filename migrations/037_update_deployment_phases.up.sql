ALTER TABLE deployment_phases
  ADD COLUMN name TEXT NOT NULL DEFAULT '',
  ADD COLUMN sort_order INT NOT NULL DEFAULT 0,
  ADD COLUMN auto_promote BOOLEAN NOT NULL DEFAULT true;

-- Backfill sort_order from phase_number
UPDATE deployment_phases SET sort_order = phase_number, name = 'phase-' || phase_number;

-- Rename traffic_pct to match Go model
ALTER TABLE deployment_phases RENAME COLUMN traffic_pct TO traffic_percent;

-- Rename duration_secs to match Go model
ALTER TABLE deployment_phases RENAME COLUMN duration_secs TO duration_seconds;

-- Add created_at/updated_at for tracking
ALTER TABLE deployment_phases
  ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

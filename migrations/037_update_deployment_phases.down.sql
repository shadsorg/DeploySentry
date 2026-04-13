ALTER TABLE deployment_phases DROP COLUMN IF EXISTS created_at;
ALTER TABLE deployment_phases DROP COLUMN IF EXISTS updated_at;
ALTER TABLE deployment_phases RENAME COLUMN duration_seconds TO duration_secs;
ALTER TABLE deployment_phases RENAME COLUMN traffic_percent TO traffic_pct;
ALTER TABLE deployment_phases DROP COLUMN IF EXISTS auto_promote;
ALTER TABLE deployment_phases DROP COLUMN IF EXISTS sort_order;
ALTER TABLE deployment_phases DROP COLUMN IF EXISTS name;

ALTER TABLE deployments
    ADD COLUMN mode   TEXT NOT NULL DEFAULT 'orchestrate',
    ADD COLUMN source TEXT;

ALTER TABLE deployments
    ADD CONSTRAINT deployments_mode_check CHECK (mode IN ('orchestrate', 'record'));

CREATE INDEX deployments_app_env_created_idx
    ON deployments (application_id, environment_id, created_at DESC);

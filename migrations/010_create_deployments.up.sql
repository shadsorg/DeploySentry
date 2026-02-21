CREATE TABLE deployments (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id  UUID        NOT NULL REFERENCES deploy_pipelines(id) ON DELETE CASCADE,
    release_id   UUID        NOT NULL,
    environment  TEXT        NOT NULL,
    status       TEXT        CHECK (status IN ('pending', 'in_progress', 'paused', 'promoting', 'completed', 'rolling_back', 'failed')),
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    initiated_by UUID        REFERENCES users(id) ON DELETE SET NULL,
    metadata     JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

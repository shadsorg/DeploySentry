CREATE TABLE deployment_phases (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id   UUID        NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    phase_number    INT         NOT NULL,
    traffic_pct     INT         NOT NULL CHECK (traffic_pct >= 0 AND traffic_pct <= 100),
    duration_secs   INT         NOT NULL,
    status          TEXT        NOT NULL CHECK (status IN ('pending', 'active', 'passed', 'failed', 'skipped')),
    health_snapshot JSONB,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ
);

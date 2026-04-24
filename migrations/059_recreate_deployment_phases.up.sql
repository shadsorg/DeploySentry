-- migrations/059_recreate_deployment_phases.up.sql
--
-- Migration 029 dropped deployment_phases as part of the platform
-- redesign, but the deploy engine (internal/deploy/engine) and repo
-- (internal/platform/database/postgres/deploy.go) still read/write this
-- table. With the table missing, orchestrate-mode deploys can't create
-- phases, and GET /deployments/:id/phases returns 500 for every row in
-- the UI — including the new record-mode rows the DeployHistory screen
-- just started producing.
--
-- Recreate the table with the schema the code expects (columns from
-- migrations 011 + 023 + 037 flattened into one CREATE).
CREATE TABLE IF NOT EXISTS deployment_phases (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id     UUID        NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    name              TEXT        NOT NULL DEFAULT '',
    status            TEXT        NOT NULL
                                  CHECK (status IN ('pending', 'active', 'passed', 'failed', 'skipped')),
    traffic_percent   INT         NOT NULL
                                  CHECK (traffic_percent >= 0 AND traffic_percent <= 100),
    duration_seconds  INT         NOT NULL,
    sort_order        INT         NOT NULL DEFAULT 0,
    auto_promote      BOOLEAN     NOT NULL DEFAULT true,
    started_at        TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_deployment_phases_deployment
    ON deployment_phases(deployment_id, sort_order);

-- Rollouts: wraps a change (deploy or config) being progressively applied.
CREATE TABLE rollouts (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id                UUID,
    target_type               TEXT NOT NULL CHECK (target_type IN ('deploy','config')),
    target_ref                JSONB NOT NULL,  -- {"deployment_id":"..."} or {"flag_key":"...","env":"..."}
    strategy_snapshot         JSONB NOT NULL,
    signal_source             JSONB NOT NULL DEFAULT '{"kind":"app_env"}',
    status                    TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','active','paused','awaiting_approval','succeeded','rolled_back','aborted','superseded')),
    current_phase_index       INTEGER NOT NULL DEFAULT 0,
    current_phase_started_at  TIMESTAMPTZ,
    last_healthy_since        TIMESTAMPTZ,
    rollback_reason           TEXT,
    created_by                UUID,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at              TIMESTAMPTZ
);
CREATE INDEX idx_rollouts_status       ON rollouts(status);
CREATE INDEX idx_rollouts_release      ON rollouts(release_id);
CREATE INDEX idx_rollouts_deployment   ON rollouts((target_ref->>'deployment_id'))
    WHERE target_type = 'deploy';
CREATE INDEX idx_rollouts_config       ON rollouts((target_ref->>'flag_key'), (target_ref->>'env'))
    WHERE target_type = 'config';

-- Rollout phases: per-rollout phase ledger (truth over time).
CREATE TABLE rollout_phases (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rollout_id             UUID NOT NULL REFERENCES rollouts(id) ON DELETE CASCADE,
    phase_index            INTEGER NOT NULL,
    step_snapshot          JSONB NOT NULL,
    status                 TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','active','awaiting_approval','passed','failed','rolled_back')),
    entered_at             TIMESTAMPTZ,
    exited_at              TIMESTAMPTZ,
    applied_percent        NUMERIC(6,3),
    health_score_at_exit   NUMERIC(4,3),
    notes                  TEXT NOT NULL DEFAULT '',
    UNIQUE (rollout_id, phase_index)
);
CREATE INDEX idx_rollout_phases_rollout ON rollout_phases(rollout_id);

-- Rollout events: audit trail.
CREATE TABLE rollout_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rollout_id    UUID NOT NULL REFERENCES rollouts(id) ON DELETE CASCADE,
    event_type    TEXT NOT NULL,
    actor_type    TEXT NOT NULL DEFAULT 'system' CHECK (actor_type IN ('user','system')),
    actor_id      UUID,
    reason        TEXT,
    payload       JSONB NOT NULL DEFAULT '{}',
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_rollout_events_rollout ON rollout_events(rollout_id, occurred_at);

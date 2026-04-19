-- Rollout groups: optional bundle grouping related rollouts with coordination policy.
-- Distinct from the pre-existing `releases` table (which tracks application version metadata).
CREATE TABLE rollout_groups (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type           TEXT NOT NULL CHECK (scope_type IN ('org','project','app')),
    scope_id             UUID NOT NULL,
    name                 TEXT NOT NULL,
    description          TEXT NOT NULL DEFAULT '',
    coordination_policy  TEXT NOT NULL DEFAULT 'independent'
        CHECK (coordination_policy IN ('independent','pause_on_sibling_abort','cascade_abort')),
    created_by           UUID,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_rollout_groups_scope ON rollout_groups(scope_type, scope_id);

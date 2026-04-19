-- Strategies: reusable rollout templates scoped to org, project, or app.
CREATE TABLE strategies (
    id                              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type                      TEXT NOT NULL CHECK (scope_type IN ('org','project','app')),
    scope_id                        UUID NOT NULL,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL DEFAULT '',
    target_type                     TEXT NOT NULL CHECK (target_type IN ('deploy','config','any')),
    steps                           JSONB NOT NULL,
    default_health_threshold        NUMERIC(4,3) NOT NULL DEFAULT 0.95 CHECK (default_health_threshold >= 0 AND default_health_threshold <= 1),
    default_rollback_on_failure     BOOLEAN NOT NULL DEFAULT TRUE,
    version                         INTEGER NOT NULL DEFAULT 1,
    is_system                       BOOLEAN NOT NULL DEFAULT FALSE,
    created_by                      UUID,
    updated_by                      UUID,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at                      TIMESTAMPTZ,
    UNIQUE (scope_type, scope_id, name)
);
CREATE INDEX idx_strategies_scope       ON strategies(scope_type, scope_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_strategies_target_type ON strategies(target_type)          WHERE deleted_at IS NULL;

-- Strategy defaults: pins a default strategy per (scope, environment, target_type).
CREATE TABLE strategy_defaults (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type        TEXT NOT NULL CHECK (scope_type IN ('org','project','app')),
    scope_id          UUID NOT NULL,
    environment       TEXT,   -- NULL = any environment
    target_type       TEXT CHECK (target_type IN ('deploy','config')), -- NULL = any target
    strategy_id       UUID NOT NULL REFERENCES strategies(id) ON DELETE RESTRICT,
    created_by        UUID,
    updated_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_strategy_defaults_key ON strategy_defaults (
    scope_type,
    scope_id,
    COALESCE(environment, ''),
    COALESCE(target_type, '')
);
CREATE INDEX idx_strategy_defaults_strategy ON strategy_defaults(strategy_id);

-- Rollout policies: onboarding + mandate config per scope.
CREATE TABLE rollout_policies (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type        TEXT NOT NULL CHECK (scope_type IN ('org','project','app')),
    scope_id          UUID NOT NULL,
    environment       TEXT,   -- NULL = any environment
    target_type       TEXT CHECK (target_type IN ('deploy','config')), -- NULL = any target
    enabled           BOOLEAN NOT NULL DEFAULT FALSE,
    policy            TEXT NOT NULL DEFAULT 'off' CHECK (policy IN ('off','prompt','mandate')),
    created_by        UUID,
    updated_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_rollout_policies_key ON rollout_policies (
    scope_type,
    scope_id,
    COALESCE(environment, ''),
    COALESCE(target_type, '')
);

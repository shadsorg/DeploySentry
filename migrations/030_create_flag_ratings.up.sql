-- 030_create_flag_ratings.up.sql
-- Flag ratings and error tracking for the CrowdSoft marketplace.

CREATE TABLE flag_ratings (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id    UUID        NOT NULL REFERENCES feature_flags(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id     UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    rating     SMALLINT    NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment    TEXT        CHECK (length(comment) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (flag_id, user_id)
);

CREATE INDEX deploy_idx_flag_ratings_flag_id ON flag_ratings (flag_id);
CREATE INDEX deploy_idx_flag_ratings_org_id ON flag_ratings (org_id);

CREATE TABLE flag_error_stats (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id           UUID        NOT NULL REFERENCES feature_flags(id) ON DELETE CASCADE,
    environment_id    UUID        NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    org_id            UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    period_start      TIMESTAMPTZ NOT NULL,
    total_evaluations BIGINT      NOT NULL DEFAULT 0,
    error_count       BIGINT      NOT NULL DEFAULT 0,
    UNIQUE (flag_id, environment_id, org_id, period_start)
);

CREATE INDEX deploy_idx_flag_error_stats_flag_id ON flag_error_stats (flag_id);

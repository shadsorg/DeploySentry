CREATE TABLE app_status (
    application_id    UUID        NOT NULL,
    environment_id    UUID        NOT NULL,
    version           TEXT        NOT NULL,
    commit_sha        TEXT,
    health_state      TEXT        NOT NULL,
    health_score      NUMERIC(4,3),
    health_reason     TEXT,
    deploy_slot       TEXT,
    tags              JSONB       NOT NULL DEFAULT '{}'::jsonb,
    source            TEXT        NOT NULL,
    reported_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (application_id, environment_id),
    CONSTRAINT app_status_health_check CHECK (health_state IN ('healthy','degraded','unhealthy','unknown'))
);

CREATE TABLE app_status_history (
    id                BIGSERIAL   PRIMARY KEY,
    application_id    UUID        NOT NULL,
    environment_id    UUID        NOT NULL,
    version           TEXT        NOT NULL,
    health_state      TEXT        NOT NULL,
    health_score      NUMERIC(4,3),
    reported_at       TIMESTAMPTZ NOT NULL
);

CREATE INDEX app_status_history_app_env_reported_idx
    ON app_status_history (application_id, environment_id, reported_at DESC);

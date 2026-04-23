CREATE TABLE deploy_integrations (
    id                  UUID        PRIMARY KEY,
    application_id      UUID        NOT NULL,
    provider            TEXT        NOT NULL,
    auth_mode           TEXT        NOT NULL DEFAULT 'hmac',
    webhook_secret_enc  BYTEA       NOT NULL,
    provider_config     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    env_mapping         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    version_extractors  JSONB       NOT NULL DEFAULT '[]'::jsonb,
    enabled             BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT deploy_integrations_provider_check CHECK (
        provider IN ('generic','railway','render','fly','heroku','vercel','netlify','github-actions')
    ),
    CONSTRAINT deploy_integrations_auth_mode_check CHECK (auth_mode IN ('hmac','bearer'))
);

CREATE INDEX deploy_integrations_app_provider_idx
    ON deploy_integrations (application_id, provider);

CREATE TABLE deploy_integration_events (
    id              UUID        PRIMARY KEY,
    integration_id  UUID        NOT NULL REFERENCES deploy_integrations(id) ON DELETE CASCADE,
    event_type      TEXT        NOT NULL,
    dedup_key       TEXT        NOT NULL UNIQUE,
    deployment_id   UUID,
    payload_json    JSONB       NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX deploy_integration_events_integration_received_idx
    ON deploy_integration_events (integration_id, received_at DESC);

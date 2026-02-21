CREATE TABLE feature_flags (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key           TEXT        NOT NULL,
    name          TEXT        NOT NULL,
    description   TEXT,
    flag_type     TEXT        NOT NULL CHECK (flag_type IN ('boolean', 'string', 'number', 'json')),
    default_value JSONB       NOT NULL,
    enabled       BOOLEAN     NOT NULL DEFAULT false,
    tags          TEXT[]      NOT NULL DEFAULT '{}',
    created_by    UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at   TIMESTAMPTZ,
    UNIQUE (project_id, key)
);

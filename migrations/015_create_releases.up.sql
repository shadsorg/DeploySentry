CREATE TABLE releases (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version     TEXT        NOT NULL,
    commit_sha  TEXT        NOT NULL,
    branch      TEXT,
    changelog   TEXT,
    artifact_url TEXT,
    status      TEXT        CHECK (status IN ('building', 'built', 'deploying', 'deployed', 'healthy', 'degraded', 'rolled_back')),
    created_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, version)
);

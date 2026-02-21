CREATE TABLE environments (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name              TEXT        NOT NULL,
    slug              TEXT        NOT NULL,
    is_production     BOOLEAN     NOT NULL DEFAULT false,
    requires_approval BOOLEAN     NOT NULL DEFAULT false,
    settings          JSONB       NOT NULL DEFAULT '{}',
    sort_order        INT         NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, slug)
);

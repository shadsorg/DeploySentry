CREATE TABLE segments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key           TEXT NOT NULL,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    combine_op    TEXT NOT NULL DEFAULT 'AND' CHECK (combine_op IN ('AND', 'OR')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, key)
);

CREATE TABLE segment_conditions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    segment_id    UUID NOT NULL REFERENCES segments(id) ON DELETE CASCADE,
    attribute     TEXT NOT NULL,
    operator      TEXT NOT NULL,
    value         TEXT NOT NULL,
    priority      INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_segments_project_id ON segments(project_id);
CREATE INDEX idx_segment_conditions_segment_id ON segment_conditions(segment_id);

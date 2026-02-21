CREATE TABLE release_environments (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id   UUID          NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    environment  TEXT          NOT NULL,
    status       TEXT          NOT NULL,
    deployed_at  TIMESTAMPTZ,
    health_score NUMERIC(5,2),
    UNIQUE (release_id, environment)
);

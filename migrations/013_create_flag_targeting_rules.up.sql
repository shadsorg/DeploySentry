CREATE TABLE flag_targeting_rules (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id     UUID        NOT NULL REFERENCES feature_flags(id) ON DELETE CASCADE,
    environment TEXT        NOT NULL,
    priority    INT         NOT NULL,
    rule_type   TEXT        NOT NULL CHECK (rule_type IN ('percentage', 'user_target', 'attribute', 'segment', 'schedule')),
    conditions  JSONB       NOT NULL,
    serve_value JSONB       NOT NULL,
    enabled     BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

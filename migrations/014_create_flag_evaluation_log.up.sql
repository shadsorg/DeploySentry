CREATE TABLE flag_evaluation_log (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id      UUID        NOT NULL,
    flag_key     TEXT        NOT NULL,
    environment  TEXT        NOT NULL,
    context_hash TEXT        NOT NULL,
    result_value JSONB       NOT NULL,
    rule_matched UUID,
    evaluated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_flag_evaluation_log_evaluated_at ON flag_evaluation_log (evaluated_at);

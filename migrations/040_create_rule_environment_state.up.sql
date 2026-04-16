CREATE TABLE rule_environment_state (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id         UUID NOT NULL REFERENCES flag_targeting_rules(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    enabled         BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (rule_id, environment_id)
);

CREATE INDEX idx_rule_env_state_rule_id ON rule_environment_state(rule_id);
CREATE INDEX idx_rule_env_state_env_id ON rule_environment_state(environment_id);

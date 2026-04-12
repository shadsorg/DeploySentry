ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS combine_op TEXT NOT NULL DEFAULT 'AND';

ALTER TABLE flag_targeting_rules DROP CONSTRAINT IF EXISTS flag_targeting_rules_rule_type_check;
ALTER TABLE flag_targeting_rules ADD CONSTRAINT flag_targeting_rules_rule_type_check
    CHECK (rule_type IN ('percentage', 'user_target', 'attribute', 'segment', 'schedule', 'compound'));

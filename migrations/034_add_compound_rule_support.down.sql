ALTER TABLE flag_targeting_rules DROP CONSTRAINT IF EXISTS flag_targeting_rules_rule_type_check;
ALTER TABLE flag_targeting_rules ADD CONSTRAINT flag_targeting_rules_rule_type_check
    CHECK (rule_type IN ('percentage', 'user_target', 'attribute', 'segment', 'schedule'));

ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS combine_op;

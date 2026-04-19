DELETE FROM strategy_defaults
WHERE strategy_id IN (
    SELECT id FROM strategies WHERE is_system = TRUE AND name LIKE 'system-%'
);
DELETE FROM strategies WHERE is_system = TRUE AND name LIKE 'system-%';

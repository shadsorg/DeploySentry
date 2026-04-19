-- Seed system-owned strategy templates for every existing org.
-- Values mirror the hardcoded Go defaults in internal/deploy/strategies/.

-- 1% → 5% → 25% → 50% → 100% with 5/5/10/10/0 minute dwells.
INSERT INTO strategies (scope_type, scope_id, name, description, target_type, steps,
                        default_health_threshold, default_rollback_on_failure, is_system)
SELECT 'org', o.id, 'system-canary',
       'System default canary rollout (1% → 5% → 25% → 50% → 100%).',
       'deploy',
       '[
          {"percent":1,"min_duration":300000000000,"max_duration":300000000000,"bake_time_healthy":0},
          {"percent":5,"min_duration":300000000000,"max_duration":300000000000,"bake_time_healthy":0},
          {"percent":25,"min_duration":600000000000,"max_duration":600000000000,"bake_time_healthy":0},
          {"percent":50,"min_duration":600000000000,"max_duration":600000000000,"bake_time_healthy":0},
          {"percent":100,"min_duration":0,"max_duration":0,"bake_time_healthy":0}
        ]'::jsonb,
       0.950, TRUE, TRUE
FROM organizations o
ON CONFLICT (scope_type, scope_id, name) DO NOTHING;

INSERT INTO strategies (scope_type, scope_id, name, description, target_type, steps,
                        default_health_threshold, default_rollback_on_failure, is_system)
SELECT 'org', o.id, 'system-blue-green',
       'System default blue-green: atomic 0 → 100 after 2-minute warmup.',
       'deploy',
       '[
          {"percent":0,"min_duration":120000000000,"max_duration":120000000000,"bake_time_healthy":0},
          {"percent":100,"min_duration":0,"max_duration":0,"bake_time_healthy":0}
        ]'::jsonb,
       0.950, TRUE, TRUE
FROM organizations o
ON CONFLICT (scope_type, scope_id, name) DO NOTHING;

INSERT INTO strategies (scope_type, scope_id, name, description, target_type, steps,
                        default_health_threshold, default_rollback_on_failure, is_system)
SELECT 'org', o.id, 'system-rolling',
       'System default rolling update: three batches with 30s delay.',
       'deploy',
       '[
          {"percent":33,"min_duration":30000000000,"max_duration":30000000000,"bake_time_healthy":0},
          {"percent":67,"min_duration":30000000000,"max_duration":30000000000,"bake_time_healthy":0},
          {"percent":100,"min_duration":0,"max_duration":0,"bake_time_healthy":0}
        ]'::jsonb,
       0.950, TRUE, TRUE
FROM organizations o
ON CONFLICT (scope_type, scope_id, name) DO NOTHING;

-- Seed default strategy assignment: any-env, deploy → system-canary per org.
INSERT INTO strategy_defaults (scope_type, scope_id, environment, target_type, strategy_id)
SELECT 'org', s.scope_id, NULL, 'deploy', s.id
FROM strategies s
WHERE s.name = 'system-canary' AND s.scope_type = 'org'
ON CONFLICT (scope_type, scope_id, COALESCE(environment,''), COALESCE(target_type,'')) DO NOTHING;

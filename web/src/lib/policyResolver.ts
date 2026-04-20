import type { RolloutPolicy, TargetType, PolicyKind } from '@/types';

/**
 * Resolves the effective policy for a (target, env) given the org-level policy rows.
 * Mirrors internal/rollout/resolver.go:ResolvePolicy priority:
 *   (env match + target match) > (env match, any target) > (any env, target match) > (any env, any target)
 *
 * Note: this client-side version only knows about org-level rows since the
 * UI fetches /orgs/:slug/rollout-policy which is scoped to the org. Project-
 * and app-level overrides aren't visible here. For Plan 5's org-only UI, this
 * is sufficient; a future improvement can walk project/app scopes.
 */
export function resolvePolicy(
  rows: RolloutPolicy[],
  env: string | undefined,
  target: TargetType,
): { enabled: boolean; policy: PolicyKind } {
  const patterns: Array<{ envMatch: boolean; targetMatch: boolean }> = [
    { envMatch: true, targetMatch: true },
    { envMatch: true, targetMatch: false },
    { envMatch: false, targetMatch: true },
    { envMatch: false, targetMatch: false },
  ];
  for (const pat of patterns) {
    for (const row of rows) {
      if (pat.envMatch) {
        if (!row.environment || !env || row.environment !== env) continue;
      } else if (row.environment) {
        continue;
      }
      if (pat.targetMatch) {
        if (!row.target_type || row.target_type !== target) continue;
      } else if (row.target_type) {
        continue;
      }
      return { enabled: row.enabled, policy: row.policy };
    }
  }
  return { enabled: false, policy: 'off' };
}

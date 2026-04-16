import type { EvaluationContext, FlagConfig, FlagConfigRule } from './types';

export function evaluateLocal(
  config: FlagConfig,
  environment: string,
  key: string,
  context?: EvaluationContext,
): { value: string; reason: string } {
  const flag = config.flags.find((f) => f.key === key);
  if (!flag) return { value: '', reason: 'flag_not_found' };

  const envState = flag.environments[environment];
  if (!envState || !envState.enabled) {
    return { value: flag.default_value, reason: 'env_disabled' };
  }

  if (flag.rules && context) {
    const sorted = [...flag.rules].sort((a, b) => a.priority - b.priority);
    for (const rule of sorted) {
      if (!rule.environments[environment]) continue;
      if (matchRule(rule, context)) {
        return { value: rule.value, reason: 'rule_match' };
      }
    }
  }

  return { value: envState.value || flag.default_value, reason: 'default' };
}

function matchRule(rule: FlagConfigRule, context: EvaluationContext): boolean {
  const val = context.attributes?.[rule.attribute] ?? '';
  const targets = rule.target_values ?? [];
  switch (rule.operator) {
    case 'equals': return targets.length > 0 && val === targets[0];
    case 'not_equals': return targets.length > 0 && val !== targets[0];
    case 'in': return targets.includes(val);
    case 'not_in': return !targets.includes(val);
    case 'contains': return targets.length > 0 && val.includes(targets[0]);
    case 'starts_with': return targets.length > 0 && val.startsWith(targets[0]);
    case 'ends_with': return targets.length > 0 && val.endsWith(targets[0]);
    case 'greater_than': return targets.length > 0 && parseFloat(val) > parseFloat(targets[0]);
    case 'less_than': return targets.length > 0 && parseFloat(val) < parseFloat(targets[0]);
    default: return false;
  }
}

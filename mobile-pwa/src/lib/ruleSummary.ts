import type { TargetingRule } from '../types';

/**
 * Produce a one-line human-readable summary of a targeting rule.
 *
 * Pure function — no React, no I/O. Used by both FlagDetailPage's rule list
 * and RuleEditSheet's live preview.
 */
export function ruleSummary(rule: TargetingRule): string {
  switch (rule.rule_type) {
    case 'percentage': {
      const n = rule.percentage ?? 0;
      return `${n}% rollout`;
    }
    case 'user_target': {
      const n = rule.user_ids?.length ?? 0;
      return `${n} user IDs`;
    }
    case 'attribute': {
      const combined = `${rule.attribute ?? ''} ${rule.operator ?? ''} ${rule.value ?? ''}`.trim();
      return combined.length > 40 ? `${combined.slice(0, 39)}…` : combined;
    }
    case 'segment':
      return `segment: ${rule.segment_id ?? ''}`;
    case 'schedule':
      return `${rule.start_time ?? ''} – ${rule.end_time ?? ''}`;
    case 'compound':
      return 'compound (edit on desktop)';
    default:
      return rule.value ?? '';
  }
}

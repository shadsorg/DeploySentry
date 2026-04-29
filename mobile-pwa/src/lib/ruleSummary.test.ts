import { describe, it, expect } from 'vitest';
import { ruleSummary } from './ruleSummary';
import type { TargetingRule } from '../types';

function makeRule(overrides: Partial<TargetingRule>): TargetingRule {
  return {
    id: 'r1',
    flag_id: 'f1',
    value: '',
    priority: 1,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('ruleSummary', () => {
  it('formats percentage rules as "{n}% rollout"', () => {
    const rule = makeRule({ rule_type: 'percentage', percentage: 25 });
    expect(ruleSummary(rule)).toBe('25% rollout');
  });

  it('uses 0% when percentage is missing on a percentage rule', () => {
    const rule = makeRule({ rule_type: 'percentage', percentage: null });
    expect(ruleSummary(rule)).toBe('0% rollout');
  });

  it('formats user_target rules with count of user_ids', () => {
    const rule = makeRule({
      rule_type: 'user_target',
      user_ids: ['alice', 'bob', 'carol'],
    });
    expect(ruleSummary(rule)).toBe('3 user IDs');
  });

  it('formats user_target with 0 when user_ids is missing', () => {
    const rule = makeRule({ rule_type: 'user_target', user_ids: null });
    expect(ruleSummary(rule)).toBe('0 user IDs');
  });

  it('formats attribute rules as "{attribute} {operator} {value}"', () => {
    const rule = makeRule({
      rule_type: 'attribute',
      attribute: 'plan',
      operator: 'eq',
      value: 'enterprise',
    });
    expect(ruleSummary(rule)).toBe('plan eq enterprise');
  });

  it('truncates attribute summaries longer than 40 chars with an ellipsis', () => {
    const rule = makeRule({
      rule_type: 'attribute',
      attribute: 'very_long_attribute_name_indeed',
      operator: 'contains',
      value: 'some_long_value_that_pushes_us_over',
    });
    const out = ruleSummary(rule);
    expect(out.length).toBe(40);
    expect(out.endsWith('…')).toBe(true);
  });

  it('formats segment rules as "segment: {segment_id}"', () => {
    const rule = makeRule({ rule_type: 'segment', segment_id: 'beta-users' });
    expect(ruleSummary(rule)).toBe('segment: beta-users');
  });

  it('formats schedule rules as "{start_time} – {end_time}"', () => {
    const rule = makeRule({
      rule_type: 'schedule',
      start_time: '2026-01-01T00:00:00Z',
      end_time: '2026-02-01T00:00:00Z',
    });
    expect(ruleSummary(rule)).toBe('2026-01-01T00:00:00Z – 2026-02-01T00:00:00Z');
  });

  it('formats compound rules with the desktop-only hint', () => {
    const rule = makeRule({ rule_type: 'compound' });
    expect(ruleSummary(rule)).toBe('compound (edit on desktop)');
  });

  it('falls back to value for legacy rules with no rule_type', () => {
    const rule = makeRule({ rule_type: undefined, value: 'legacy-value' });
    expect(ruleSummary(rule)).toBe('legacy-value');
  });
});

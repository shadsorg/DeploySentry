import { describe, it, expect } from 'vitest';
import { resolvePolicy } from './policyResolver';

describe('resolvePolicy', () => {
  it('returns off when no rows', () => {
    expect(resolvePolicy([], 'prod', 'deploy')).toEqual({ enabled: false, policy: 'off' });
  });

  it('matches most-specific key first', () => {
    const rows = [
      {
        id: '1',
        scope_type: 'org' as const,
        scope_id: 'o',
        enabled: true,
        policy: 'off' as const,
        created_at: '',
        updated_at: '',
      },
      {
        id: '2',
        scope_type: 'org' as const,
        scope_id: 'o',
        target_type: 'deploy' as const,
        enabled: true,
        policy: 'mandate' as const,
        created_at: '',
        updated_at: '',
      },
    ];
    expect(resolvePolicy(rows, 'prod', 'deploy').policy).toBe('mandate');
  });

  it('falls through to wildcard', () => {
    const rows = [
      {
        id: '1',
        scope_type: 'org' as const,
        scope_id: 'o',
        enabled: true,
        policy: 'prompt' as const,
        created_at: '',
        updated_at: '',
      },
    ];
    expect(resolvePolicy(rows, 'staging', 'config').policy).toBe('prompt');
  });

  it('env-specific beats any-env', () => {
    const rows = [
      {
        id: '1',
        scope_type: 'org' as const,
        scope_id: 'o',
        environment: 'prod',
        enabled: true,
        policy: 'mandate' as const,
        created_at: '',
        updated_at: '',
      },
      {
        id: '2',
        scope_type: 'org' as const,
        scope_id: 'o',
        enabled: true,
        policy: 'off' as const,
        created_at: '',
        updated_at: '',
      },
    ];
    expect(resolvePolicy(rows, 'prod', 'deploy').policy).toBe('mandate');
    expect(resolvePolicy(rows, 'dev', 'deploy').policy).toBe('off');
  });
});

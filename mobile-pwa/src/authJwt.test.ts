import { describe, it, expect } from 'vitest';
import { getTokenExpiryMs } from './authJwt';

function makeJwt(payload: Record<string, unknown>): string {
  const toB64u = (s: string) => btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  return `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify(payload))}.sig`;
}

describe('getTokenExpiryMs', () => {
  it('returns null for null/undefined/empty input', () => {
    expect(getTokenExpiryMs(null)).toBeNull();
    expect(getTokenExpiryMs(undefined)).toBeNull();
    expect(getTokenExpiryMs('')).toBeNull();
  });

  it('returns null for API-key tokens (ds_ prefix)', () => {
    expect(getTokenExpiryMs('ds_abc123')).toBeNull();
  });

  it('returns null for malformed JWTs', () => {
    expect(getTokenExpiryMs('not-a-jwt')).toBeNull();
    expect(getTokenExpiryMs('a.b')).toBeNull();
  });

  it('returns ms-since-epoch for a JWT with exp', () => {
    const expSec = 1_800_000_000;
    expect(getTokenExpiryMs(makeJwt({ exp: expSec }))).toBe(expSec * 1000);
  });

  it('returns null when exp is missing', () => {
    expect(getTokenExpiryMs(makeJwt({ sub: 'u1' }))).toBeNull();
  });
});

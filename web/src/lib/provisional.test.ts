import { describe, it, expect } from 'vitest';
import { newProvisionalId, isProvisionalId } from './provisional';

describe('newProvisionalId', () => {
  it('mints a UUID with the provisional variant byte (0xc0)', () => {
    const id = newProvisionalId();
    expect(id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
    // Variant byte: char 19 is the high nibble of byte 8 → must be in [c-f].
    expect(id.charAt(19)).toMatch(/[c-f]/);
    expect(isProvisionalId(id)).toBe(true);
  });

  it('produces unique ids on successive calls', () => {
    const ids = new Set();
    for (let i = 0; i < 16; i++) ids.add(newProvisionalId());
    expect(ids.size).toBe(16);
  });

  it('round-trips through JSON.stringify / parse unchanged', () => {
    const id = newProvisionalId();
    expect(JSON.parse(JSON.stringify({ id })).id).toBe(id);
  });
});

describe('isProvisionalId', () => {
  it('returns false for an RFC-4122 v4 UUID', () => {
    expect(isProvisionalId('550e8400-e29b-41d4-a716-446655440000')).toBe(false);
  });

  it('returns false for malformed input', () => {
    expect(isProvisionalId('not-a-uuid')).toBe(false);
    expect(isProvisionalId('')).toBe(false);
  });
});

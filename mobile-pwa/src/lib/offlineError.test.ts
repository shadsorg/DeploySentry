import { describe, it, expect, vi, afterEach } from 'vitest';
import {
  OfflineWriteBlockedError,
  assertOnlineForWrite,
  isOfflineWriteBlockedError,
} from './offlineError';

describe('OfflineWriteBlockedError', () => {
  it('is an instance of Error and has a default message', () => {
    const err = new OfflineWriteBlockedError();
    expect(err).toBeInstanceOf(Error);
    expect(err).toBeInstanceOf(OfflineWriteBlockedError);
    expect(err.message).toBe("You're offline — connect to make changes.");
  });

  it('has name === "OfflineWriteBlockedError"', () => {
    const err = new OfflineWriteBlockedError();
    expect(err.name).toBe('OfflineWriteBlockedError');
  });

  it('accepts a custom message', () => {
    const err = new OfflineWriteBlockedError('custom');
    expect(err.message).toBe('custom');
  });
});

describe('isOfflineWriteBlockedError', () => {
  it('returns true for an OfflineWriteBlockedError instance', () => {
    expect(isOfflineWriteBlockedError(new OfflineWriteBlockedError())).toBe(true);
  });

  it('returns false for a plain Error', () => {
    expect(isOfflineWriteBlockedError(new Error('boom'))).toBe(false);
  });

  it('returns false for non-Error values', () => {
    expect(isOfflineWriteBlockedError(null)).toBe(false);
    expect(isOfflineWriteBlockedError(undefined)).toBe(false);
    expect(isOfflineWriteBlockedError('string')).toBe(false);
    expect(isOfflineWriteBlockedError(42)).toBe(false);
    expect(isOfflineWriteBlockedError({ name: 'OfflineWriteBlockedError' })).toBe(false);
  });
});

describe('assertOnlineForWrite', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('does not throw when navigator.onLine is true', () => {
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
    expect(() => assertOnlineForWrite()).not.toThrow();
  });

  it('throws OfflineWriteBlockedError when navigator.onLine is false', () => {
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
    expect(() => assertOnlineForWrite()).toThrow(OfflineWriteBlockedError);
  });
});

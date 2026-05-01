import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useStagingEnabled, setStagingEnabled } from './useStagingEnabled';

beforeEach(() => {
  localStorage.clear();
});

describe('useStagingEnabled', () => {
  it('returns false when no orgSlug is supplied', () => {
    const { result } = renderHook(() => useStagingEnabled(undefined));
    expect(result.current).toBe(false);
  });

  it('reads the per-org localStorage key on mount', () => {
    localStorage.setItem('ds_staging_enabled:acme', 'true');
    const { result } = renderHook(() => useStagingEnabled('acme'));
    expect(result.current).toBe(true);
  });

  it('does not bleed across orgs', () => {
    localStorage.setItem('ds_staging_enabled:acme', 'true');
    const { result } = renderHook(() => useStagingEnabled('beta'));
    expect(result.current).toBe(false);
  });

  it('updates when the imperative setter fires the custom event', () => {
    const { result } = renderHook(() => useStagingEnabled('acme'));
    expect(result.current).toBe(false);
    act(() => setStagingEnabled('acme', true));
    expect(result.current).toBe(true);
    act(() => setStagingEnabled('acme', false));
    expect(result.current).toBe(false);
  });

  it('responds to a storage event from another tab', () => {
    const { result } = renderHook(() => useStagingEnabled('acme'));
    expect(result.current).toBe(false);
    act(() => {
      localStorage.setItem('ds_staging_enabled:acme', 'true');
      window.dispatchEvent(
        new StorageEvent('storage', {
          key: 'ds_staging_enabled:acme',
          newValue: 'true',
        }),
      );
    });
    expect(result.current).toBe(true);
  });
});

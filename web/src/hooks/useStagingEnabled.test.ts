import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useStagingEnabled, __resetStagingEnabledCacheForTests } from './useStagingEnabled';

// Mock the API module so no real fetches happen.
vi.mock('@/api', () => ({
  stagingApi: {
    getEnabled: vi.fn(),
  },
  settingsApi: {
    set: vi.fn(),
  },
  entitiesApi: {
    getOrg: vi.fn(),
  },
}));

import { stagingApi } from '@/api';

beforeEach(() => {
  vi.clearAllMocks();
  __resetStagingEnabledCacheForTests();
});

describe('useStagingEnabled', () => {
  it('returns false when no orgSlug is supplied', () => {
    const { result } = renderHook(() => useStagingEnabled(undefined));
    expect(result.current).toBe(false);
  });

  it('returns false initially, then true after fetch resolves with enabled=true', async () => {
    (stagingApi.getEnabled as ReturnType<typeof vi.fn>).mockResolvedValue({ enabled: true });

    const { result } = renderHook(() => useStagingEnabled('acme'));

    // Before fetch resolves: false.
    expect(result.current).toBe(false);

    await waitFor(() => expect(result.current).toBe(true));
    expect(stagingApi.getEnabled).toHaveBeenCalledWith('acme');
  });

  it('returns false when fetch resolves with enabled=false', async () => {
    (stagingApi.getEnabled as ReturnType<typeof vi.fn>).mockResolvedValue({ enabled: false });

    const { result } = renderHook(() => useStagingEnabled('acme'));

    await waitFor(() => expect(stagingApi.getEnabled).toHaveBeenCalledTimes(1));
    expect(result.current).toBe(false);
  });

  it('returns false when fetch rejects', async () => {
    (stagingApi.getEnabled as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('network'));

    const { result } = renderHook(() => useStagingEnabled('acme'));

    await waitFor(() => expect(stagingApi.getEnabled).toHaveBeenCalledTimes(1));
    expect(result.current).toBe(false);
  });

  it('does not bleed across orgs', async () => {
    (stagingApi.getEnabled as ReturnType<typeof vi.fn>).mockImplementation((slug: string) =>
      Promise.resolve({ enabled: slug === 'acme' }),
    );

    const { result: acme } = renderHook(() => useStagingEnabled('acme'));
    const { result: beta } = renderHook(() => useStagingEnabled('beta'));

    await waitFor(() => expect(acme.current).toBe(true));
    expect(beta.current).toBe(false);
  });

  it('refetches when the ds:staging-enabled custom event fires', async () => {
    (stagingApi.getEnabled as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({ enabled: false })
      .mockResolvedValueOnce({ enabled: true });

    const { result } = renderHook(() => useStagingEnabled('acme'));

    await waitFor(() => expect(stagingApi.getEnabled).toHaveBeenCalledTimes(1));
    expect(result.current).toBe(false);

    act(() => {
      window.dispatchEvent(new CustomEvent('ds:staging-enabled'));
    });

    await waitFor(() => expect(result.current).toBe(true));
    expect(stagingApi.getEnabled).toHaveBeenCalledTimes(2);
  });

  it('dedupes concurrent calls to the same org', async () => {
    const fetchSpy = vi.spyOn(stagingApi, 'getEnabled').mockResolvedValue({ enabled: true });
    __resetStagingEnabledCacheForTests();

    const { result: a } = renderHook(() => useStagingEnabled('acme'));
    const { result: b } = renderHook(() => useStagingEnabled('acme'));
    const { result: c } = renderHook(() => useStagingEnabled('acme'));

    await waitFor(() => {
      expect(a.current).toBe(true);
      expect(b.current).toBe(true);
      expect(c.current).toBe(true);
    });
    expect(fetchSpy).toHaveBeenCalledTimes(1);
  });

  it('serves subsequent mounts from cache without refetching', async () => {
    const fetchSpy = vi.spyOn(stagingApi, 'getEnabled').mockResolvedValue({ enabled: true });
    __resetStagingEnabledCacheForTests();

    const first = renderHook(() => useStagingEnabled('acme'));
    await waitFor(() => expect(first.result.current).toBe(true));
    first.unmount();

    const second = renderHook(() => useStagingEnabled('acme'));
    // Should resolve synchronously from cache.
    expect(second.result.current).toBe(true);
    expect(fetchSpy).toHaveBeenCalledTimes(1);
  });

  it('recovers after a failed fetch (does not cache failures)', async () => {
    const fetchSpy = vi
      .spyOn(stagingApi, 'getEnabled')
      .mockRejectedValueOnce(new Error('429'))
      .mockResolvedValueOnce({ enabled: true });
    __resetStagingEnabledCacheForTests();

    const first = renderHook(() => useStagingEnabled('acme'));
    await waitFor(() => expect(first.result.current).toBe(false));
    first.unmount();

    const second = renderHook(() => useStagingEnabled('acme'));
    await waitFor(() => expect(second.result.current).toBe(true));
    expect(fetchSpy).toHaveBeenCalledTimes(2);
  });
});

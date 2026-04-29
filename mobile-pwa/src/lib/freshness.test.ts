import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useTrackedFetch } from './freshness';

describe('useTrackedFetch', () => {
  it('starts with data: null and inflight: true on mount', () => {
    const fetcher = vi.fn(() => new Promise<string>(() => {}));
    const { result } = renderHook(() => useTrackedFetch(fetcher, []));
    expect(result.current.data).toBe(null);
    expect(result.current.error).toBe(null);
    expect(result.current.freshness.lastSuccess).toBe(null);
    expect(result.current.freshness.inflight).toBe(true);
  });

  it('populates data, sets lastSuccess, and clears inflight on success', async () => {
    const fetcher = vi.fn().mockResolvedValue('hello');
    const { result } = renderHook(() => useTrackedFetch(fetcher, []));

    await waitFor(() => expect(result.current.freshness.inflight).toBe(false));

    expect(result.current.data).toBe('hello');
    expect(result.current.error).toBe(null);
    expect(typeof result.current.freshness.lastSuccess).toBe('number');
    expect(Number.isFinite(result.current.freshness.lastSuccess)).toBe(true);
  });

  it('refetch() flips inflight true then resolves with a refreshed lastSuccess', async () => {
    let call = 0;
    const fetcher = vi.fn(() => Promise.resolve(`v${++call}`));
    const { result } = renderHook(() => useTrackedFetch(fetcher, []));

    await waitFor(() => expect(result.current.freshness.inflight).toBe(false));
    expect(result.current.data).toBe('v1');
    const firstSuccess = result.current.freshness.lastSuccess;

    // Make refetch's resolution observable; await a microtask gap so timestamp advances.
    await new Promise((r) => setTimeout(r, 2));

    act(() => {
      result.current.refetch();
    });
    expect(result.current.freshness.inflight).toBe(true);

    await waitFor(() => expect(result.current.freshness.inflight).toBe(false));
    expect(result.current.data).toBe('v2');
    expect(result.current.freshness.lastSuccess).not.toBe(null);
    expect(result.current.freshness.lastSuccess!).toBeGreaterThanOrEqual(firstSuccess!);
  });

  it('on error: error set, inflight false, data null on first load', async () => {
    const fetcher = vi.fn().mockRejectedValue(new Error('boom'));
    const { result } = renderHook(() => useTrackedFetch(fetcher, []));

    await waitFor(() => expect(result.current.freshness.inflight).toBe(false));

    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe('boom');
    expect(result.current.data).toBe(null);
    expect(result.current.freshness.lastSuccess).toBe(null);
  });

  it('on error: data is retained from a previous successful fetch', async () => {
    let mode: 'ok' | 'fail' = 'ok';
    const fetcher = vi.fn(() =>
      mode === 'ok' ? Promise.resolve('cached') : Promise.reject(new Error('net')),
    );
    const { result } = renderHook(() => useTrackedFetch(fetcher, []));

    await waitFor(() => expect(result.current.data).toBe('cached'));
    const firstSuccess = result.current.freshness.lastSuccess;

    mode = 'fail';
    act(() => {
      result.current.refetch();
    });
    await waitFor(() => expect(result.current.freshness.inflight).toBe(false));

    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.data).toBe('cached');
    // lastSuccess unchanged on error
    expect(result.current.freshness.lastSuccess).toBe(firstSuccess);
  });

  it('re-fires the fetcher when deps change', async () => {
    const fetcher = vi.fn().mockResolvedValue('x');
    const { result, rerender } = renderHook(
      ({ id }: { id: number }) => useTrackedFetch(fetcher, [id]),
      { initialProps: { id: 1 } },
    );
    await waitFor(() => expect(result.current.freshness.inflight).toBe(false));
    expect(fetcher).toHaveBeenCalledTimes(1);

    rerender({ id: 2 });
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
  });
});

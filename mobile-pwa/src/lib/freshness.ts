import { useCallback, useEffect, useRef, useState } from 'react';

export interface FetchFreshness {
  /** Date.now() at the moment of the most recent successful fetch, or null if none yet. */
  lastSuccess: number | null;
  /** True while a fetch is currently running. */
  inflight: boolean;
}

export interface UseTrackedFetchResult<T> {
  data: T | null;
  freshness: FetchFreshness;
  error: Error | null;
  refetch: () => void;
}

/**
 * Tracks freshness around an async fetcher.
 *
 * - Fires on mount and whenever `deps` change.
 * - `inflight` is true while a fetch is running.
 * - `lastSuccess` updates only on success; data from prior successes is retained on error.
 * - `refetch()` re-fires the fetcher.
 *
 * Stale fetches (deps changed mid-flight, or refetch fired again) have their results
 * dropped via a per-call sequence number so we never apply a slow response on top of a fresh one.
 */
export function useTrackedFetch<T>(
  fetcher: () => Promise<T>,
  deps: unknown[],
): UseTrackedFetchResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [lastSuccess, setLastSuccess] = useState<number | null>(null);
  const [inflight, setInflight] = useState<boolean>(true);
  const [tick, setTick] = useState<number>(0);

  // Latest sequence number; only the latest call may update state.
  const seqRef = useRef<number>(0);
  // Hold fetcher in a ref so that changing fetcher identity doesn't itself re-trigger
  // (deps array is the contract for re-firing).
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  useEffect(() => {
    const mySeq = ++seqRef.current;
    setInflight(true);
    let cancelled = false;
    fetcherRef
      .current()
      .then((value) => {
        if (cancelled || mySeq !== seqRef.current) return;
        setData(value);
        setError(null);
        setLastSuccess(Date.now());
        setInflight(false);
      })
      .catch((err: unknown) => {
        if (cancelled || mySeq !== seqRef.current) return;
        setError(err instanceof Error ? err : new Error(String(err)));
        setInflight(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick]);

  const refetch = useCallback(() => {
    setTick((t) => t + 1);
  }, []);

  return {
    data,
    error,
    freshness: { lastSuccess, inflight },
    refetch,
  };
}

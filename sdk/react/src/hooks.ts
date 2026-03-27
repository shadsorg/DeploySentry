import { useCallback, useContext, useMemo, useSyncExternalStore } from 'react';
import type { DeploySentryClient } from './client';
import { DeploySentryContext } from './context';
import type { Flag, FlagCategory, FlagDetail, FlagMetadata } from './types';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const EMPTY_FLAGS: Flag[] = [];

const DEFAULT_METADATA: FlagMetadata = {
  category: 'feature',
  purpose: '',
  owners: [],
  isPermanent: false,
  expiresAt: undefined,
  tags: [],
};

function useClient(): DeploySentryClient {
  const client = useContext(DeploySentryContext);
  if (!client) {
    throw new Error(
      'DeploySentry hooks must be used within a <DeploySentryProvider>. ' +
        'Wrap your component tree with the provider before calling any hook.',
    );
  }
  return client;
}

/**
 * Subscribe to the client's flag store via `useSyncExternalStore`.
 *
 * Every flag-change notification from the client (initial fetch or SSE
 * update) will trigger a synchronous re-render of any component that
 * consumes this hook.
 */
function useFlagSnapshot(client: DeploySentryClient): Flag[] {
  const subscribe = useCallback(
    (onStoreChange: () => void) => client.subscribe(onStoreChange),
    [client],
  );

  const getSnapshot = useCallback(() => client.getAllFlags(), [client]);

  // For SSR, return an empty array. Flags are fetched client-side.
  const getServerSnapshot = useCallback(() => EMPTY_FLAGS, []);

  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

// ---------------------------------------------------------------------------
// Public hooks
// ---------------------------------------------------------------------------

/**
 * Return the raw {@link DeploySentryClient} instance.
 *
 * Useful for advanced use-cases such as manual re-identification or
 * subscribing to change events outside of React's lifecycle.
 *
 * @throws If called outside of a `<DeploySentryProvider>`.
 */
export function useDeploySentry(): DeploySentryClient {
  return useClient();
}

/**
 * Evaluate a feature flag and return its resolved value.
 *
 * The component re-renders whenever the flag value changes (via SSE).
 *
 * @param key          - The unique flag key.
 * @param defaultValue - Value returned while the flag is loading or if the
 *                       key does not exist.
 * @returns The resolved flag value, or `defaultValue`.
 *
 * @example
 * ```tsx
 * const showBanner = useFlag('show-banner', false);
 * ```
 */
export function useFlag<T = unknown>(key: string, defaultValue: T): T {
  const client = useClient();
  // Subscribe to the store so we re-render on updates.
  useFlagSnapshot(client);

  const flag = client.getFlag(key);
  if (!flag) return defaultValue;
  return (flag.enabled ? flag.value : defaultValue) as T;
}

/**
 * Return detailed evaluation information for a single flag.
 *
 * Includes the resolved value, enabled state, full metadata, and a
 * `loading` indicator that is `true` until the initial fetch completes.
 *
 * @param key - The unique flag key.
 *
 * @example
 * ```tsx
 * const { value, enabled, metadata, loading } = useFlagDetail('new-checkout');
 * ```
 */
export function useFlagDetail<T = unknown>(key: string): FlagDetail<T> {
  const client = useClient();
  useFlagSnapshot(client);

  const flag = client.getFlag(key);
  const loading = !client.isInitialised;

  if (!flag) {
    return {
      value: undefined as T,
      enabled: false,
      metadata: DEFAULT_METADATA,
      loading,
    };
  }

  const metadata: FlagMetadata = {
    category: flag.category,
    purpose: flag.purpose,
    owners: flag.owners,
    isPermanent: flag.isPermanent,
    expiresAt: flag.expiresAt,
    tags: flag.tags,
  };

  return {
    value: flag.value as T,
    enabled: flag.enabled,
    metadata,
    loading,
  };
}

/**
 * Return all flags that belong to the given category.
 *
 * The result is referentially stable across renders as long as the
 * underlying flag data has not changed.
 *
 * @param category - One of the predefined {@link FlagCategory} values.
 *
 * @example
 * ```tsx
 * const experiments = useFlagsByCategory('experiment');
 * ```
 */
export function useFlagsByCategory(category: FlagCategory): Flag[] {
  const client = useClient();
  const allFlags = useFlagSnapshot(client);

  return useMemo(
    () => allFlags.filter((f) => f.category === category),
    [allFlags, category],
  );
}

/**
 * Return all non-permanent flags whose `expiresAt` date is in the past.
 *
 * This is useful for building admin dashboards that surface stale flags
 * that should be cleaned up.
 *
 * @example
 * ```tsx
 * const expired = useExpiredFlags();
 * ```
 */
export function useExpiredFlags(): Flag[] {
  const client = useClient();
  const allFlags = useFlagSnapshot(client);

  return useMemo(() => {
    const now = Date.now();
    return allFlags.filter((f) => {
      if (f.isPermanent || !f.expiresAt) return false;
      return new Date(f.expiresAt).getTime() <= now;
    });
  }, [allFlags]);
}

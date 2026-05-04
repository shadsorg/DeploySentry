import { useEffect, useState } from 'react';
import { stagingApi, entitiesApi, settingsApi } from '@/api';

// Module-level cache: one entry per orgSlug. Holds either a Promise still
// in flight or a resolved boolean. All concurrent and subsequent callers
// share the same lookup. Cache survives the lifetime of the JS module
// (effectively the tab session).
const cache = new Map<string, Promise<boolean> | boolean>();

// Subscribers per orgSlug — every active hook instance for that slug.
// Notified when the cached value changes (e.g. via setStagingEnabled).
const subscribers = new Map<string, Set<(v: boolean) => void>>();

function notify(orgSlug: string, value: boolean) {
  const subs = subscribers.get(orgSlug);
  if (!subs) return;
  for (const s of subs) s(value);
}

function getCached(orgSlug: string): boolean | undefined {
  const v = cache.get(orgSlug);
  return typeof v === 'boolean' ? v : undefined;
}

function loadEnabled(orgSlug: string): Promise<boolean> {
  const existing = cache.get(orgSlug);
  if (typeof existing === 'boolean') return Promise.resolve(existing);
  if (existing instanceof Promise) return existing;
  const p = stagingApi
    .getEnabled(orgSlug)
    .then((res) => {
      const enabled = !!res.enabled;
      cache.set(orgSlug, enabled);
      notify(orgSlug, enabled);
      return enabled;
    })
    .catch((err) => {
      // Failed lookup — treat as off, but DON'T cache the failure so a
      // retry on the next mount can recover (avoids a one-bad-network-blip
      // marooning the gate at false for the whole session).
      cache.delete(orgSlug);
      console.error('useStagingEnabled: lookup failed for', orgSlug, err);
      return false;
    });
  cache.set(orgSlug, p);
  return p;
}

/**
 * Per-org gate for routing dashboard mutations through the staging layer
 * vs. writing to production directly. Backed by the org-level setting
 * `staged-changes-enabled`.
 *
 * Concurrent callers and subsequent mounts share a module-level cache so
 * rapid tab switches don't fan out into N parallel API calls. Returns
 * false until the first lookup resolves; pages that need to gate a click
 * handler on this should expect that race only on the very first mount
 * after page load.
 */
export function useStagingEnabled(orgSlug?: string): boolean {
  const [enabled, setEnabled] = useState<boolean>(() => {
    if (!orgSlug) return false;
    return getCached(orgSlug) ?? false;
  });

  useEffect(() => {
    if (!orgSlug) {
      setEnabled(false);
      return;
    }

    // Synchronous fast path for cache hits.
    const cached = getCached(orgSlug);
    if (cached !== undefined) {
      setEnabled(cached);
    } else {
      // Kick off (or join) the shared in-flight promise.
      // The subscription registered below will receive the update via notify()
      // once the promise settles, but we also set state directly here for
      // the initial mount in case the promise settles before the subscriber
      // is added (microtask timing).
      loadEnabled(orgSlug).then((v) => setEnabled(v));
    }

    // Subscribe so a `setStagingEnabled` call pushes updates to this instance.
    let subs = subscribers.get(orgSlug);
    if (!subs) {
      subs = new Set();
      subscribers.set(orgSlug, subs);
    }
    const sub = (v: boolean) => setEnabled(v);
    subs.add(sub);

    const onCustom = () => {
      // Custom event fired by setStagingEnabled — invalidate cache and refetch.
      cache.delete(orgSlug);
      loadEnabled(orgSlug).then((v) => setEnabled(v));
    };
    window.addEventListener('ds:staging-enabled', onCustom);

    return () => {
      subs!.delete(sub);
      if (subs!.size === 0) subscribers.delete(orgSlug);
      window.removeEventListener('ds:staging-enabled', onCustom);
    };
  }, [orgSlug]);

  return enabled;
}

/**
 * Imperative setter. Resolves slug → orgID, PUTs the org-level setting,
 * invalidates the local cache, and notifies all subscribers so any
 * in-tree useStagingEnabled instances refresh immediately.
 *
 * Returns a Promise so the caller can surface PUT failures to the user.
 * Fire-and-forget callers will silently lose errors.
 */
export async function setStagingEnabled(orgSlug: string, enabled: boolean): Promise<void> {
  const org = await entitiesApi.getOrg(orgSlug);
  await settingsApi.set({
    scope: 'org',
    target_id: org.id,
    key: 'staged-changes-enabled',
    value: enabled,
  });
  cache.set(orgSlug, enabled);
  notify(orgSlug, enabled);
  window.dispatchEvent(new CustomEvent('ds:staging-enabled'));
}

// Test-only: clear the module-level cache + subscribers between tests.
// Don't use in production code.
export function __resetStagingEnabledCacheForTests(): void {
  cache.clear();
  subscribers.clear();
}

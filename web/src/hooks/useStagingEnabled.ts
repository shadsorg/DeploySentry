import { useEffect, useState } from 'react';

/**
 * Per-org opt-in for routing dashboard mutations through the staging layer
 * instead of writing to production directly. Persisted in localStorage so
 * an operator can toggle it without server-side flag wiring; the proper
 * `staged-changes-enabled` feature flag will replace this later (Phase C
 * tail / Phase D), but the hook signature stays the same.
 *
 * Storage key: `ds_staging_enabled:<orgSlug>`. Value: "true" | absent.
 *
 * Listens to the `storage` event so toggling the key in another tab
 * propagates without a refresh, and to a custom `ds:staging-enabled`
 * event so a setter in this tab can broadcast within the same window.
 */
export function useStagingEnabled(orgSlug?: string): boolean {
  const key = orgSlug ? `ds_staging_enabled:${orgSlug}` : null;
  const [enabled, setEnabled] = useState<boolean>(() => readKey(key));

  useEffect(() => {
    if (!key) {
      setEnabled(false);
      return;
    }
    setEnabled(readKey(key));
    const onStorage = (e: StorageEvent) => {
      if (e.key === key) setEnabled(readKey(key));
    };
    const onCustom = () => setEnabled(readKey(key));
    window.addEventListener('storage', onStorage);
    window.addEventListener('ds:staging-enabled', onCustom);
    return () => {
      window.removeEventListener('storage', onStorage);
      window.removeEventListener('ds:staging-enabled', onCustom);
    };
  }, [key]);

  return enabled;
}

/**
 * Imperative setter for the same per-org flag. Useful for a Settings-page
 * toggle. Fires a `ds:staging-enabled` event so any same-tab listeners
 * (including useStagingEnabled) refresh immediately.
 */
export function setStagingEnabled(orgSlug: string, enabled: boolean): void {
  const key = `ds_staging_enabled:${orgSlug}`;
  if (enabled) {
    localStorage.setItem(key, 'true');
  } else {
    localStorage.removeItem(key);
  }
  window.dispatchEvent(new CustomEvent('ds:staging-enabled'));
}

function readKey(key: string | null): boolean {
  if (!key) return false;
  try {
    return localStorage.getItem(key) === 'true';
  } catch {
    return false;
  }
}

import { useEffect, useState } from 'react';
import { stagingApi, settingsApi, entitiesApi } from '@/api';

/**
 * Per-org opt-in for routing dashboard mutations through the staging layer.
 * Reads from GET /api/v1/orgs/:orgSlug/staging (server-side org setting).
 *
 * Defaults to false until the fetch resolves, and on any error.
 * Refetches when the custom `ds:staging-enabled` event fires (same-tab toggle).
 */
export function useStagingEnabled(orgSlug?: string): boolean {
  const [enabled, setEnabled] = useState<boolean>(false);

  useEffect(() => {
    if (!orgSlug) {
      setEnabled(false);
      return;
    }

    let cancelled = false;

    function fetchEnabled() {
      stagingApi.getEnabled(orgSlug!).then(
        (res) => {
          if (!cancelled) setEnabled(res.enabled);
        },
        () => {
          if (!cancelled) setEnabled(false);
        },
      );
    }

    fetchEnabled();

    const onCustom = () => fetchEnabled();
    window.addEventListener('ds:staging-enabled', onCustom);
    return () => {
      cancelled = true;
      window.removeEventListener('ds:staging-enabled', onCustom);
    };
  }, [orgSlug]);

  return enabled;
}

/**
 * Imperative setter — writes the staged-changes-enabled org-level setting via
 * the settings PUT endpoint, then dispatches `ds:staging-enabled` so the
 * same-tab hook refreshes.
 *
 * Resolves orgSlug → orgID first so the settings API gets the UUID it needs.
 */
export async function setStagingEnabled(orgSlug: string, enabled: boolean): Promise<void> {
  const org = await entitiesApi.getOrg(orgSlug);
  await settingsApi.set({
    scope: 'org',
    target_id: org.id,
    key: 'staged-changes-enabled',
    value: enabled,
  });
  window.dispatchEvent(new CustomEvent('ds:staging-enabled'));
}

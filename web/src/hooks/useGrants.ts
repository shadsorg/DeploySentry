import { useState, useEffect, useCallback } from 'react';
import { grantsApi } from '../api';
import type { ResourceGrant } from '../api';

export function useGrants(
  orgSlug: string | undefined,
  projectSlug: string | undefined,
  appSlug?: string | undefined,
) {
  const [grants, setGrants] = useState<ResourceGrant[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug || !projectSlug) {
      setGrants([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    const fetcher = appSlug
      ? grantsApi.listAppGrants(orgSlug, projectSlug, appSlug)
      : grantsApi.listProjectGrants(orgSlug, projectSlug);
    fetcher
      .then((g) => setGrants(g ?? []))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug, appSlug]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { grants, loading, error, refresh };
}

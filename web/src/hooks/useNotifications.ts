import { useCallback, useEffect, useState } from 'react';
import { notificationsApi, NotificationPreferences } from '../api';

export function useNotifications() {
  const [preferences, setPreferences] = useState<NotificationPreferences | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const refresh = useCallback(() => {
    setLoading(true);
    setError(null);
    notificationsApi
      .getPreferences()
      .then((res) => setPreferences(res))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const save = useCallback(
    async (data: Parameters<typeof notificationsApi.savePreferences>[0]) => {
      setSaving(true);
      try {
        await notificationsApi.savePreferences(data);
        await refresh();
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : 'Save failed';
        setError(message);
        throw err;
      } finally {
        setSaving(false);
      }
    },
    [refresh],
  );

  const reset = useCallback(async () => {
    setSaving(true);
    try {
      await notificationsApi.resetPreferences();
      await refresh();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Reset failed';
      setError(message);
      throw err;
    } finally {
      setSaving(false);
    }
  }, [refresh]);

  return { preferences, loading, error, saving, refresh, save, reset };
}

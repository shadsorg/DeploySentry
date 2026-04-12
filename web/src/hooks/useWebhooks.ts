import { useCallback, useEffect, useState } from 'react';
import { webhooksApi, Webhook } from '../api';

export function useWebhooks() {
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    setLoading(true);
    setError(null);
    webhooksApi
      .list()
      .then((res) => setWebhooks(res.webhooks ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { webhooks, loading, error, refresh };
}

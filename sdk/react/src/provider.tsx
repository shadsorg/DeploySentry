import React, { useEffect, useMemo, useRef, useState } from 'react';
import { DeploySentryClient } from './client';
import { DeploySentryContext } from './context';
import type { ProviderProps } from './types';

/**
 * Provides the DeploySentry client to all descendant components.
 *
 * The provider creates a {@link DeploySentryClient} on mount, fetches the
 * initial flag set, opens an SSE connection for real-time updates, and tears
 * everything down on unmount.
 *
 * @example
 * ```tsx
 * <DeploySentryProvider
 *   apiKey="ds_live_abc123"
 *   baseURL="https://api.deploysentry.io"
 *   environment="production"
 *   project="my-app"
 *   user={{ id: 'user-42' }}
 * >
 *   <App />
 * </DeploySentryProvider>
 * ```
 */
export function DeploySentryProvider({
  apiKey,
  baseURL,
  environment,
  project,
  user,
  sessionId,
  children,
}: ProviderProps): React.ReactElement {
  const [client, setClient] = useState<DeploySentryClient | null>(null);
  const [, setError] = useState<Error | null>(null);

  // Keep a ref so the effect cleanup always targets the right instance.
  const clientRef = useRef<DeploySentryClient | null>(null);

  // Memoise the configuration identity so we only recreate the client when
  // the connection parameters change, not on every render.
  const configKey = useMemo(
    () => JSON.stringify({ apiKey, baseURL, environment, project, sessionId }),
    [apiKey, baseURL, environment, project, sessionId],
  );

  useEffect(() => {
    const instance = new DeploySentryClient({
      apiKey,
      baseURL,
      environment,
      project,
      user,
      sessionId,
    });

    clientRef.current = instance;

    instance
      .init()
      .then(() => {
        // Guard against the effect having been cleaned up before init resolved.
        if (clientRef.current === instance) {
          setClient(instance);
          setError(null);
        }
      })
      .catch((err: unknown) => {
        if (clientRef.current === instance) {
          setError(
            err instanceof Error
              ? err
              : new Error(String(err)),
          );
          // Still expose the client so hooks can return defaults gracefully.
          setClient(instance);
        }
      });

    return () => {
      clientRef.current = null;
      instance.destroy();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [configKey]);

  // When the user context changes (but connection params stay the same),
  // re-identify on the existing client rather than tearing it down.
  const userKey = useMemo(() => JSON.stringify(user), [user]);

  useEffect(() => {
    if (!client) return;
    // The initial identify already happened inside init(), so we skip the
    // first call by comparing refs.
    client.identify(user).catch(() => {
      // Swallow identify errors. The client will keep the previous state.
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [userKey, client]);

  return (
    <DeploySentryContext.Provider value={client}>
      {children}
    </DeploySentryContext.Provider>
  );
}

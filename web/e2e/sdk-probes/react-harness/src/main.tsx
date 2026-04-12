import React, { useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { DeploySentryProvider, useFlag } from '@deploysentry/react';
import type { UserContext } from '@deploysentry/react';

declare global {
  interface Window {
    __ds_observations: Array<{ flagKey: string; value: unknown; ts: number }>;
  }
}
window.__ds_observations = [];

interface HarnessConfig {
  apiUrl: string;
  apiKey: string;
  project: string;
  environment: string;
  flagKeys: string[];
  user: UserContext | undefined;
}

function parseQuery(): HarnessConfig {
  const q = new URLSearchParams(window.location.search);
  const contextRaw = q.get('context');
  let user: UserContext | undefined;
  if (contextRaw) {
    try {
      const parsed = JSON.parse(contextRaw) as Partial<UserContext> & Record<string, unknown>;
      if (parsed && typeof parsed.id === 'string') {
        user = {
          id: parsed.id,
          attributes: (parsed.attributes as Record<string, string> | undefined) ?? undefined,
        };
      }
    } catch {
      // ignore parse errors; user stays undefined
    }
  }
  return {
    apiUrl: q.get('apiUrl') ?? '',
    apiKey: q.get('apiKey') ?? '',
    project: q.get('project') ?? '',
    environment: q.get('environment') ?? '',
    flagKeys: (q.get('flagKeys') ?? '').split(',').filter(Boolean),
    user,
  };
}

function Observer({ flagKey }: { flagKey: string }): null {
  const isVariant = flagKey.startsWith('variant:');
  const realKey = isVariant ? flagKey.slice('variant:'.length) : flagKey;

  // For boolean flags, useFlag returns `flag.enabled ? flag.value : default`.
  // We use a sentinel default ("__ds_disabled__") so that when the flag is
  // disabled we observe the sentinel, and when enabled we observe the actual
  // value — making toggles visible regardless of the stored default_value.
  const sentinel = isVariant ? 'control' : '__ds_disabled__';
  const raw = useFlag<string | boolean>(realKey, sentinel);
  const value = raw === '__ds_disabled__' ? false : (raw === 'false' ? false : raw === 'true' ? true : raw);

  useEffect(() => {
    window.__ds_observations.push({ flagKey, value, ts: Date.now() });
  }, [flagKey, value]);
  return null;
}

function App(): React.ReactElement {
  const cfg = parseQuery();
  return (
    <DeploySentryProvider
      apiKey={cfg.apiKey}
      baseURL={cfg.apiUrl}
      project={cfg.project}
      environment={cfg.environment}
      user={cfg.user}
    >
      {cfg.flagKeys.map((k) => (
        <Observer key={k} flagKey={k} />
      ))}
    </DeploySentryProvider>
  );
}

const rootEl = document.getElementById('root');
if (!rootEl) throw new Error('root element missing');
createRoot(rootEl).render(<App />);

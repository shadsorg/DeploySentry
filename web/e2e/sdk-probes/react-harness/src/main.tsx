import React, { useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { DeploySentryProvider, useFlag, useFlagDetail } from '@dr-sentry/react';
import type { UserContext } from '@dr-sentry/react';

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
  application: string;
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
    application: q.get('application') ?? '',
    environment: q.get('environment') ?? '',
    flagKeys: (q.get('flagKeys') ?? '').split(',').filter(Boolean),
    user,
  };
}

// eslint-disable-next-line react-refresh/only-export-components
function Observer({ flagKey }: { flagKey: string }): null {
  const isVariant = flagKey.startsWith('variant:');
  const realKey = isVariant ? flagKey.slice('variant:'.length) : flagKey;

  // For variant (string) flags, observe the resolved value via useFlag.
  // For boolean flags, observe the `enabled` state via useFlagDetail —
  // boolean toggles only flip `enabled`, not the stored default_value, so
  // the Node probe reads `detail.enabled` and we must mirror that.
  const detail = useFlagDetail<string>(realKey);
  const variant = useFlag<string>(realKey, 'control');
  const value: string | boolean = isVariant ? variant : detail.enabled;

  useEffect(() => {
    window.__ds_observations.push({ flagKey, value, ts: Date.now() });
  }, [flagKey, value]);
  return null;
}

// eslint-disable-next-line react-refresh/only-export-components
function App(): React.ReactElement {
  const cfg = parseQuery();
  return (
    <DeploySentryProvider
      apiKey={cfg.apiKey}
      baseURL={cfg.apiUrl}
      project={cfg.project}
      application={cfg.application}
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

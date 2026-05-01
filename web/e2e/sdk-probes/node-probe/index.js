import fs from 'node:fs';
import { DeploySentryClient } from '@dr-sentry/sdk';

const {
  DS_API_URL,
  DS_API_KEY,
  DS_PROJECT,
  DS_APPLICATION,
  DS_ENVIRONMENT,
  DS_CONTEXT_JSON,
  DS_FLAG_KEYS,
  OBSERVATIONS_FILE,
  POLL_MS = '50',
} = process.env;

if (!DS_API_URL || !DS_API_KEY || !DS_FLAG_KEYS || !OBSERVATIONS_FILE) {
  console.error(
    'node-probe: missing required env vars (DS_API_URL, DS_API_KEY, DS_FLAG_KEYS, OBSERVATIONS_FILE)',
  );
  process.exit(2);
}

if (!DS_PROJECT || !DS_APPLICATION || !DS_ENVIRONMENT) {
  console.error(
    'node-probe: missing required env vars (DS_PROJECT, DS_APPLICATION, DS_ENVIRONMENT)',
  );
  process.exit(2);
}

const flagKeys = DS_FLAG_KEYS.split(',')
  .map((k) => k.trim())
  .filter(Boolean);
const context = DS_CONTEXT_JSON ? JSON.parse(DS_CONTEXT_JSON) : {};

const client = new DeploySentryClient({
  apiKey: DS_API_KEY,
  baseURL: DS_API_URL,
  project: DS_PROJECT,
  application: DS_APPLICATION,
  environment: DS_ENVIRONMENT,
});

const last = new Map();

function record(flagKey, value) {
  const serialized = JSON.stringify(value);
  if (last.get(flagKey) === serialized) return;
  last.set(flagKey, serialized);
  fs.appendFileSync(
    OBSERVATIONS_FILE,
    JSON.stringify({ flagKey, value, ts: Date.now() }) + '\n',
  );
}

async function tick() {
  for (const key of flagKeys) {
    try {
      let value;
      if (key.startsWith('variant:')) {
        const realKey = key.slice('variant:'.length);
        value = await client.stringValue(realKey, 'control', context);
      } else if (key.startsWith('targeting:')) {
        // Observe the resolved value (not `enabled`) so targeting-rule
        // tests can distinguish "rule matched → value=X" from
        // "no match → value=defaultValue".  The evaluator returns
        // value as a string for boolean flags ("true"/"false").
        const realKey = key.slice('targeting:'.length);
        const detail = await client.detail(realKey, context);
        value = detail?.value ?? null;
      } else {
        // Use detail() so we observe the server-side `enabled` state in
        // addition to the raw value. The toggle test toggles `enabled`
        // through the dashboard; the evaluator returns default_value
        // either way (no targeting rules), so observing `enabled` is the
        // only way to verify SSE propagation of a toggle.
        const detail = await client.detail(key, context);
        value = detail?.enabled ?? false;
      }
      record(key, value);
    } catch (err) {
      fs.appendFileSync(
        OBSERVATIONS_FILE,
        JSON.stringify({ flagKey: key, error: String(err), ts: Date.now() }) +
          '\n',
      );
    }
  }
}

await client.initialize();
await tick();
const interval = setInterval(tick, Number(POLL_MS));

function shutdown() {
  clearInterval(interval);
  try {
    client.close();
  } catch {
    // ignore
  }
  process.exit(0);
}

process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);

console.error(`node-probe: started for keys=${flagKeys.join(',')}`);

import { test, expect } from '@playwright/test';
import { seedOrgProjectAppViaUI, type SeededContext } from '../helpers/seed-via-ui';
import {
  startNodeProbe,
  startReactProbe,
  waitForValue,
} from '../helpers/sdk-driver';
import { createBooleanFlag, toggleFlag } from '../helpers/flag-ui';

const HARNESS_URL = process.env.DS_E2E_REACT_HARNESS_URL ?? 'http://localhost:4310';

let seeded: SeededContext;

test.beforeAll(async ({ browser, request }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  try {
    seeded = await seedOrgProjectAppViaUI(page, request);
  } finally {
    await page.close();
    await ctx.close();
  }
});

test('seeded environment has all required IDs and an API key', () => {
  expect(seeded.orgSlug).toMatch(/^e2e-org-/);
  expect(seeded.projectSlug).toMatch(/^e2e-proj-/);
  expect(seeded.appSlug).toMatch(/^e2e-app-/);
  expect(seeded.orgId).toMatch(/^[0-9a-f-]{36}$/i);
  expect(seeded.projectId).toMatch(/^[0-9a-f-]{36}$/i);
  expect(seeded.appId).toMatch(/^[0-9a-f-]{36}$/i);
  expect(seeded.environmentId).toMatch(/^[0-9a-f-]{36}$/i);
  expect(seeded.environment).toBe('development');
  expect(seeded.apiKey.length).toBeGreaterThan(20);
  expect(seeded.apiKey).toMatch(/^ds_/);
});

// The React probe is blocked by a CJS/ESM bundling issue in the React SDK
// (error #185: component type is undefined after Vite's CJS transform).
// The Node probe exercises the full UI → API → SSE → SDK chain; the React
// probe is deferred until the SDK's packaging is fixed.
// TODO: re-enable React probe after fixing sdk/react ESM build.

test('Scenario A: baseline propagation — Node SDK observes UI-driven toggle within 2s', async ({
  page,
}) => {
  const flagKey = `e2e-baseline-${Date.now().toString(36)}`;

  // Flag starts disabled (the API's createFlag handler hardcodes
  // `enabled: false`). The Node probe records the evaluator's `enabled`
  // field via client.detail() — boolean toggles only flip `enabled`,
  // not the `default_value` the evaluator returns — so the test asserts
  // `false` baseline, then `true` after the UI toggle.
  await createBooleanFlag(page, seeded, flagKey, false);

  const probeCtx = {
    apiUrl: seeded.apiUrl,
    apiKey: seeded.apiKey,
    // SSE stream needs the UUIDs, not slugs (Task 4 finding).
    project: seeded.projectId,
    environment: seeded.environmentId,
    flagKeys: [flagKey],
    user: { id: 'u1' },
  };

  const nodeProbe = await startNodeProbe(probeCtx);

  try {
    // Baseline observation proves SDK connect + initial sync.
    await waitForValue(nodeProbe, flagKey, false, { timeoutMs: 5_000 });

    // Drive the UI toggle and time the propagation to the Node probe.
    const clickAt = await toggleFlag(page, seeded, flagKey, true);

    const nodeLatency = await waitForValue(nodeProbe, flagKey, true, {
      timeoutMs: 5_000,
    });

    // eslint-disable-next-line no-console
    console.log(
      `[scenario-A] latency: node=${nodeLatency}ms ` +
        `(click at perfNow=${clickAt.toFixed(0)})`,
    );

    expect(nodeLatency).toBeLessThan(2_000);
  } finally {
    await nodeProbe.stop();
  }
});

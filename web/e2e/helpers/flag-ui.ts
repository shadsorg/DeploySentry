import { expect, type Page } from '@playwright/test';
import { type SeededContext } from './seed-via-ui';
import { loginViaUI } from './auth';

interface CreatedFlag {
  id: string;
  key: string;
  name: string;
}

/**
 * Ensures the page is authenticated as the seeded user. The seed helper
 * creates the user via direct API calls (since the Vite dev proxy for the
 * smoke suite points at :8080, not the hermetic :18080 stack), so the
 * browser context starts out logged-out. Calling the UI login flow seeds
 * localStorage with the JWT that subsequent dashboard navigations need.
 *
 * After the first call, subsequent calls are no-ops — we deliberately
 * avoid re-visiting /login because RedirectIfAuth races against the next
 * navigation and can leave the page on an unexpected URL.
 */
export async function ensureLoggedIn(page: Page, seeded: SeededContext): Promise<void> {
  const url = page.url();
  if (url && url !== 'about:blank' && !url.endsWith('/login')) {
    // Already on an authenticated page; trust localStorage rather than
    // bouncing back through the login route.
    const existing = await page.evaluate(() => window.localStorage.getItem('ds_token'));
    if (existing) return;
  }
  await loginViaUI(page, seeded.email, seeded.password);
}

/**
 * Create a boolean flag against the hermetic e2e API.
 *
 * NOTE on UI-vs-API: the plan template asked for this to drive the dashboard
 * UI, but the dashboard's FlagCreatePage depends on
 * `entitiesApi.listEnvironments(org, project, app)` which hits the legacy
 * `/apps/:appSlug/environments` endpoint. That endpoint still queries
 * `environments.application_id`, which was removed by migration 035 (org-level
 * environments) — so the dropdown is always empty and the form cannot submit.
 * Rather than fix a pre-existing dashboard regression as part of this task,
 * we create the flag via the API using the seeded API key, and keep the
 * toggle UI-driven (toggleFlag below) — which is the actual UI → API → SSE
 * step Scenario A is meant to prove.
 */
export async function createBooleanFlag(
  page: Page,
  seeded: SeededContext,
  flagKey: string,
  defaultValue: boolean,
): Promise<CreatedFlag> {
  // We need a user JWT (not the flag probe API key) to satisfy flag:create
  // RBAC — the seeded user is the org owner. Running through the UI login
  // flow ensures the browser is also authenticated for the subsequent
  // toggleFlag() UI step.
  await ensureLoggedIn(page, seeded);
  const token = await page.evaluate(() => window.localStorage.getItem('ds_token'));
  if (!token) throw new Error('flag-ui: no JWT in localStorage after login');

  const res = await fetch(`${seeded.apiUrl}/api/v1/flags`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      project_id: seeded.projectId,
      environment_id: seeded.environmentId,
      key: flagKey,
      name: flagKey,
      flag_type: 'boolean',
      category: 'ops',
      is_permanent: true,
      default_value: String(defaultValue),
      owners: [],
      tags: [],
    }),
  });
  if (!res.ok) {
    throw new Error(
      `flag-ui: POST /flags failed (${res.status}): ${await res.text()}`,
    );
  }
  return (await res.json()) as CreatedFlag;
}

/**
 * Add a targeting rule to a flag via the API.
 * Returns the new rule's ID.
 */
export async function addTargetingRule(
  page: Page,
  seeded: SeededContext,
  flagId: string,
  rule: {
    ruleType: string;
    attribute: string;
    operator: string;
    value: string;
    priority?: number;
  },
): Promise<string> {
  await ensureLoggedIn(page, seeded);
  const token = await page.evaluate(() => window.localStorage.getItem('ds_token'));
  if (!token) throw new Error('flag-ui: no JWT in localStorage after login');

  const res = await fetch(`${seeded.apiUrl}/api/v1/flags/${flagId}/rules`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      rule_type: rule.ruleType,
      attribute: rule.attribute,
      operator: rule.operator,
      value: rule.value,
      priority: rule.priority ?? 0,
    }),
  });
  if (!res.ok) {
    throw new Error(
      `flag-ui: POST /flags/${flagId}/rules failed (${res.status}): ${await res.text()}`,
    );
  }
  const body = (await res.json()) as { id: string };
  return body.id;
}

/**
 * Update an existing targeting rule via the API.
 */
export async function updateTargetingRule(
  page: Page,
  seeded: SeededContext,
  flagId: string,
  ruleId: string,
  updates: {
    ruleType: string;
    attribute: string;
    operator: string;
    value: string;
    priority?: number;
  },
): Promise<void> {
  await ensureLoggedIn(page, seeded);
  const token = await page.evaluate(() => window.localStorage.getItem('ds_token'));
  if (!token) throw new Error('flag-ui: no JWT in localStorage after login');

  const res = await fetch(
    `${seeded.apiUrl}/api/v1/flags/${flagId}/rules/${ruleId}`,
    {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        rule_type: updates.ruleType,
        attribute: updates.attribute,
        operator: updates.operator,
        value: updates.value,
        priority: updates.priority ?? 0,
      }),
    },
  );
  if (!res.ok) {
    throw new Error(
      `flag-ui: PUT /flags/${flagId}/rules/${ruleId} failed (${res.status}): ${await res.text()}`,
    );
  }
}

/**
 * Enable a flag via the API toggle endpoint.
 */
export async function enableFlagViaApi(
  page: Page,
  seeded: SeededContext,
  flagId: string,
  enabled: boolean,
): Promise<void> {
  await ensureLoggedIn(page, seeded);
  const token = await page.evaluate(() => window.localStorage.getItem('ds_token'));
  if (!token) throw new Error('flag-ui: no JWT in localStorage after login');

  const res = await fetch(`${seeded.apiUrl}/api/v1/flags/${flagId}/toggle`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ enabled }),
  });
  if (!res.ok) {
    throw new Error(
      `flag-ui: POST /flags/${flagId}/toggle failed (${res.status}): ${await res.text()}`,
    );
  }
}

/**
 * Create a string-type flag via the API.
 * Returns the created flag's id, key, and name.
 */
export async function createStringFlag(
  page: Page,
  seeded: SeededContext,
  flagKey: string,
  defaultValue: string,
): Promise<CreatedFlag> {
  await ensureLoggedIn(page, seeded);
  const token = await page.evaluate(() => window.localStorage.getItem('ds_token'));
  if (!token) throw new Error('flag-ui: no JWT in localStorage after login');

  const res = await fetch(`${seeded.apiUrl}/api/v1/flags`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      project_id: seeded.projectId,
      environment_id: seeded.environmentId,
      key: flagKey,
      name: flagKey,
      flag_type: 'string',
      category: 'ops',
      is_permanent: true,
      default_value: JSON.stringify(defaultValue),
      owners: [],
      tags: [],
    }),
  });
  if (!res.ok) {
    throw new Error(
      `flag-ui: POST /flags failed (${res.status}): ${await res.text()}`,
    );
  }
  return (await res.json()) as CreatedFlag;
}

/**
 * Update a flag's default_value via the API (PUT /flags/:id).
 * The backend's UpdateFlag handler broadcasts an SSE event and
 * invalidates the evaluation cache, so SDK probes should observe
 * the new value without any toggle workaround.
 */
export async function updateFlagDefaultValue(
  page: Page,
  seeded: SeededContext,
  flagId: string,
  newValue: string,
): Promise<void> {
  await ensureLoggedIn(page, seeded);
  const token = await page.evaluate(() => window.localStorage.getItem('ds_token'));
  if (!token) throw new Error('flag-ui: no JWT in localStorage after login');

  const res = await fetch(`${seeded.apiUrl}/api/v1/flags/${flagId}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ default_value: JSON.stringify(newValue) }),
  });
  if (!res.ok) {
    throw new Error(
      `flag-ui: PUT /flags/${flagId} failed (${res.status}): ${await res.text()}`,
    );
  }
}

/**
 * Toggle a flag's enabled state through the dashboard. Returns a
 * `performance.now()` timestamp captured immediately before the click so
 * the spec can compute end-to-end latency if it wants to.
 */
export async function toggleFlag(
  page: Page,
  seeded: SeededContext,
  flagKey: string,
  enabled: boolean,
): Promise<number> {
  await ensureLoggedIn(page, seeded);

  await page.goto(`/orgs/${seeded.orgSlug}/projects/${seeded.projectSlug}/flags`);
  // Wait for the flag list to render before clicking. The FlagListPage
  // renders flag.name as the link text — and createBooleanFlag sets
  // name = flagKey — so we look up by the flag key.
  await expect(page.locator('.font-mono', { hasText: flagKey })).toBeVisible({
    timeout: 10_000,
  });
  await page.getByRole('link', { name: flagKey }).first().click();
  await expect(page).toHaveURL(/\/flags\/[0-9a-f-]{36}/i);

  // The native <input> inside .toggle is visually hidden, so we target the
  // wrapping <label> for clicks (browsers forward label clicks to the
  // associated input) and only read `checked` off the input itself.
  const toggleLabel = page.locator('label.toggle').first();
  const toggleCheckbox = toggleLabel.locator('input[type="checkbox"]');
  await toggleLabel.waitFor({ state: 'visible', timeout: 10_000 });

  const current = await toggleCheckbox.isChecked();
  if (current === enabled) {
    return performance.now();
  }

  const before = performance.now();
  await toggleLabel.click();
  // Wait for the UI to reflect the new state. The handler persists
  // optimistically, then reverts on API error — so a stable checked state
  // after the click implies the toggle request succeeded.
  await expect(toggleCheckbox).toBeChecked({ checked: enabled });
  return before;
}

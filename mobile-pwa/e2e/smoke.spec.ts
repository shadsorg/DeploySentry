import { test, expect } from '@playwright/test';

test('login screen renders on first visit', async ({ page }) => {
  await page.goto('/m/');
  await expect(page.getByRole('heading', { name: /deploy sentry/i })).toBeVisible();
  await expect(page.getByLabel('Email')).toBeVisible();
  await expect(page.getByLabel('Password')).toBeVisible();
  await expect(page.getByRole('button', { name: /sign in$/i })).toBeVisible();
});

test('unauthenticated deep-link bounces to /login with next param', async ({ page }) => {
  await page.goto('/m/orgs/acme/status');
  await expect(page).toHaveURL(/\/m\/login\?next=%2Forgs%2Facme%2Fstatus$/);
});

test.skip('manifest is reachable and valid JSON', async () => {
  // Skipped: vite dev server returns index.html for unknown paths.
  // Manifest validity is verified by the `vite build` smoke in Task 11 /
  // by the prod preview server in follow-up Playwright setups.
});

test('authenticated status page smoke (API mocked at browser level)', async ({ page, context }) => {
  await context.addInitScript(() => {
    const toB64u = (s: string) =>
      btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const payload = { exp: Math.floor(Date.now() / 1000) + 3600 };
    const token = `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify(payload))}.sig`;
    localStorage.setItem('ds_token', token);
  });
  await context.route('**/api/v1/users/me', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }),
    }),
  );
  await context.route('**/api/v1/orgs', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        organizations: [{ id: '1', slug: 'acme', name: 'Acme', created_at: '', updated_at: '' }],
      }),
    }),
  );
  await context.route('**/api/v1/orgs/acme/status', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        org: { id: '1', slug: 'acme', name: 'Acme' },
        generated_at: '2026-04-24T00:00:00Z',
        projects: [
          {
            project: { id: 'p1', slug: 'payments', name: 'Payments' },
            aggregate_health: 'healthy',
            applications: [],
          },
        ],
      }),
    }),
  );

  await page.goto('/m/orgs/acme/status');
  await page.getByText('Payments').waitFor({ state: 'visible' });
  await page.getByRole('button', { name: /flags/i }).waitFor({ state: 'visible' });
});

test('authenticated history page smoke (API mocked at browser level)', async ({ page, context }) => {
  await context.addInitScript(() => {
    const toB64u = (s: string) =>
      btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const payload = { exp: Math.floor(Date.now() / 1000) + 3600 };
    const token = `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify(payload))}.sig`;
    localStorage.setItem('ds_token', token);
  });
  await context.route('**/api/v1/users/me', (r) =>
    r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }) }),
  );
  await context.route('**/api/v1/orgs/acme/projects', (r) =>
    r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ projects: [] }) }),
  );
  await context.route('**/api/v1/orgs/acme/deployments**', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        deployments: [
          {
            id: 'd1',
            application_id: 'a1',
            environment_id: 'e1',
            version: 'v9.9.9',
            strategy: 'canary',
            status: 'completed',
            traffic_percent: 100,
            created_by: 'u1',
            created_at: new Date(Date.now() - 60_000).toISOString(),
            updated_at: '',
            completed_at: null,
            application: { id: 'a1', slug: 'api', name: 'API' },
            environment: { id: 'e1', slug: 'prod', name: 'Production' },
            project: { id: 'p1', slug: 'pay', name: 'Payments' },
          },
        ],
      }),
    }),
  );
  await page.goto('/m/orgs/acme/history');
  await page.getByText('v9.9.9').waitFor({ state: 'visible' });
  await page.getByRole('button', { name: 'Failed' }).click();
});

test('authenticated flags page smoke (API mocked at browser level)', async ({ page, context }) => {
  await context.addInitScript(() => {
    const toB64u = (s: string) =>
      btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const payload = { exp: Math.floor(Date.now() / 1000) + 3600 };
    const token = `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify(payload))}.sig`;
    localStorage.setItem('ds_token', token);
  });
  await context.route('**/api/v1/users/me', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }),
    }),
  );
  await context.route('**/api/v1/orgs/acme/projects', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        projects: [{ id: 'p1', slug: 'payments', name: 'Payments' }],
      }),
    }),
  );
  await context.route('**/api/v1/orgs/acme/environments', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ environments: [] }),
    }),
  );
  await context.route('**/api/v1/flags**', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ flags: [] }),
    }),
  );

  // Single-project org auto-redirects from /flags to /flags/:projectSlug,
  // so the FlagListPage renders. With zero flags we expect the empty state.
  await page.goto('/m/orgs/acme/flags');
  const flagRow = page.locator('a[href*="/flags/payments/"]').first();
  const emptyState = page.getByText('No flags in this project.');
  await expect(flagRow.or(emptyState)).toBeVisible();
});

test('flag detail env toggle issues PUT (API mocked at browser level)', async ({ page, context }) => {
  await context.addInitScript(() => {
    const toB64u = (s: string) =>
      btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const payload = { exp: Math.floor(Date.now() / 1000) + 3600 };
    const token = `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify(payload))}.sig`;
    localStorage.setItem('ds_token', token);
  });
  await context.route('**/api/v1/users/me', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }),
    }),
  );
  await context.route('**/api/v1/orgs/acme/environments', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        environments: [
          { id: 'env-dev', slug: 'dev', name: 'Development', sort_order: 1 },
        ],
      }),
    }),
  );
  // Order matters: the most-specific routes must be registered first so the
  // generic /flags/:id matcher doesn't swallow them.
  await context.route('**/api/v1/flags/flag-1/rules/environment-states', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        rule_environment_states: [
          { rule_id: 'rule-1', environment_id: 'env-dev', enabled: true },
        ],
      }),
    }),
  );
  await context.route('**/api/v1/flags/flag-1/rules', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        rules: [
          {
            id: 'rule-1',
            flag_id: 'flag-1',
            rule_type: 'percentage',
            percentage: 25,
            value: 'true',
            priority: 1,
            created_at: '',
            updated_at: '',
          },
        ],
      }),
    }),
  );
  await context.route('**/api/v1/flags/flag-1/environments/env-dev', (route) => {
    if (route.request().method() === 'PUT') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          flag_id: 'flag-1',
          environment_id: 'env-dev',
          enabled: true,
          value: 'true',
        }),
      });
    }
    return route.continue();
  });
  await context.route('**/api/v1/flags/flag-1/environments', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        environment_states: [
          {
            flag_id: 'flag-1',
            environment_id: 'env-dev',
            enabled: false,
            value: 'false',
          },
        ],
      }),
    }),
  );
  await context.route('**/api/v1/flags/flag-1', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: 'flag-1',
        project_id: 'proj-1',
        application_id: null,
        key: 'checkout_v2',
        name: 'Checkout v2',
        flag_type: 'boolean',
        category: 'release',
        is_permanent: false,
        expires_at: '2026-12-31T00:00:00Z',
        default_value: 'false',
        enabled: true,
        archived: false,
        owners: ['alice'],
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      }),
    }),
  );
  await context.route('**/api/v1/audit-log**', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ entries: [], total: 0 }),
    }),
  );

  await page.goto('/m/orgs/acme/flags/payments/flag-1');
  await page.getByText('checkout_v2').waitFor({ state: 'visible' });

  // Tap the env toggle and assert the PUT was issued.
  const putRequest = page.waitForRequest(
    (req) =>
      req.method() === 'PUT' &&
      /\/api\/v1\/flags\/flag-1\/environments\/env-dev$/.test(req.url()),
  );
  await page.getByRole('switch', { name: /Toggle Development/i }).click();
  const req = await putRequest;
  expect(req.method()).toBe('PUT');
  expect(JSON.parse(req.postData() ?? '{}')).toMatchObject({ enabled: true });
});

test('flag detail offline write shows OfflineWriteBlockedModal (API mocked)', async ({
  page,
  context,
}) => {
  await context.addInitScript(() => {
    const toB64u = (s: string) =>
      btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const payload = { exp: Math.floor(Date.now() / 1000) + 3600 };
    const token = `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify(payload))}.sig`;
    localStorage.setItem('ds_token', token);
  });
  await context.route('**/api/v1/users/me', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }),
    }),
  );
  await context.route('**/api/v1/orgs/acme/environments', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        environments: [
          { id: 'env-dev', slug: 'dev', name: 'Development', sort_order: 1 },
        ],
      }),
    }),
  );
  // Order matters: most-specific routes first.
  await context.route('**/api/v1/flags/flag-1/rules/environment-states', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        rule_environment_states: [
          { rule_id: 'rule-1', environment_id: 'env-dev', enabled: true },
        ],
      }),
    }),
  );
  await context.route('**/api/v1/flags/flag-1/rules', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        rules: [
          {
            id: 'rule-1',
            flag_id: 'flag-1',
            rule_type: 'percentage',
            percentage: 25,
            value: 'true',
            priority: 1,
            created_at: '',
            updated_at: '',
          },
        ],
      }),
    }),
  );
  await context.route('**/api/v1/flags/flag-1/environments', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        environment_states: [
          {
            flag_id: 'flag-1',
            environment_id: 'env-dev',
            enabled: false,
            value: 'false',
          },
        ],
      }),
    }),
  );
  await context.route('**/api/v1/flags/flag-1', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: 'flag-1',
        project_id: 'proj-1',
        application_id: null,
        key: 'checkout_v2',
        name: 'Checkout v2',
        flag_type: 'boolean',
        category: 'release',
        is_permanent: false,
        expires_at: '2026-12-31T00:00:00Z',
        default_value: 'false',
        enabled: true,
        archived: false,
        owners: ['alice'],
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      }),
    }),
  );
  await context.route('**/api/v1/audit-log**', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ entries: [], total: 0 }),
    }),
  );

  await page.goto('/m/orgs/acme/flags/payments/flag-1');
  await page.getByText('checkout_v2').waitFor({ state: 'visible' });

  // Flip the page context offline. The api client checks navigator.onLine
  // synchronously and throws OfflineWriteBlockedError before fetch fires,
  // so no PUT will be issued.
  await context.setOffline(true);
  await page.getByRole('switch', { name: /Toggle Development/i }).click();

  // The modal renders an alertdialog with the "You're offline" heading.
  await expect(page.getByRole('heading', { name: "You're offline" })).toBeVisible();

  // Restore online state, then dismiss with "Got it".
  await context.setOffline(false);
  await page.getByRole('button', { name: 'Got it' }).click();
  await expect(page.getByRole('heading', { name: "You're offline" })).toBeHidden();
});

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

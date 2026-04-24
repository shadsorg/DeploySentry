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

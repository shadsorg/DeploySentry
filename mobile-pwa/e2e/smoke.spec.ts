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

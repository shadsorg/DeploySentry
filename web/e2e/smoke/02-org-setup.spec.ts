import { test, expect } from '@playwright/test';
import path from 'path';

const AUTH_FILE = path.join(__dirname, '..', '.auth', 'user.json');

test.use({ storageState: AUTH_FILE });
test.describe.configure({ mode: 'serial' });

const ORG_NAME = 'E2E Test Org';
const PROJECT_NAME = 'E2E Test Project';
const APP_NAME = 'E2E Test App';

// Derived slugs (matches the app's slug generation logic)
const ORG_SLUG = 'e2e-test-org';
const PROJECT_SLUG = 'e2e-test-project';
const APP_SLUG = 'e2e-test-app';

test('create org → URL contains org slug', async ({ page }) => {
  await page.goto('/orgs/new');
  await page.getByPlaceholder(/acme corp/i).fill(ORG_NAME);
  // Slug is auto-generated; wait for it, then submit
  await expect(page.getByPlaceholder(/acme-corp/i)).toHaveValue(ORG_SLUG);
  await page.getByRole('button', { name: /create organization/i }).click();
  await expect(page).toHaveURL(new RegExp(ORG_SLUG));
});

test('create project → URL contains project slug', async ({ page }) => {
  await page.goto(`/orgs/${ORG_SLUG}/projects/new`);
  await page.getByPlaceholder(/my project/i).fill(PROJECT_NAME);
  await expect(page.getByPlaceholder(/my-project/i)).toHaveValue(PROJECT_SLUG);
  await page.getByRole('button', { name: /create project/i }).click();
  await expect(page).toHaveURL(new RegExp(PROJECT_SLUG));
});

test('create app → URL contains app slug', async ({ page }) => {
  await page.goto(`/orgs/${ORG_SLUG}/projects/${PROJECT_SLUG}/apps/new`);
  await page.getByPlaceholder(/api server/i).fill(APP_NAME);
  await expect(page.getByPlaceholder(/api-server/i)).toHaveValue(APP_SLUG);
  await page.getByRole('button', { name: /create application/i }).click();
  await expect(page).toHaveURL(new RegExp(APP_SLUG));
});

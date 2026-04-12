import { test, expect } from '@playwright/test';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const AUTH_FILE = path.join(__dirname, '..', '.auth', 'user.json');

test.use({ storageState: AUTH_FILE });

const ORG_SLUG = 'e2e-test-org';
const KEY_NAME = 'E2E Smoke Key';

test('navigate to API keys page', async ({ page }) => {
  await page.goto(`/orgs/${ORG_SLUG}/api-keys`);
  await expect(page.getByRole('heading', { name: /api keys/i })).toBeVisible();
});

test('create key with name and scope → ds_ token revealed', async ({ page }) => {
  await page.goto(`/orgs/${ORG_SLUG}/api-keys`);

  // Open the create form
  await page.getByRole('button', { name: /create api key/i }).click();

  // Fill name
  await page.getByPlaceholder(/production backend/i).fill(KEY_NAME);

  // Select at least one scope — pick flags:read
  await page.getByRole('checkbox', { name: 'flags:read' }).check();

  // Submit
  await page.getByRole('button', { name: /create key/i }).click();

  // The revealed key should contain the ds_ prefix
  await expect(page.locator('.key-reveal code')).toBeVisible();
  const revealedText = await page.locator('.key-reveal code').textContent();
  expect(revealedText).toMatch(/^ds_/);
});

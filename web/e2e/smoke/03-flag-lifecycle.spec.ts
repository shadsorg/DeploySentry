import { test, expect } from '@playwright/test';
import path from 'path';

const AUTH_FILE = path.join(__dirname, '..', '.auth', 'user.json');

test.use({ storageState: AUTH_FILE });
test.describe.configure({ mode: 'serial' });

const ORG_SLUG = 'e2e-test-org';
const PROJECT_SLUG = 'e2e-test-project';
const FLAG_KEY = 'e2e-smoke-flag';
const FLAG_NAME = 'E2E Smoke Flag';

test('create a boolean flag → navigates back to flags list', async ({ page }) => {
  await page.goto(`/orgs/${ORG_SLUG}/projects/${PROJECT_SLUG}/flags/new`);

  // Fill in key and name
  await page.locator('#flag-key').fill(FLAG_KEY);
  await page.locator('#flag-name').fill(FLAG_NAME);

  // Flag type should already be boolean (default), but make sure
  await page.locator('#flag-type').selectOption('boolean');

  // Category: feature (default)
  await page.locator('#flag-category').selectOption('feature');

  // Mark as permanent so we don't need an expiry date
  await page.getByText(/permanent flag/i).locator('..').locator('input[type="checkbox"]').check();

  // Default value
  await page.locator('#flag-default').fill('false');

  await page.getByRole('button', { name: /create flag/i }).click();

  // Should navigate back to the flags list
  await expect(page).toHaveURL(new RegExp(`/orgs/${ORG_SLUG}/projects/${PROJECT_SLUG}/flags`));
});

test('flag appears in the flags list', async ({ page }) => {
  await page.goto(`/orgs/${ORG_SLUG}/projects/${PROJECT_SLUG}/flags`);
  await expect(page.getByText(FLAG_NAME)).toBeVisible();
});

test('toggle flag state on detail page', async ({ page }) => {
  await page.goto(`/orgs/${ORG_SLUG}/projects/${PROJECT_SLUG}/flags`);

  // Click the flag name link to navigate to detail page
  await page.getByRole('link', { name: FLAG_NAME }).click();
  await expect(page).toHaveURL(/\/flags\//);

  // The toggle is a checkbox inside a .toggle label
  const toggleCheckbox = page.locator('.toggle input[type="checkbox"]').first();
  const initialState = await toggleCheckbox.isChecked();
  await toggleCheckbox.click();
  // State should have flipped
  await expect(toggleCheckbox).toBeChecked({ checked: !initialState });
});

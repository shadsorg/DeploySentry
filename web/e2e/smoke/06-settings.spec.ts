import { test, expect } from '@playwright/test';
import path from 'path';

const AUTH_FILE = path.join(__dirname, '..', '.auth', 'user.json');

test.use({ storageState: AUTH_FILE });

const ORG_SLUG = 'e2e-test-org';
const ENV_NAME = 'E2E Staging';
const WEBHOOK_URL = 'https://hooks.example.com/e2e-smoke';

test('create an environment → appears in the environments list', async ({ page }) => {
  await page.goto(`/orgs/${ORG_SLUG}/settings`);

  // Environments tab should be active by default for org settings
  await expect(page.getByText(/add environment/i)).toBeVisible();

  // Fill in the environment name
  await page.getByPlaceholder(/e\.g\. QA/i).fill(ENV_NAME);

  // Click Add Environment button
  await page.getByRole('button', { name: /add environment/i }).click();

  // The new environment should appear in the environments table
  await expect(page.getByRole('cell', { name: ENV_NAME })).toBeVisible();
});

test('switch to webhooks tab and add a webhook → appears in list', async ({ page }) => {
  await page.goto(`/orgs/${ORG_SLUG}/settings`);

  // Switch to the Webhooks tab
  await page.getByRole('button', { name: /webhooks/i }).click();

  // Open the add webhook form
  await page.getByRole('button', { name: /add webhook/i }).click();

  // Fill in the URL
  await page.getByPlaceholder(/https:\/\/hooks\.example\.com\/.*/i).fill(WEBHOOK_URL);

  // Select at least one event
  await page.getByRole('checkbox', { name: 'flag.changed' }).check();

  // Save
  await page.getByRole('button', { name: /create webhook/i }).click();

  // The webhook URL should now appear in the table
  await expect(page.getByText(WEBHOOK_URL)).toBeVisible();
});

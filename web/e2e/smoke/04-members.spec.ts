import { test, expect } from '@playwright/test';
import path from 'path';

const AUTH_FILE = path.join(__dirname, '..', '.auth', 'user.json');

test.use({ storageState: AUTH_FILE });

const ORG_SLUG = 'e2e-test-org';
const MEMBER_EMAIL = `e2e-member-${Date.now()}@test.deploysentry.io`;

test('navigate to members page', async ({ page }) => {
  await page.goto(`/orgs/${ORG_SLUG}/members`);
  await expect(page.getByRole('heading', { name: /members/i })).toBeVisible();
});

test('add member by email → member appears in list', async ({ page }) => {
  await page.goto(`/orgs/${ORG_SLUG}/members`);

  // Fill email input and select role, then click Add
  await page.getByPlaceholder(/email address/i).fill(MEMBER_EMAIL);
  await page.getByRole('button', { name: /^add$/i }).click();

  // Member email should now appear somewhere on the page
  await expect(page.getByText(MEMBER_EMAIL)).toBeVisible();
});

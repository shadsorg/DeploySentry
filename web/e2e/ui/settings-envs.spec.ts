import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Settings — Environments Tab', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/orgs/test-org/settings');
    // Environments tab is the default for org-level settings
  });

  test('renders environment list', async ({ page }) => {
    // Check that the environment names appear in table rows
    const table = page.locator('table');
    await expect(table.getByText('production').first()).toBeVisible();
    await expect(table.getByText('staging').first()).toBeVisible();
    await expect(table.getByText('development').first()).toBeVisible();
  });

  test('add new environment calls POST API', async ({ page }) => {
    let postCalled = false;
    await page.route(/\/api\/v1\/orgs\/test-org\/environments(\?.*)?$/, async (route) => {
      if (route.request().method() === 'POST') {
        postCalled = true;
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'env-new', name: 'qa', slug: 'qa' }),
        });
        return;
      }
      await route.fallback();
    });

    const nameInput = page.getByPlaceholder('e.g. QA');
    await nameInput.fill('qa');
    await page.getByRole('button', { name: /add environment/i }).click();
    expect(postCalled).toBe(true);
  });

  test('delete confirmation is shown when deleting an environment', async ({ page }) => {
    const deleteBtn = page.getByRole('button', { name: /delete/i }).first();
    await deleteBtn.click();
    await expect(page.getByRole('button', { name: /confirm/i })).toBeVisible();
  });
});

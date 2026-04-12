import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Flag List Page', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/orgs/test-org/projects/test-project/flags');
  });

  test('renders flag table with fixture data', async ({ page }) => {
    await expect(page.getByText('dark-mode')).toBeVisible();
  });

  test('search filters flags', async ({ page }) => {
    const searchInput = page.getByPlaceholder(/search/i);
    await searchInput.fill('checkout');
    await expect(page.getByText('checkout-variant')).toBeVisible();
    await expect(page.getByText('dark-mode')).toBeHidden();
  });

  test('shows empty state when no flags', async ({ page }) => {
    await mockAuthenticatedPage(page, {
      '**/api/v1/flags': { body: { flags: [] } },
    });
    await page.goto('/orgs/test-org/projects/test-project/flags');
    await expect(page.getByText(/no flags/i)).toBeVisible();
  });

  test('create flag link or button exists', async ({ page }) => {
    const createBtn = page.getByRole('link', { name: /create/i }).or(
      page.getByRole('button', { name: /create/i })
    );
    await expect(createBtn.first()).toBeVisible();
  });
});

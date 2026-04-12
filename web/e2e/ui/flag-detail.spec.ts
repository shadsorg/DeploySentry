import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Flag Detail Page', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/orgs/test-org/projects/test-project/flags/flag-1');
  });

  test('renders flag metadata', async ({ page }) => {
    await expect(page.locator('.detail-header-title')).toHaveText('Dark Mode');
    await expect(page.locator('.detail-header-subtitle')).toHaveText('dark-mode');
    await expect(page.locator(`.badge`).filter({ hasText: 'feature' })).toBeVisible();
  });

  test('toggle changes enabled state', async ({ page }) => {
    // Initially enabled
    await expect(page.locator('label.toggle')).toContainText('Enabled');
    // Click the toggle label
    await page.locator('label.toggle').click();
    // Should now show Disabled
    await expect(page.locator('label.toggle')).toContainText('Disabled');
  });

  test('targeting rules tab shows rules', async ({ page }) => {
    const rulesTab = page.locator('button.detail-tab', { hasText: /targeting rules/i });
    if (await rulesTab.isVisible()) {
      await rulesTab.click();
    }
    await expect(page.getByText(/percentage|user_target/i).first()).toBeVisible();
  });
});

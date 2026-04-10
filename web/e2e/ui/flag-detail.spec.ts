import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Flag Detail Page', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/orgs/test-org/projects/test-project/flags/flag-1');
  });

  test('renders flag metadata', async ({ page }) => {
    await expect(page.getByText('Dark Mode')).toBeVisible();
    await expect(page.getByText('dark-mode')).toBeVisible();
    await expect(page.getByText(/feature/i)).toBeVisible();
  });

  test('toggle calls toggle API', async ({ page }) => {
    let toggleCalled = false;
    await page.route('**/api/v1/flags/flag-1/toggle', async (route) => {
      toggleCalled = true;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: 'flag-1', enabled: false }),
      });
    });

    const toggle = page.getByRole('switch').or(page.getByRole('checkbox', { name: /toggle|enabled/i })).first();
    await toggle.click();
    expect(toggleCalled).toBe(true);
  });

  test('targeting rules tab shows rules', async ({ page }) => {
    const rulesTab = page.getByRole('tab', { name: /rules|targeting/i });
    if (await rulesTab.isVisible()) {
      await rulesTab.click();
    }
    await expect(page.getByText(/percentage|user_target|rule/i).first()).toBeVisible();
  });
});

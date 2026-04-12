import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Settings — Webhooks Tab', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/orgs/test-org/settings');
    const webhooksTab = page.locator('button.tab', { hasText: /webhooks/i });
    await webhooksTab.click();
  });

  test('renders webhook list', async ({ page }) => {
    await expect(page.getByText('hooks.slack.com')).toBeVisible();
    await expect(page.getByText('pagerduty.com')).toBeVisible();
  });

  test('add webhook form expands on button click', async ({ page }) => {
    const addBtn = page.getByRole('button', { name: /add webhook/i });
    await addBtn.click();
    await expect(page.getByPlaceholder(/hooks\.example\.com/i)).toBeVisible();
  });

  test('save new webhook calls POST API', async ({ page }) => {
    let postCalled = false;
    await page.route(/\/api\/v1\/webhooks(\?.*)?$/, async (route) => {
      if (route.request().method() === 'POST') {
        postCalled = true;
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'new-wh', url: 'https://example.com/hook', events: [], is_active: true }),
        });
        return;
      }
      await route.fallback();
    });

    const addBtn = page.getByRole('button', { name: /add webhook/i });
    await addBtn.click();
    const urlInput = page.getByPlaceholder(/hooks\.example\.com/i);
    await urlInput.fill('https://example.com/hook');
    await page.getByRole('button', { name: /create webhook/i }).click();
    expect(postCalled).toBe(true);
  });

  test('shows empty state when no webhooks', async ({ page }) => {
    await mockAuthenticatedPage(page, {
      '**/api/v1/webhooks': { body: { webhooks: [] } },
    });
    await page.goto('/orgs/test-org/settings');
    const webhooksTab = page.locator('button.tab', { hasText: /webhooks/i });
    await webhooksTab.click();
    await expect(page.getByText(/no webhooks/i)).toBeVisible();
  });
});

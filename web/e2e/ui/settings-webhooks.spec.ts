import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Settings — Webhooks Tab', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/orgs/test-org/settings');
    const webhooksTab = page.getByRole('tab', { name: /webhooks/i });
    if (await webhooksTab.isVisible()) {
      await webhooksTab.click();
    }
  });

  test('renders webhook list', async ({ page }) => {
    await expect(page.getByText('Slack Notifier')).toBeVisible();
    await expect(page.getByText('PagerDuty Alerts')).toBeVisible();
  });

  test('add webhook form expands on button click', async ({ page }) => {
    const addBtn = page.getByRole('button', { name: /add webhook|new webhook/i });
    await addBtn.click();
    await expect(page.getByPlaceholder(/url|endpoint/i).first()).toBeVisible();
  });

  test('save new webhook calls POST API', async ({ page }) => {
    let postCalled = false;
    await page.route('**/api/v1/webhooks', async (route) => {
      if (route.request().method() === 'POST') {
        postCalled = true;
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'new-wh', name: 'Test Hook', url: 'https://example.com/hook', events: [], active: true }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ webhooks: [] }),
      });
    });

    const addBtn = page.getByRole('button', { name: /add webhook|new webhook/i });
    await addBtn.click();
    const urlInput = page.getByPlaceholder(/url|endpoint/i).first();
    await urlInput.fill('https://example.com/hook');
    const nameInput = page.getByPlaceholder(/name/i).first();
    if (await nameInput.isVisible()) {
      await nameInput.fill('Test Hook');
    }
    await page.getByRole('button', { name: /save|create|add/i }).last().click();
    expect(postCalled).toBe(true);
  });

  test('shows empty state when no webhooks', async ({ page }) => {
    await mockAuthenticatedPage(page, {
      '**/api/v1/webhooks': { body: { webhooks: [] } },
    });
    await page.goto('/orgs/test-org/settings');
    const webhooksTab = page.getByRole('tab', { name: /webhooks/i });
    if (await webhooksTab.isVisible()) {
      await webhooksTab.click();
    }
    await expect(page.getByText(/no webhooks|add your first/i)).toBeVisible();
  });
});

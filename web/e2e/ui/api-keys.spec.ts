import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('API Keys Page', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/orgs/test-org/api-keys');
  });

  test('renders API key list', async ({ page }) => {
    await expect(page.getByText('CI/CD Pipeline Key')).toBeVisible();
    await expect(page.getByText('Read-Only SDK Key')).toBeVisible();
  });

  test('create key and reveal token', async ({ page }) => {
    await page.route(/\/api\/v1\/api-keys(\?.*)?$/, async (route) => {
      if (route.request().method() === 'POST') {
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            id: '550e8400-e29b-41d4-a716-446655440103',
            token: 'ds_live_newkey1234567890abcdef',
            name: 'New Test Key',
            scopes: ['flags:read', 'flags:write'],
          }),
        });
        return;
      }
      await route.fallback();
    });

    const createBtn = page.getByRole('button', { name: /create api key/i });
    await createBtn.click();
    const nameInput = page.getByPlaceholder(/production backend/i);
    await nameInput.fill('New Test Key');
    // Must select at least one scope for the form to submit
    await page.getByLabel('flags:read').check();
    await page.getByRole('button', { name: /create key/i }).click();

    // The token should be revealed after creation
    await expect(page.getByText(/ds_live_newkey/i)).toBeVisible();
  });
});

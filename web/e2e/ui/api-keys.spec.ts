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
    await page.route('**/api/v1/api-keys', async (route) => {
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
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          api_keys: [
            { id: 'key-1', name: 'CI/CD Pipeline Key', prefix: 'ds_live_a1b2c3' },
            { id: 'key-2', name: 'Read-Only SDK Key', prefix: 'ds_live_d4e5f6' },
          ],
        }),
      });
    });

    const createBtn = page.getByRole('button', { name: /create|new|generate/i });
    await createBtn.click();
    const nameInput = page.getByPlaceholder(/name|key name/i).first();
    if (await nameInput.isVisible()) {
      await nameInput.fill('New Test Key');
    }
    await page.getByRole('button', { name: /create|save|generate/i }).last().click();

    // The token should be revealed after creation
    await expect(page.getByText(/ds_live_newkey/i)).toBeVisible();
  });
});

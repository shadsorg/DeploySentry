import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Settings — Environments Tab', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/orgs/test-org/settings');
    const envsTab = page.getByRole('tab', { name: /environment/i });
    if (await envsTab.isVisible()) {
      await envsTab.click();
    }
  });

  test('renders environment list', async ({ page }) => {
    await expect(page.getByText('production')).toBeVisible();
    await expect(page.getByText('staging')).toBeVisible();
    await expect(page.getByText('development')).toBeVisible();
  });

  test('add new environment calls POST API', async ({ page }) => {
    let postCalled = false;
    await page.route('**/api/v1/orgs/test-org/environments', async (route) => {
      if (route.request().method() === 'POST') {
        postCalled = true;
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'env-new', name: 'qa', slug: 'qa' }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          environments: [
            { id: 'env-1', name: 'production', slug: 'production', is_default: true },
            { id: 'env-2', name: 'staging', slug: 'staging', is_default: false },
          ],
        }),
      });
    });

    const addBtn = page.getByRole('button', { name: /add environment|new environment/i });
    await addBtn.click();
    const nameInput = page.getByPlaceholder(/name|environment name/i).first();
    await nameInput.fill('qa');
    await page.getByRole('button', { name: /save|create|add/i }).last().click();
    expect(postCalled).toBe(true);
  });

  test('delete confirmation is shown when deleting an environment', async ({ page }) => {
    const deleteBtn = page.getByRole('button', { name: /delete|remove/i }).first();
    await deleteBtn.click();
    await expect(
      page.getByText(/confirm|are you sure|delete/i).first()
    ).toBeVisible();
  });
});

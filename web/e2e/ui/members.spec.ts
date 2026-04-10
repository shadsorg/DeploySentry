import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Members Page', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/orgs/test-org/members');
  });

  test('renders member list', async ({ page }) => {
    await expect(page.getByText('Alice Smith')).toBeVisible();
    await expect(page.getByText('Bob Johnson')).toBeVisible();
  });

  test('add member calls POST API', async ({ page }) => {
    let postCalled = false;
    await page.route('**/api/v1/orgs/test-org/members', async (route) => {
      if (route.request().method() === 'POST') {
        postCalled = true;
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            id: 'mem-new',
            email: 'carol@example.com',
            name: 'Carol White',
            role: 'member',
          }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          members: [
            { id: 'mem-1', name: 'Alice Smith', email: 'alice@example.com', role: 'owner' },
            { id: 'mem-2', name: 'Bob Johnson', email: 'bob@example.com', role: 'member' },
          ],
        }),
      });
    });

    const addBtn = page.getByRole('button', { name: /add member|invite/i });
    await addBtn.click();
    const emailInput = page.getByPlaceholder(/email/i).first();
    await emailInput.fill('carol@example.com');
    await page.getByRole('button', { name: /send invite|add|invite/i }).last().click();
    expect(postCalled).toBe(true);
  });
});

import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage, setupMockApi } from '../helpers/mock-api';

test.describe('Navigation', () => {
  test('deep link to nested project URL works', async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/orgs/test-org/projects/test-project/flags');
    await expect(page).toHaveURL(/\/orgs\/test-org\/projects\/test-project\/flags/);
    await expect(page.getByText('dark-mode')).toBeVisible();
  });

  test('deep link to app deployments URL works', async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/orgs/test-org/projects/test-project/apps/test-app/deployments');
    await expect(page).toHaveURL(/\/deployments/);
    await expect(page.getByText(/1\.4\.[12]|deployment/i).first()).toBeVisible();
  });

  test('unauthenticated user is redirected to /login', async ({ page }) => {
    await setupMockApi(page, {
      '**/api/v1/users/me': { status: 401, body: { error: 'Unauthorized' } },
    });
    // No token in localStorage — do not call mockAuthenticatedPage
    await page.goto('/orgs/test-org/projects');
    await page.waitForURL(/\/login/, { timeout: 5000 });
    await expect(page).toHaveURL(/\/login/);
  });

  test('authenticated user is redirected away from /login', async ({ page }) => {
    await mockAuthenticatedPage(page);
    await page.goto('/login');
    await page.waitForURL(/\/(orgs|$)/, { timeout: 5000 });
    await expect(page).not.toHaveURL(/\/login/);
  });
});

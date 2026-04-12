import { test, expect } from '@playwright/test';
import { setupMockApi } from '../helpers/mock-api';

test.describe('Login Page', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockApi(page);
    await page.goto('/login');
  });

  test('renders email, password fields and Sign in button', async ({ page }) => {
    await expect(page.locator('#email')).toBeVisible();
    await expect(page.locator('#password')).toBeVisible();
    await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible();
  });

  test('redirects on valid login', async ({ page }) => {
    await page.locator('#email').fill('alice@example.com');
    await page.locator('#password').fill('password123');
    await page.getByRole('button', { name: /sign in/i }).click();
    await page.waitForURL(/\/(orgs|$)/, { timeout: 5000 });
  });

  test('shows .auth-error on invalid credentials', async ({ page }) => {
    await setupMockApi(page, {
      '**/api/v1/auth/login': { status: 401, body: { error: 'Invalid credentials' } },
    });
    await page.goto('/login');
    await page.locator('#email').fill('wrong@example.com');
    await page.locator('#password').fill('badpassword');
    await page.getByRole('button', { name: /sign in/i }).click();
    await expect(page.locator('.auth-error')).toBeVisible();
  });

  test('"Create one" link navigates to /register', async ({ page }) => {
    await page.getByRole('link', { name: /create one/i }).click();
    await expect(page).toHaveURL(/\/register/);
  });
});

import { test, expect } from '@playwright/test';
import { setupMockApi } from '../helpers/mock-api';

test.describe('Register Page', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockApi(page);
    await page.goto('/register');
  });

  test('renders all registration fields', async ({ page }) => {
    await expect(page.locator('#name')).toBeVisible();
    await expect(page.locator('#email')).toBeVisible();
    await expect(page.locator('#password')).toBeVisible();
    await expect(page.locator('#confirmPassword')).toBeVisible();
    await expect(page.getByRole('button', { name: /create account/i })).toBeVisible();
  });

  test('redirects on successful registration', async ({ page }) => {
    await page.locator('#name').fill('Alice Smith');
    await page.locator('#email').fill('alice@example.com');
    await page.locator('#password').fill('password123');
    await page.locator('#confirmPassword').fill('password123');
    await page.getByRole('button', { name: /create account/i }).click();
    await page.waitForURL(/\/(orgs|$)/, { timeout: 5000 });
  });

  test('shows error when passwords do not match (client-side)', async ({ page }) => {
    await page.locator('#name').fill('Alice Smith');
    await page.locator('#email').fill('alice@example.com');
    await page.locator('#password').fill('password123');
    await page.locator('#confirmPassword').fill('different123');
    await page.getByRole('button', { name: /create account/i }).click();
    await expect(page.locator('.auth-error')).toBeVisible();
  });

  test('shows .auth-error on duplicate email (409)', async ({ page }) => {
    await setupMockApi(page, {
      '**/api/v1/auth/register': { status: 409, body: { error: 'Email already in use' } },
    });
    await page.goto('/register');
    await page.locator('#name').fill('Alice Smith');
    await page.locator('#email').fill('existing@example.com');
    await page.locator('#password').fill('password123');
    await page.locator('#confirmPassword').fill('password123');
    await page.getByRole('button', { name: /create account/i }).click();
    await expect(page.locator('.auth-error')).toBeVisible();
  });

  test('"Sign in" link navigates to /login', async ({ page }) => {
    await page.getByRole('link', { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/login/);
  });
});

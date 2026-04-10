import { test, expect } from '@playwright/test';
import path from 'path';
import { TEST_USER, loginAndSaveState } from '../helpers/auth';

const AUTH_FILE = path.join(__dirname, '..', '.auth', 'user.json');

test.describe.configure({ mode: 'serial' });

test('register new user redirects away from /register', async ({ page }) => {
  await page.goto('/register');
  await page.locator('#name').fill(TEST_USER.name);
  await page.locator('#email').fill(TEST_USER.email);
  await page.locator('#password').fill(TEST_USER.password);
  await page.locator('#confirmPassword').fill(TEST_USER.password);
  await page.getByRole('button', { name: /create account/i }).click();
  await expect(page).not.toHaveURL(/\/register/);
});

test('login with invalid credentials shows error', async ({ page }) => {
  await page.goto('/login');
  await page.locator('#email').fill(TEST_USER.email);
  await page.locator('#password').fill('wrongpassword123!');
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.locator('.auth-error')).toBeVisible();
});

test('login with valid credentials redirects away and saves state', async ({ page }) => {
  await loginAndSaveState(page, TEST_USER.email, TEST_USER.password, AUTH_FILE);
  await expect(page).not.toHaveURL(/\/login/);
});

test('session persists on reload', async ({ page }) => {
  await page.goto('/');
  await page.reload();
  await expect(page).not.toHaveURL(/\/login/);
});

test('logout clears session and redirects to /login', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /sign out/i }).click();
  await expect(page).toHaveURL(/\/login/);
});

test('protected route without auth redirects to /login', async ({ browser }) => {
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto('/');
  await expect(page).toHaveURL(/\/login/);
  await context.close();
});

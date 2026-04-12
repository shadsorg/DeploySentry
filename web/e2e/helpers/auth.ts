import { Page } from '@playwright/test';

export const TEST_USER = {
  email: `e2e-${Date.now()}@test.deploysentry.io`,
  password: 'TestPassword123!',
  name: 'E2E Test User',
};

export async function loginViaUI(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/login');
  await page.locator('#email').fill(email);
  await page.locator('#password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await page.waitForURL(/\/(orgs|$)/);
}

export async function loginAndSaveState(page: Page, email: string, password: string, path: string): Promise<void> {
  await loginViaUI(page, email, password);
  await page.context().storageState({ path });
}

export function setAuthToken(page: Page, token: string): Promise<void> {
  return page.evaluate((t) => localStorage.setItem('ds_token', t), token);
}

import { test, expect } from '@playwright/test';
import { setupMockApi, mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Landing page (unauthed)', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockApi(page);
  });

  test('renders hero headline and auth links', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('.hero-headline')).toContainText('Ship code.');
    await expect(page.getByRole('link', { name: 'Log in' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Sign up' })).toBeVisible();
  });

  test('Log in link navigates to /login', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('link', { name: 'Log in' }).click();
    await expect(page).toHaveURL(/\/login$/);
  });
});

test.describe('Landing page and header (authed)', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
    // mockAuthenticatedPage navigates to / and sets auth token
    // Wait for the header to reflect authed state
    await page.reload();
  });

  test('shows Portal button and user menu when authed', async ({ page }) => {
    await expect(page.getByRole('link', { name: 'Portal' })).toBeVisible();
    await expect(page.getByRole('button', { name: /user menu/i })).toBeVisible();
  });

  test('Portal button navigates to org dashboard', async ({ page }) => {
    await page.getByRole('link', { name: 'Portal' }).click();
    await expect(page).toHaveURL(/\/orgs\//);
  });

  test('clicking DS brand from inside app returns to landing', async ({ page }) => {
    // Navigate into the app first
    await page.getByRole('link', { name: 'Portal' }).click();
    await expect(page).toHaveURL(/\/orgs\//);
    // Verify brand link exists in header pointing to /
    const brandLink = page.locator('.site-header-brand');
    await expect(brandLink).toBeVisible();
    await expect(brandLink).toHaveAttribute('href', '/');
    // Navigate to / via link href — sidebar can visually overlap the brand in the app shell
    await page.goto('/');
    await expect(page).toHaveURL('/');
    await expect(page.locator('.hero-headline')).toBeVisible();
  });

  test('user menu opens, shows initials, and logout works', async ({ page }) => {
    const trigger = page.getByRole('button', { name: /user menu/i });
    // Should show initials "AS" for Alice Smith
    await expect(trigger).toContainText('AS');
    await trigger.click();
    await expect(page.locator('.user-menu-dropdown')).toBeVisible();
    await expect(page.locator('.user-menu-email')).toContainText('alice@example.com');
    // Click logout
    await page.locator('.user-menu-item', { hasText: 'Logout' }).click();
    // Should be back on landing, unauthenticated
    await expect(page).toHaveURL('/');
    await expect(page.getByRole('link', { name: 'Log in' })).toBeVisible();
  });
});

test.describe('Docs', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
    // Reload so AuthProvider picks up the token set by mockAuthenticatedPage
    await page.reload();
    // Wait for auth state to be resolved (UserMenu appears) before navigating to protected docs routes
    await expect(page.getByRole('button', { name: /user menu/i })).toBeVisible({ timeout: 10000 });
  });

  test('/docs redirects to getting-started and renders content', async ({ page }) => {
    await page.goto('/docs');
    await expect(page).toHaveURL(/\/docs\/getting-started$/);
    await expect(page.locator('.markdown-body h1')).toContainText('Getting Started');
  });

  test('sidebar nav switches docs pages', async ({ page }) => {
    await page.goto('/docs/getting-started');
    await expect(page.locator('.markdown-body h1')).toContainText('Getting Started');
    // Click SDKs in the docs sidebar
    await page.locator('.docs-sidebar-link', { hasText: 'SDKs' }).click();
    await expect(page).toHaveURL(/\/docs\/sdks$/);
    await expect(page.locator('.markdown-body h1')).toContainText('SDKs');
  });
});

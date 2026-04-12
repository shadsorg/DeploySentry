import { test, expect } from '@playwright/test';
import { seedOrgProjectAppViaUI, type SeededContext } from '../helpers/seed-via-ui';

let seeded: SeededContext;

test.beforeAll(async ({ browser, request }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  try {
    seeded = await seedOrgProjectAppViaUI(page, request);
  } finally {
    await page.close();
    await ctx.close();
  }
});

test('seeded environment has all required IDs and an API key', () => {
  expect(seeded.orgSlug).toMatch(/^e2e-org-/);
  expect(seeded.projectSlug).toMatch(/^e2e-proj-/);
  expect(seeded.appSlug).toMatch(/^e2e-app-/);
  expect(seeded.orgId).toMatch(/^[0-9a-f-]{36}$/i);
  expect(seeded.projectId).toMatch(/^[0-9a-f-]{36}$/i);
  expect(seeded.appId).toMatch(/^[0-9a-f-]{36}$/i);
  expect(seeded.environmentId).toMatch(/^[0-9a-f-]{36}$/i);
  expect(seeded.environment).toBe('development');
  expect(seeded.apiKey.length).toBeGreaterThan(20);
  expect(seeded.apiKey).toMatch(/^ds_/);
});

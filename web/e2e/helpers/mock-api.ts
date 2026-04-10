import { Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

type FixtureName =
  | 'auth' | 'orgs' | 'projects' | 'apps' | 'flags' | 'deployments'
  | 'releases' | 'members' | 'api-keys' | 'webhooks' | 'notifications'
  | 'settings' | 'environments';

function loadFixture(name: FixtureName): Record<string, unknown> {
  const filePath = path.join(__dirname, '..', 'fixtures', `${name}.json`);
  return JSON.parse(fs.readFileSync(filePath, 'utf-8'));
}

export interface MockOverrides {
  [route: string]: { status?: number; body: unknown };
}

export async function setupMockApi(page: Page, overrides?: MockOverrides): Promise<void> {
  const auth = loadFixture('auth');
  const orgs = loadFixture('orgs');
  const projects = loadFixture('projects');
  const apps = loadFixture('apps');
  const flags = loadFixture('flags');
  const deployments = loadFixture('deployments');
  const releases = loadFixture('releases');
  const membersData = loadFixture('members');
  const apiKeys = loadFixture('api-keys');
  const webhooks = loadFixture('webhooks');
  const notifications = loadFixture('notifications');
  const settingsData = loadFixture('settings');
  const environments = loadFixture('environments');

  const routes: Record<string, { method?: string; body: unknown }> = {
    '**/api/v1/auth/login': { method: 'POST', body: auth['login'] },
    '**/api/v1/auth/register': { method: 'POST', body: auth['register'] },
    '**/api/v1/users/me': { body: auth['me'] },
    '**/api/v1/orgs': { body: orgs['list'] },
    '**/api/v1/orgs/test-org': { body: orgs['detail'] },
    '**/api/v1/orgs/test-org/projects': { body: projects['list'] },
    '**/api/v1/orgs/test-org/projects/test-project': { body: projects['detail'] },
    '**/api/v1/orgs/test-org/projects/test-project/apps': { body: apps['list'] },
    '**/api/v1/orgs/test-org/projects/test-project/apps/test-app': { body: apps['detail'] },
    '**/api/v1/orgs/test-org/environments': { body: environments['list'] },
    '**/api/v1/orgs/test-org/members': { body: membersData['list'] },
    '**/api/v1/flags': { body: flags['list'] },
    '**/api/v1/flags/flag-1': { body: flags['detail'] },
    '**/api/v1/flags/flag-1/rules': { body: flags['rules'] },
    '**/api/v1/deployments': { body: deployments['list'] },
    '**/api/v1/deployments/dep-1': { body: deployments['detail'] },
    '**/api/v1/releases': { body: releases['list'] },
    '**/api/v1/releases/rel-1': { body: releases['detail'] },
    '**/api/v1/api-keys': { body: apiKeys['list'] },
    '**/api/v1/webhooks': { body: webhooks['list'] },
    '**/api/v1/notifications/preferences': { body: notifications['preferences'] },
    '**/api/v1/settings': { body: settingsData['list'] },
  };

  for (const [pattern, config] of Object.entries(routes)) {
    await page.route(pattern, async (route) => {
      const override = overrides?.[pattern];
      if (override) {
        await route.fulfill({
          status: override.status ?? 200,
          contentType: 'application/json',
          body: JSON.stringify(override.body),
        });
        return;
      }
      if (config.method && route.request().method() !== config.method) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ success: true }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(config.body),
      });
    });
  }

  // Default handler for unmatched API routes
  await page.route('**/api/v1/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({}),
    });
  });
}

export async function mockAuthenticatedPage(page: Page, overrides?: MockOverrides): Promise<void> {
  await setupMockApi(page, overrides);
  await page.evaluate(() => localStorage.setItem('ds_token', 'test-jwt-token'));
}

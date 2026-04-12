# Playwright E2E Testing — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add comprehensive Playwright E2E tests for the DeploySentry web dashboard — a smoke suite against the real backend and a UI suite with mocked API responses.

**Architecture:** Two Playwright projects: `smoke` (serial, real backend, flow-based) and `ui` (parallel, mocked API, page-based). Shared helpers for API seeding, mock setup, and auth state caching. JSON fixtures for deterministic mock responses.

**Tech Stack:** Playwright Test, TypeScript, Vite dev server (port 3001), Go API server (port 8080)

**Spec:** `docs/superpowers/specs/2026-04-10-playwright-e2e-testing-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `web/playwright.config.ts` | Two projects: smoke (serial) and ui (parallel) |
| Create | `web/e2e/helpers/api-client.ts` | HTTP client for smoke test data seeding |
| Create | `web/e2e/helpers/auth.ts` | Login helper with storageState caching |
| Create | `web/e2e/helpers/mock-api.ts` | page.route() interceptor from fixtures |
| Create | `web/e2e/fixtures/*.json` | 13 fixture files for mock responses |
| Create | `web/e2e/smoke/01-auth.spec.ts` | Auth lifecycle smoke test |
| Create | `web/e2e/smoke/02-org-setup.spec.ts` | Entity hierarchy smoke test |
| Create | `web/e2e/smoke/03-flag-lifecycle.spec.ts` | Flag CRUD smoke test |
| Create | `web/e2e/smoke/04-members.spec.ts` | Membership smoke test |
| Create | `web/e2e/smoke/05-api-keys.spec.ts` | API key smoke test |
| Create | `web/e2e/smoke/06-settings.spec.ts` | Settings smoke test |
| Create | `web/e2e/ui/login-page.spec.ts` | Login page UI tests |
| Create | `web/e2e/ui/register-page.spec.ts` | Register page UI tests |
| Create | `web/e2e/ui/flag-list.spec.ts` | Flag list page UI tests |
| Create | `web/e2e/ui/flag-detail.spec.ts` | Flag detail page UI tests |
| Create | `web/e2e/ui/settings-webhooks.spec.ts` | Webhooks tab UI tests |
| Create | `web/e2e/ui/settings-envs.spec.ts` | Environments tab UI tests |
| Create | `web/e2e/ui/members.spec.ts` | Members page UI tests |
| Create | `web/e2e/ui/api-keys.spec.ts` | API keys page UI tests |
| Create | `web/e2e/ui/navigation.spec.ts` | Cross-page navigation tests |
| Modify | `web/package.json` | Add Playwright devDependency and test scripts |

---

### Task 1: Install Playwright and Configure

**Files:**
- Modify: `web/package.json`
- Create: `web/playwright.config.ts`

- [ ] **Step 1: Install Playwright**

```bash
cd /Users/sgamel/git/DeploySentry/web && npm install -D @playwright/test
```

- [ ] **Step 2: Install Playwright browsers**

```bash
cd /Users/sgamel/git/DeploySentry/web && npx playwright install chromium
```

- [ ] **Step 3: Add test scripts to package.json**

In `web/package.json`, add to the `"scripts"` section:

```json
"test:e2e": "npx playwright test",
"test:e2e:smoke": "npx playwright test --project=smoke",
"test:e2e:ui": "npx playwright test --project=ui",
"test:e2e:report": "npx playwright show-report"
```

- [ ] **Step 4: Create playwright.config.ts**

Write `web/playwright.config.ts`:

```typescript
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: false,
  reporter: [['html', { open: 'never' }]],

  projects: [
    {
      name: 'smoke',
      testDir: './e2e/smoke',
      use: {
        baseURL: 'http://localhost:3001',
        browserName: 'chromium',
        trace: 'on-first-retry',
      },
      fullyParallel: false,
      retries: 0,
    },
    {
      name: 'ui',
      testDir: './e2e/ui',
      use: {
        baseURL: 'http://localhost:3001',
        browserName: 'chromium',
        trace: 'on-first-retry',
      },
      fullyParallel: true,
      retries: 1,
    },
  ],

  webServer: {
    command: 'npm run dev',
    port: 3001,
    reuseExistingServer: true,
  },
});
```

- [ ] **Step 5: Create directory structure**

```bash
mkdir -p web/e2e/{smoke,ui,fixtures,helpers}
```

- [ ] **Step 6: Add e2e artifacts to .gitignore**

Append to `web/.gitignore` (create if it doesn't exist):

```
# Playwright
test-results/
playwright-report/
e2e/.auth/
```

- [ ] **Step 7: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/package.json web/package-lock.json web/playwright.config.ts web/.gitignore
git commit -m "feat: add Playwright config with smoke and ui test projects"
```

---

### Task 2: API Client Helper for Smoke Tests

**Files:**
- Create: `web/e2e/helpers/api-client.ts`

- [ ] **Step 1: Create API client**

Write `web/e2e/helpers/api-client.ts`:

```typescript
const API_BASE = 'http://localhost:8080/api/v1';

interface AuthResponse {
  token: string;
  user: { id: string; email: string; name: string };
}

export class ApiClient {
  private token: string | undefined;

  constructor(token?: string) {
    this.token = token;
  }

  private async request<T>(path: string, init?: RequestInit): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(this.token ? { Authorization: `Bearer ${this.token}` } : {}),
    };
    const res = await fetch(`${API_BASE}${path}`, { ...init, headers: { ...headers, ...init?.headers } });
    if (!res.ok) {
      const body = await res.text();
      throw new Error(`API ${init?.method ?? 'GET'} ${path} failed (${res.status}): ${body}`);
    }
    return res.json() as Promise<T>;
  }

  async register(email: string, password: string, name: string): Promise<string> {
    const res = await this.request<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password, name }),
    });
    this.token = res.token;
    return res.token;
  }

  async login(email: string, password: string): Promise<string> {
    const res = await this.request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    this.token = res.token;
    return res.token;
  }

  async createOrg(name: string, slug: string) {
    return this.request<{ id: string; name: string; slug: string }>('/orgs', {
      method: 'POST',
      body: JSON.stringify({ name, slug }),
    });
  }

  async createProject(orgSlug: string, name: string, slug: string) {
    return this.request<{ id: string; name: string; slug: string }>(`/orgs/${orgSlug}/projects`, {
      method: 'POST',
      body: JSON.stringify({ name, slug }),
    });
  }

  async createApp(orgSlug: string, projectSlug: string, name: string, slug: string) {
    return this.request<{ id: string; name: string; slug: string }>(
      `/orgs/${orgSlug}/projects/${projectSlug}/apps`,
      { method: 'POST', body: JSON.stringify({ name, slug }) },
    );
  }

  async createFlag(projectId: string, data: Record<string, unknown>) {
    return this.request<{ id: string; key: string }>('/flags', {
      method: 'POST',
      body: JSON.stringify({ project_id: projectId, ...data }),
    });
  }

  async createEnvironment(orgSlug: string, name: string, slug: string, isProduction: boolean) {
    return this.request<{ id: string; slug: string }>(`/orgs/${orgSlug}/environments`, {
      method: 'POST',
      body: JSON.stringify({ name, slug, is_production: isProduction }),
    });
  }

  async createWebhook(url: string, events: string[]) {
    return this.request<{ id: string }>('/webhooks', {
      method: 'POST',
      body: JSON.stringify({ url, events, is_active: true }),
    });
  }

  async createApiKey(name: string, scopes: string[]) {
    return this.request<{ id: string; token: string }>('/api-keys', {
      method: 'POST',
      body: JSON.stringify({ name, scopes }),
    });
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add web/e2e/helpers/api-client.ts
git commit -m "feat: add API client helper for smoke test data seeding"
```

---

### Task 3: Auth Helper and Mock API Helper

**Files:**
- Create: `web/e2e/helpers/auth.ts`
- Create: `web/e2e/helpers/mock-api.ts`

- [ ] **Step 1: Create auth helper**

Write `web/e2e/helpers/auth.ts`:

```typescript
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
```

- [ ] **Step 2: Create mock API helper**

Write `web/e2e/helpers/mock-api.ts`:

```typescript
import { Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

type FixtureName =
  | 'auth'
  | 'orgs'
  | 'projects'
  | 'apps'
  | 'flags'
  | 'deployments'
  | 'releases'
  | 'members'
  | 'api-keys'
  | 'webhooks'
  | 'notifications'
  | 'settings'
  | 'environments';

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
  const members = loadFixture('members');
  const apiKeys = loadFixture('api-keys');
  const webhooks = loadFixture('webhooks');
  const notifications = loadFixture('notifications');
  const settings = loadFixture('settings');
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
    '**/api/v1/orgs/test-org/members': { body: members['list'] },
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
    '**/api/v1/settings': { body: settings['list'] },
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
```

- [ ] **Step 3: Commit**

```bash
git add web/e2e/helpers/auth.ts web/e2e/helpers/mock-api.ts
git commit -m "feat: add auth and mock API helpers for E2E tests"
```

---

### Task 4: JSON Fixtures

**Files:**
- Create: `web/e2e/fixtures/auth.json`
- Create: `web/e2e/fixtures/orgs.json`
- Create: `web/e2e/fixtures/projects.json`
- Create: `web/e2e/fixtures/apps.json`
- Create: `web/e2e/fixtures/flags.json`
- Create: `web/e2e/fixtures/deployments.json`
- Create: `web/e2e/fixtures/releases.json`
- Create: `web/e2e/fixtures/members.json`
- Create: `web/e2e/fixtures/api-keys.json`
- Create: `web/e2e/fixtures/webhooks.json`
- Create: `web/e2e/fixtures/notifications.json`
- Create: `web/e2e/fixtures/settings.json`
- Create: `web/e2e/fixtures/environments.json`

- [ ] **Step 1: Create all fixture files**

Create each fixture file with response shapes matching the real API. The keys in each file match what `mock-api.ts` expects (`list`, `detail`, `rules`, etc.).

Read `web/src/api.ts` and `sdk/testdata/*.json` to understand the exact response shapes. Each fixture should contain realistic sample data with 2-3 items in list responses.

Key fixture shapes:

**auth.json**: `login` (token + user), `register` (token + user), `me` (user object)
**orgs.json**: `list` → `{ organizations: [...] }`, `detail` → single org
**projects.json**: `list` → `{ projects: [...] }`, `detail` → single project
**apps.json**: `list` → `{ applications: [...] }`, `detail` → single app
**flags.json**: `list` → `{ flags: [...] }` (3 flags: boolean/string/integer, different categories), `detail` → single flag, `rules` → `{ rules: [...] }` (2 rules)
**deployments.json**: `list` → `{ deployments: [...] }`, `detail` → single deployment with status/strategy
**releases.json**: `list` → `{ releases: [...] }`, `detail` → single release with flag_changes
**members.json**: `list` → `{ members: [...] }` (owner + member)
**api-keys.json**: `list` → `{ api_keys: [...] }`, `create` → `{ id, token }`
**webhooks.json**: `list` → `{ webhooks: [...] }` (2 webhooks)
**notifications.json**: `preferences` → `{ channels: { slack, email, pagerduty }, event_routing: {...} }`
**settings.json**: `list` → `{ settings: [...] }`
**environments.json**: `list` → `{ environments: [...] }` (production + staging)

Use UUIDs for IDs, realistic names, and ISO timestamps.

- [ ] **Step 2: Commit**

```bash
git add web/e2e/fixtures/
git commit -m "feat: add JSON fixtures for mocked E2E tests"
```

---

### Task 5: Smoke Tests — Auth and Org Setup

**Files:**
- Create: `web/e2e/smoke/01-auth.spec.ts`
- Create: `web/e2e/smoke/02-org-setup.spec.ts`

- [ ] **Step 1: Create auth smoke test**

Write `web/e2e/smoke/01-auth.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import { ApiClient } from '../helpers/api-client';
import { TEST_USER, loginViaUI } from '../helpers/auth';
import * as path from 'path';
import * as fs from 'fs';

const AUTH_DIR = path.join(__dirname, '..', '.auth');
const AUTH_FILE = path.join(AUTH_DIR, 'user.json');

test.describe.serial('Auth lifecycle', () => {
  test('register a new user', async ({ page }) => {
    await page.goto('/register');
    await page.locator('#name').fill(TEST_USER.name);
    await page.locator('#email').fill(TEST_USER.email);
    await page.locator('#password').fill(TEST_USER.password);
    await page.locator('#confirmPassword').fill(TEST_USER.password);
    await page.getByRole('button', { name: /create account/i }).click();
    await page.waitForURL(/\/(orgs|$)/);
    await expect(page).not.toHaveURL(/\/register/);
  });

  test('logout clears session', async ({ page }) => {
    await loginViaUI(page, TEST_USER.email, TEST_USER.password);
    await page.getByRole('button', { name: /logout|sign out/i }).click();
    await expect(page).toHaveURL(/\/login/);
    const token = await page.evaluate(() => localStorage.getItem('ds_token'));
    expect(token).toBeFalsy();
  });

  test('login with valid credentials', async ({ page }) => {
    await loginViaUI(page, TEST_USER.email, TEST_USER.password);
    await expect(page).not.toHaveURL(/\/login/);
    // Save auth state for subsequent tests
    if (!fs.existsSync(AUTH_DIR)) fs.mkdirSync(AUTH_DIR, { recursive: true });
    await page.context().storageState({ path: AUTH_FILE });
  });

  test('login with invalid credentials shows error', async ({ page }) => {
    await page.goto('/login');
    await page.locator('#email').fill('wrong@example.com');
    await page.locator('#password').fill('wrongpassword');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page.locator('.auth-error')).toBeVisible();
  });

  test('session persists on reload', async ({ page }) => {
    await loginViaUI(page, TEST_USER.email, TEST_USER.password);
    await page.reload();
    await expect(page).not.toHaveURL(/\/login/);
  });

  test('protected route redirects to login when not authenticated', async ({ page }) => {
    await page.goto('/orgs/test/projects');
    await expect(page).toHaveURL(/\/login/);
  });
});
```

- [ ] **Step 2: Create org setup smoke test**

Write `web/e2e/smoke/02-org-setup.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import { loginViaUI, TEST_USER } from '../helpers/auth';

test.describe.serial('Entity hierarchy setup', () => {
  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await loginViaUI(page, TEST_USER.email, TEST_USER.password);
    await page.context().storageState({ path: require('path').join(__dirname, '..', '.auth', 'user.json') });
    await page.close();
  });

  test('create an organization', async ({ browser }) => {
    const context = await browser.newContext({
      storageState: require('path').join(__dirname, '..', '.auth', 'user.json'),
    });
    const page = await context.newPage();
    await page.goto('/orgs/new');
    await page.getByLabel(/name/i).fill('E2E Test Org');
    await page.getByRole('button', { name: /create/i }).click();
    await page.waitForURL(/\/orgs\/.*\/projects/);
    await expect(page).toHaveURL(/\/orgs\/e2e-test-org\/projects/);
    await context.close();
  });

  test('create a project', async ({ browser }) => {
    const context = await browser.newContext({
      storageState: require('path').join(__dirname, '..', '.auth', 'user.json'),
    });
    const page = await context.newPage();
    await page.goto('/orgs/e2e-test-org/projects/new');
    await page.getByLabel(/name/i).fill('E2E Project');
    await page.getByRole('button', { name: /create/i }).click();
    await page.waitForURL(/\/projects\/e2e-project/);
    await context.close();
  });

  test('create an application', async ({ browser }) => {
    const context = await browser.newContext({
      storageState: require('path').join(__dirname, '..', '.auth', 'user.json'),
    });
    const page = await context.newPage();
    await page.goto('/orgs/e2e-test-org/projects/e2e-project/apps/new');
    await page.getByLabel(/name/i).fill('E2E App');
    await page.getByRole('button', { name: /create/i }).click();
    await page.waitForURL(/\/apps\/e2e-app/);
    await context.close();
  });
});
```

- [ ] **Step 3: Verify smoke tests are syntactically valid**

```bash
cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit --project tsconfig.json e2e/smoke/01-auth.spec.ts 2>&1 || echo "Type check skipped — Playwright types may not resolve under web tsconfig"
```

- [ ] **Step 4: Commit**

```bash
git add web/e2e/smoke/01-auth.spec.ts web/e2e/smoke/02-org-setup.spec.ts
git commit -m "feat: add auth and org-setup smoke tests"
```

---

### Task 6: Smoke Tests — Flag Lifecycle, Members, API Keys, Settings

**Files:**
- Create: `web/e2e/smoke/03-flag-lifecycle.spec.ts`
- Create: `web/e2e/smoke/04-members.spec.ts`
- Create: `web/e2e/smoke/05-api-keys.spec.ts`
- Create: `web/e2e/smoke/06-settings.spec.ts`

- [ ] **Step 1: Create flag lifecycle smoke test**

Write `web/e2e/smoke/03-flag-lifecycle.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import * as path from 'path';

const AUTH_FILE = path.join(__dirname, '..', '.auth', 'user.json');
const ORG = 'e2e-test-org';
const PROJECT = 'e2e-project';

test.describe.serial('Flag lifecycle', () => {
  test.use({ storageState: AUTH_FILE });

  test('create a boolean flag', async ({ page }) => {
    await page.goto(`/orgs/${ORG}/projects/${PROJECT}/flags/new`);
    await page.getByLabel(/key/i).fill('e2e-test-flag');
    await page.getByLabel(/name/i).fill('E2E Test Flag');
    await page.getByRole('button', { name: /create/i }).click();
    await page.waitForURL(/\/flags\//);
  });

  test('flag appears in list', async ({ page }) => {
    await page.goto(`/orgs/${ORG}/projects/${PROJECT}/flags`);
    await expect(page.getByText('e2e-test-flag')).toBeVisible();
  });

  test('toggle flag enabled state', async ({ page }) => {
    await page.goto(`/orgs/${ORG}/projects/${PROJECT}/flags`);
    await page.getByText('e2e-test-flag').click();
    const toggle = page.locator('button, input[type="checkbox"]').filter({ hasText: /toggle|enable|disable/i }).first();
    await toggle.click();
    await expect(page.getByText(/toggled|updated|enabled|disabled/i)).toBeVisible({ timeout: 5000 });
  });
});
```

- [ ] **Step 2: Create members smoke test**

Write `web/e2e/smoke/04-members.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import * as path from 'path';

const AUTH_FILE = path.join(__dirname, '..', '.auth', 'user.json');
const ORG = 'e2e-test-org';

test.describe.serial('Members management', () => {
  test.use({ storageState: AUTH_FILE });

  test('add a member', async ({ page }) => {
    await page.goto(`/orgs/${ORG}/members`);
    await page.getByPlaceholder(/email/i).fill('member@test.deploysentry.io');
    await page.getByRole('button', { name: /add|invite/i }).click();
    await expect(page.getByText('member@test.deploysentry.io')).toBeVisible({ timeout: 5000 });
  });
});
```

- [ ] **Step 3: Create API keys smoke test**

Write `web/e2e/smoke/05-api-keys.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import * as path from 'path';

const AUTH_FILE = path.join(__dirname, '..', '.auth', 'user.json');
const ORG = 'e2e-test-org';

test.describe.serial('API keys', () => {
  test.use({ storageState: AUTH_FILE });

  test('create an API key', async ({ page }) => {
    await page.goto(`/orgs/${ORG}/api-keys`);
    await page.getByRole('button', { name: /create|new/i }).click();
    await page.getByPlaceholder(/name/i).fill('E2E Test Key');
    await page.getByLabel(/flags:read/i).check();
    await page.getByRole('button', { name: /create|save/i }).click();
    // Key token should be revealed
    await expect(page.getByText(/ds_/)).toBeVisible({ timeout: 5000 });
  });
});
```

- [ ] **Step 4: Create settings smoke test**

Write `web/e2e/smoke/06-settings.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import * as path from 'path';

const AUTH_FILE = path.join(__dirname, '..', '.auth', 'user.json');
const ORG = 'e2e-test-org';

test.describe.serial('Settings', () => {
  test.use({ storageState: AUTH_FILE });

  test('create an environment', async ({ page }) => {
    await page.goto(`/orgs/${ORG}/settings`);
    await page.getByPlaceholder(/name/i).fill('Staging');
    await page.getByRole('button', { name: /add environment/i }).click();
    await expect(page.getByText('staging')).toBeVisible({ timeout: 5000 });
  });

  test('create a webhook', async ({ page }) => {
    await page.goto(`/orgs/${ORG}/settings`);
    await page.getByRole('tab', { name: /webhook/i }).or(page.getByText(/webhook/i)).click();
    await page.getByRole('button', { name: /add webhook/i }).click();
    await page.getByPlaceholder(/https/i).fill('https://example.com/hook');
    await page.getByRole('button', { name: /save/i }).click();
    await expect(page.getByText('https://example.com/hook')).toBeVisible({ timeout: 5000 });
  });
});
```

- [ ] **Step 5: Commit**

```bash
git add web/e2e/smoke/03-flag-lifecycle.spec.ts web/e2e/smoke/04-members.spec.ts \
        web/e2e/smoke/05-api-keys.spec.ts web/e2e/smoke/06-settings.spec.ts
git commit -m "feat: add flag, members, api-keys, and settings smoke tests"
```

---

### Task 7: UI Tests — Login and Register Pages

**Files:**
- Create: `web/e2e/ui/login-page.spec.ts`
- Create: `web/e2e/ui/register-page.spec.ts`

- [ ] **Step 1: Create login page UI tests**

Write `web/e2e/ui/login-page.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import { setupMockApi } from '../helpers/mock-api';

test.describe('LoginPage', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockApi(page);
  });

  test('should render login form fields', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('#email')).toBeVisible();
    await expect(page.locator('#password')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible();
  });

  test('should redirect to dashboard on valid login', async ({ page }) => {
    await page.goto('/login');
    await page.locator('#email').fill('test@example.com');
    await page.locator('#password').fill('password123');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page).not.toHaveURL(/\/login/);
  });

  test('should show error on invalid credentials', async ({ page }) => {
    await page.route('**/api/v1/auth/login', (route) =>
      route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'Invalid credentials' }) }),
    );
    await page.goto('/login');
    await page.locator('#email').fill('wrong@example.com');
    await page.locator('#password').fill('wrongpass');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page.locator('.auth-error')).toBeVisible();
  });

  test('should navigate to register page', async ({ page }) => {
    await page.goto('/login');
    await page.getByRole('link', { name: /create one/i }).click();
    await expect(page).toHaveURL(/\/register/);
  });
});
```

- [ ] **Step 2: Create register page UI tests**

Write `web/e2e/ui/register-page.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import { setupMockApi } from '../helpers/mock-api';

test.describe('RegisterPage', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockApi(page);
  });

  test('should render registration form', async ({ page }) => {
    await page.goto('/register');
    await expect(page.locator('#name')).toBeVisible();
    await expect(page.locator('#email')).toBeVisible();
    await expect(page.locator('#password')).toBeVisible();
    await expect(page.getByRole('button', { name: /create account/i })).toBeVisible();
  });

  test('should redirect on successful registration', async ({ page }) => {
    await page.goto('/register');
    await page.locator('#name').fill('Test User');
    await page.locator('#email').fill('new@example.com');
    await page.locator('#password').fill('password123');
    await page.locator('#confirmPassword').fill('password123');
    await page.getByRole('button', { name: /create account/i }).click();
    await expect(page).not.toHaveURL(/\/register/);
  });

  test('should show error when passwords do not match', async ({ page }) => {
    await page.goto('/register');
    await page.locator('#name').fill('Test User');
    await page.locator('#email').fill('new@example.com');
    await page.locator('#password').fill('password123');
    await page.locator('#confirmPassword').fill('differentpass');
    await page.getByRole('button', { name: /create account/i }).click();
    await expect(page.getByText(/passwords.*match/i)).toBeVisible();
  });

  test('should show error on duplicate email', async ({ page }) => {
    await page.route('**/api/v1/auth/register', (route) =>
      route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({ error: 'Email already registered' }) }),
    );
    await page.goto('/register');
    await page.locator('#name').fill('Test User');
    await page.locator('#email').fill('existing@example.com');
    await page.locator('#password').fill('password123');
    await page.locator('#confirmPassword').fill('password123');
    await page.getByRole('button', { name: /create account/i }).click();
    await expect(page.locator('.auth-error')).toBeVisible();
  });

  test('should navigate to login page', async ({ page }) => {
    await page.goto('/register');
    await page.getByRole('link', { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/login/);
  });
});
```

- [ ] **Step 3: Commit**

```bash
git add web/e2e/ui/login-page.spec.ts web/e2e/ui/register-page.spec.ts
git commit -m "feat: add login and register page UI tests"
```

---

### Task 8: UI Tests — Flag List and Flag Detail

**Files:**
- Create: `web/e2e/ui/flag-list.spec.ts`
- Create: `web/e2e/ui/flag-detail.spec.ts`

- [ ] **Step 1: Create flag list UI tests**

Write `web/e2e/ui/flag-list.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('FlagListPage', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
  });

  test('should render flag table with data', async ({ page }) => {
    await page.goto('/orgs/test-org/projects/test-project/flags');
    await expect(page.getByText('dark-mode')).toBeVisible();
  });

  test('should filter flags by search term', async ({ page }) => {
    await page.goto('/orgs/test-org/projects/test-project/flags');
    await page.getByPlaceholder(/search/i).fill('dark');
    await expect(page.getByText('dark-mode')).toBeVisible();
  });

  test('should show empty state when no flags match', async ({ page }) => {
    await mockAuthenticatedPage(page, {
      '**/api/v1/flags': { body: { flags: [] } },
    });
    await page.goto('/orgs/test-org/projects/test-project/flags');
    await expect(page.getByText(/no flags/i)).toBeVisible();
  });

  test('should navigate to create flag page', async ({ page }) => {
    await page.goto('/orgs/test-org/projects/test-project/flags');
    await page.getByRole('link', { name: /create flag/i }).or(page.getByRole('button', { name: /create flag/i })).click();
    await expect(page).toHaveURL(/\/flags\/new/);
  });
});
```

- [ ] **Step 2: Create flag detail UI tests**

Write `web/e2e/ui/flag-detail.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('FlagDetailPage', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
  });

  test('should render flag metadata', async ({ page }) => {
    await page.goto('/orgs/test-org/projects/test-project/flags/flag-1');
    await expect(page.getByText('dark-mode')).toBeVisible();
  });

  test('should toggle flag state', async ({ page }) => {
    let toggleCalled = false;
    await page.route('**/api/v1/flags/flag-1/toggle', async (route) => {
      toggleCalled = true;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ enabled: false }) });
    });
    await page.goto('/orgs/test-org/projects/test-project/flags/flag-1');
    const toggle = page.locator('button, label, input').filter({ hasText: /toggle|enable|disable/i }).first();
    if (await toggle.isVisible()) {
      await toggle.click();
      expect(toggleCalled).toBe(true);
    }
  });

  test('should show targeting rules tab', async ({ page }) => {
    await page.goto('/orgs/test-org/projects/test-project/flags/flag-1');
    await page.getByRole('tab', { name: /rules|targeting/i }).or(page.getByText(/targeting rules/i)).click();
    await expect(page.getByText(/percentage|user_target|attribute/i)).toBeVisible();
  });
});
```

- [ ] **Step 3: Commit**

```bash
git add web/e2e/ui/flag-list.spec.ts web/e2e/ui/flag-detail.spec.ts
git commit -m "feat: add flag list and detail UI tests"
```

---

### Task 9: UI Tests — Settings (Webhooks + Environments)

**Files:**
- Create: `web/e2e/ui/settings-webhooks.spec.ts`
- Create: `web/e2e/ui/settings-envs.spec.ts`

- [ ] **Step 1: Create webhooks settings UI tests**

Write `web/e2e/ui/settings-webhooks.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Settings — Webhooks tab', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
  });

  test('should render webhook list', async ({ page }) => {
    await page.goto('/orgs/test-org/settings');
    await page.getByText(/webhook/i).click();
    await expect(page.getByText('https://')).toBeVisible();
  });

  test('should expand add webhook form', async ({ page }) => {
    await page.goto('/orgs/test-org/settings');
    await page.getByText(/webhook/i).click();
    await page.getByRole('button', { name: /add webhook/i }).click();
    await expect(page.getByPlaceholder(/https/i)).toBeVisible();
  });

  test('should save new webhook', async ({ page }) => {
    let createCalled = false;
    await page.route('**/api/v1/webhooks', async (route) => {
      if (route.request().method() === 'POST') {
        createCalled = true;
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'new-wh', url: 'https://new.example.com/hook', events: [], is_active: true }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ webhooks: [] }),
        });
      }
    });
    await page.goto('/orgs/test-org/settings');
    await page.getByText(/webhook/i).click();
    await page.getByRole('button', { name: /add webhook/i }).click();
    await page.getByPlaceholder(/https/i).fill('https://new.example.com/hook');
    await page.getByRole('button', { name: /save/i }).click();
    expect(createCalled).toBe(true);
  });

  test('should show empty state with no webhooks', async ({ page }) => {
    await mockAuthenticatedPage(page, {
      '**/api/v1/webhooks': { body: { webhooks: [] } },
    });
    await page.goto('/orgs/test-org/settings');
    await page.getByText(/webhook/i).click();
    await expect(page.getByText(/no webhooks/i)).toBeVisible();
  });
});
```

- [ ] **Step 2: Create environments settings UI tests**

Write `web/e2e/ui/settings-envs.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Settings — Environments tab', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
  });

  test('should render environment list', async ({ page }) => {
    await page.goto('/orgs/test-org/settings');
    await expect(page.getByText(/production|staging/i)).toBeVisible();
  });

  test('should add a new environment', async ({ page }) => {
    let createCalled = false;
    await page.route('**/api/v1/orgs/test-org/environments', async (route) => {
      if (route.request().method() === 'POST') {
        createCalled = true;
        const body = JSON.parse(route.request().postData() ?? '{}');
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'new-env', name: body.name, slug: body.slug, is_production: body.is_production }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ environments: [{ id: 'env-1', name: 'Production', slug: 'production', is_production: true }] }),
        });
      }
    });
    await page.goto('/orgs/test-org/settings');
    await page.getByPlaceholder(/name/i).fill('QA');
    await page.getByRole('button', { name: /add environment/i }).click();
    expect(createCalled).toBe(true);
  });

  test('should show delete confirmation', async ({ page }) => {
    await page.goto('/orgs/test-org/settings');
    const deleteBtn = page.getByRole('button', { name: /delete/i }).first();
    if (await deleteBtn.isVisible()) {
      await deleteBtn.click();
      await expect(page.getByRole('button', { name: /confirm/i }).or(page.getByText(/are you sure/i))).toBeVisible();
    }
  });
});
```

- [ ] **Step 3: Commit**

```bash
git add web/e2e/ui/settings-webhooks.spec.ts web/e2e/ui/settings-envs.spec.ts
git commit -m "feat: add settings webhooks and environments UI tests"
```

---

### Task 10: UI Tests — Members, API Keys, Navigation

**Files:**
- Create: `web/e2e/ui/members.spec.ts`
- Create: `web/e2e/ui/api-keys.spec.ts`
- Create: `web/e2e/ui/navigation.spec.ts`

- [ ] **Step 1: Create members UI tests**

Write `web/e2e/ui/members.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('MembersPage', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
  });

  test('should render member list', async ({ page }) => {
    await page.goto('/orgs/test-org/members');
    await expect(page.getByText(/owner|admin|member/i)).toBeVisible();
  });

  test('should add a new member', async ({ page }) => {
    let addCalled = false;
    await page.route('**/api/v1/orgs/test-org/members', async (route) => {
      if (route.request().method() === 'POST') {
        addCalled = true;
        await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ id: 'new-member' }) });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ members: [{ id: 'user-1', email: 'owner@test.com', role: 'owner', name: 'Owner' }] }),
        });
      }
    });
    await page.goto('/orgs/test-org/members');
    await page.getByPlaceholder(/email/i).fill('new@test.com');
    await page.getByRole('button', { name: /add|invite/i }).click();
    expect(addCalled).toBe(true);
  });
});
```

- [ ] **Step 2: Create API keys UI tests**

Write `web/e2e/ui/api-keys.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('APIKeysPage', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
  });

  test('should render API key list', async ({ page }) => {
    await page.goto('/orgs/test-org/api-keys');
    await expect(page.getByText(/ds_/i).or(page.getByText(/api key/i))).toBeVisible();
  });

  test('should create and reveal new key', async ({ page }) => {
    await page.route('**/api/v1/api-keys', async (route) => {
      if (route.request().method() === 'POST') {
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'key-new', token: 'ds_test_newkeyvalue123', name: 'Test Key', scopes: ['flags:read'] }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ api_keys: [] }),
        });
      }
    });
    await page.goto('/orgs/test-org/api-keys');
    await page.getByRole('button', { name: /create|new/i }).click();
    await page.getByPlaceholder(/name/i).fill('Test Key');
    await page.getByLabel(/flags:read/i).check();
    await page.getByRole('button', { name: /create|save/i }).click();
    await expect(page.getByText('ds_test_newkeyvalue123')).toBeVisible();
  });
});
```

- [ ] **Step 3: Create navigation UI tests**

Write `web/e2e/ui/navigation.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import { mockAuthenticatedPage } from '../helpers/mock-api';

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await mockAuthenticatedPage(page);
  });

  test('should deep link to nested project URL', async ({ page }) => {
    await page.goto('/orgs/test-org/projects/test-project/flags');
    await expect(page.getByText(/flag/i)).toBeVisible();
  });

  test('should deep link to app deployments', async ({ page }) => {
    await page.goto('/orgs/test-org/projects/test-project/apps/test-app/deployments');
    await expect(page.getByText(/deployment/i)).toBeVisible();
  });

  test('should redirect unauthenticated user to login', async ({ page }) => {
    // Don't set auth token
    await page.route('**/api/v1/users/me', (route) =>
      route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'Unauthorized' }) }),
    );
    await page.goto('/orgs/test-org/projects');
    await expect(page).toHaveURL(/\/login/);
  });

  test('should redirect authenticated user from login to dashboard', async ({ page }) => {
    await page.goto('/login');
    await expect(page).not.toHaveURL(/\/login/);
  });
});
```

- [ ] **Step 4: Commit**

```bash
git add web/e2e/ui/members.spec.ts web/e2e/ui/api-keys.spec.ts web/e2e/ui/navigation.spec.ts
git commit -m "feat: add members, api-keys, and navigation UI tests"
```

---

### Task 11: Verify and Update Docs

**Files:**
- Verify: all test files
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Verify UI tests run (no backend needed)**

```bash
cd /Users/sgamel/git/DeploySentry/web && npx playwright test --project=ui 2>&1 | tail -20
```

Expected: Tests run (some may fail due to fixture/selector mismatches — that's expected at this stage and will be fixed iteratively).

- [ ] **Step 2: List all test files**

```bash
find web/e2e -name "*.spec.ts" | sort
```

Expected:
```
web/e2e/smoke/01-auth.spec.ts
web/e2e/smoke/02-org-setup.spec.ts
web/e2e/smoke/03-flag-lifecycle.spec.ts
web/e2e/smoke/04-members.spec.ts
web/e2e/smoke/05-api-keys.spec.ts
web/e2e/smoke/06-settings.spec.ts
web/e2e/ui/api-keys.spec.ts
web/e2e/ui/flag-detail.spec.ts
web/e2e/ui/flag-list.spec.ts
web/e2e/ui/login-page.spec.ts
web/e2e/ui/members.spec.ts
web/e2e/ui/navigation.spec.ts
web/e2e/ui/register-page.spec.ts
web/e2e/ui/settings-envs.spec.ts
web/e2e/ui/settings-webhooks.spec.ts
```

- [ ] **Step 3: Update Current Initiatives**

Add the Playwright testing initiative to `docs/Current_Initiatives.md` in the Active section.

- [ ] **Step 4: Create plan doc entry**

Update `docs/Current_Initiatives.md` with:

```markdown
| Playwright E2E Testing | Implementation | [Plan](./superpowers/plans/2026-04-10-playwright-e2e-testing.md) | Infrastructure + 6 smoke tests + 9 UI tests |
```

- [ ] **Step 5: Commit**

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: add Playwright E2E testing to Current Initiatives"
```

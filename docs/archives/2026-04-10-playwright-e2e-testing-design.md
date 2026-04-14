# Playwright E2E Testing Strategy — Design Spec

## Overview

Comprehensive E2E test coverage for the DeploySentry web dashboard using Playwright. Two test layers: a smoke suite that runs against the real backend to validate integration, and a UI suite that uses mocked API responses to validate all page interactions in isolation.

## Architecture

```
web/
  e2e/
    smoke/                    # Flow-based tests against real backend
      auth.spec.ts
      org-setup.spec.ts
      flag-lifecycle.spec.ts
      deployment-actions.spec.ts
      release-actions.spec.ts
      members.spec.ts
      api-keys.spec.ts
      settings-environments.spec.ts
      settings-webhooks.spec.ts
      settings-general.spec.ts
    ui/                       # Page-based tests with mocked API
      login-page.spec.ts
      register-page.spec.ts
      project-list.spec.ts
      app-list.spec.ts
      flag-list.spec.ts
      flag-create.spec.ts
      flag-detail.spec.ts
      deployments.spec.ts
      deployment-detail.spec.ts
      releases.spec.ts
      release-detail.spec.ts
      settings-envs.spec.ts
      settings-webhooks.spec.ts
      settings-notifications.spec.ts
      settings-general.spec.ts
      members.spec.ts
      api-keys.spec.ts
      navigation.spec.ts
    fixtures/                 # Shared JSON response fixtures
      auth.json
      orgs.json
      projects.json
      apps.json
      flags.json
      deployments.json
      releases.json
      members.json
      api-keys.json
      webhooks.json
      notifications.json
      settings.json
      environments.json
    helpers/
      api-client.ts           # HTTP helper for smoke test data seeding
      mock-api.ts              # page.route() interceptor setup from fixtures
      auth.ts                  # Login helper with storageState caching
  playwright.config.ts         # Two projects: smoke (serial) and ui (parallel)
```

## Layer 1: Smoke Suite (Real Backend)

### Purpose

Validate that the frontend and backend integrate correctly end-to-end. Catches mismatches in API contracts, auth flows, data persistence, and cross-service interactions.

### Prerequisites

```bash
make dev-up       # PostgreSQL, Redis, NATS
make migrate-up   # Apply all migrations
make run-api      # Start API on :8080
npm run dev       # Start Vite dev server on :3001 (or run against built static files)
```

### Test Data Strategy

Hybrid API-based seeding — no raw SQL. Each flow test uses the app's own endpoints:

1. `auth.spec.ts` runs first (serial dependency) — registers a test user, stores the auth token
2. `org-setup.spec.ts` runs second — creates an org, project, and app via API
3. Subsequent tests use the seeded org/project/app context

The `api-client.ts` helper wraps `fetch()` calls to the backend directly (bypassing the browser) for fast setup/teardown.

```typescript
// helpers/api-client.ts — shape
export class ApiClient {
  constructor(private baseUrl: string, private token?: string) {}

  async register(email: string, password: string, name: string): Promise<{ token: string }>
  async login(email: string, password: string): Promise<{ token: string }>
  async createOrg(name: string, slug: string): Promise<Org>
  async createProject(orgSlug: string, name: string, slug: string): Promise<Project>
  async createApp(orgSlug: string, projectSlug: string, name: string, slug: string): Promise<App>
  async createFlag(projectId: string, data: CreateFlagRequest): Promise<Flag>
  async createWebhook(data: { url: string; events: string[] }): Promise<Webhook>
  async addMember(orgSlug: string, email: string, role: string): Promise<void>
  async createApiKey(name: string, scopes: string[]): Promise<ApiKey>
  async createEnvironment(orgSlug: string, name: string, slug: string, isProduction: boolean): Promise<Environment>
}
```

### Auth State Caching

Playwright's `storageState` feature saves authenticated state to a JSON file after first login:

```typescript
// helpers/auth.ts
const AUTH_FILE = path.join(__dirname, '..', '.auth', 'user.json');

export async function ensureAuthenticated(browser: Browser): Promise<BrowserContext> {
  if (fs.existsSync(AUTH_FILE)) {
    return browser.newContext({ storageState: AUTH_FILE });
  }
  const context = await browser.newContext();
  const page = await context.newPage();
  // ... login via UI ...
  await context.storageState({ path: AUTH_FILE });
  return context;
}
```

### Smoke Test Inventory

| File | Flow | Tests |
|------|------|-------|
| `auth.spec.ts` | Auth lifecycle | Register with valid creds → redirects to `/`. Login with valid creds → redirects to `/`. Login with invalid creds → error message. Session persists on reload. Logout clears session → redirects to `/login`. Protected route without auth → redirects to `/login`. |
| `org-setup.spec.ts` | Entity hierarchy | Create org → slug auto-generates, redirects to projects. Create project → appears in project list. Create app → appears in app list. |
| `flag-lifecycle.spec.ts` | Flag CRUD | Create boolean flag with category and owners. Flag appears in list. View flag detail → correct metadata. Toggle flag enabled/disabled. Add percentage targeting rule → rule appears in table. Archive flag → confirmation, flag removed from active list. |
| `deployment-actions.spec.ts` | Deployment workflow | Navigate to deployments list. View deployment detail. Verify action buttons match deployment status (if data exists). |
| `release-actions.spec.ts` | Release workflow | Navigate to releases list. Tab filters work (all/draft/rolling_out/etc.). View release detail. Verify action buttons match release status (if data exists). |
| `members.spec.ts` | Membership | Add member by email with role. Member appears in list. Change member role via dropdown. Remove member with confirmation. |
| `api-keys.spec.ts` | API keys | Create key with name and scopes. Key token revealed with copy button. Key appears in list. Revoke key with confirmation. Key removed from list. |
| `settings-environments.spec.ts` | Environments | Create environment (name, slug, production toggle). Environment appears in list. Delete environment with confirmation. |
| `settings-webhooks.spec.ts` | Webhooks | Create webhook with URL and events. Webhook appears in list. Test webhook → success/failure feedback. Delete webhook. |
| `settings-general.spec.ts` | Settings persistence | Update project name → save → verify persisted on reload. Update app description → save → verify persisted. |

### Smoke Test Execution

- Run **serially** — tests share state (user → org → project created by earlier tests)
- **0 retries** — flakiness in smoke tests means a real problem
- Test ordering enforced via Playwright's `test.describe.serial` or file naming (`01-auth.spec.ts`, `02-org-setup.spec.ts`, etc.)
- Cleanup: a `globalTeardown` script optionally drops the test user/org via API

## Layer 2: UI Suite (Mocked API)

### Purpose

Validate all UI interactions, form behaviors, error states, and navigation without needing a running backend. Fast, deterministic, parallelizable.

### Mock Strategy

The `mock-api.ts` helper intercepts all `/api/v1/*` routes and returns fixture data:

```typescript
// helpers/mock-api.ts — shape
export async function setupMockApi(page: Page, overrides?: Partial<MockOverrides>): Promise<void> {
  // Auth
  await page.route('**/api/v1/auth/login', async (route) => { ... });
  await page.route('**/api/v1/users/me', async (route) => { ... });

  // Flags
  await page.route('**/api/v1/flags', async (route) => { ... });
  await page.route('**/api/v1/flags/*', async (route) => { ... });

  // ... all API routes with fixture responses ...
}
```

The `overrides` parameter lets individual tests customize responses:

```typescript
await setupMockApi(page, {
  flags: { list: { flags: [] } },  // empty state test
});
```

For testing error states:

```typescript
await page.route('**/api/v1/flags', (route) =>
  route.fulfill({ status: 500, body: JSON.stringify({ error: 'Internal server error' }) })
);
```

### Fixture Files

Each fixture file contains response shapes matching the real API:

```json
// fixtures/flags.json
{
  "list": {
    "flags": [
      {
        "id": "flag-1",
        "key": "dark-mode",
        "name": "Dark Mode",
        "enabled": true,
        "flag_type": "boolean",
        "default_value": "false",
        "metadata": {
          "category": "feature",
          "purpose": "Enable dark mode for users",
          "owners": ["frontend-team"],
          "is_permanent": false,
          "tags": ["ui", "theme"]
        }
      }
    ]
  },
  "detail": { ... },
  "rules": { ... }
}
```

### UI Test Inventory

| File | Page | Tests |
|------|------|-------|
| `login-page.spec.ts` | LoginPage | Renders form fields. Submit with valid creds → redirect. Submit with empty fields → validation. Submit with invalid creds → error message. "Create account" link navigates to register. |
| `register-page.spec.ts` | RegisterPage | Renders all fields. Submit with valid data → redirect. Password mismatch → error. Existing email → API error displayed. "Sign in" link navigates to login. |
| `project-list.spec.ts` | ProjectListPage | Renders project cards. Empty state message when no projects. "Create Project" button navigates. Click project card navigates to flags. |
| `app-list.spec.ts` | ApplicationsListPage | Renders app cards with descriptions. Empty state. "Create App" button. Click card navigates. |
| `flag-list.spec.ts` | FlagListPage | Renders flag table. Search filters by name/key. Category dropdown filters. Status filter (all/enabled/disabled/archived). Pagination controls. Empty state. "Create Flag" button. |
| `flag-create.spec.ts` | FlagCreatePage | All field types render (boolean, string, integer, JSON). Category selector (5 options). Release category requires expiration or permanent toggle. Owners comma-separated input. Tags input. Submit creates flag → redirect. Validation: key is required, slug auto-generates. |
| `flag-detail.spec.ts` | FlagDetailPage | Header shows flag metadata. Toggle switch calls API. Targeting rules tab shows rules table. Add rule form → rule appears. Edit rule inline. Delete rule with confirm. Archive button in danger zone → confirm dialog. Environments tab shows state. |
| `deployments.spec.ts` | DeploymentsPage | Table renders deployment rows. Strategy filter (canary/blue-green/rolling). Status filter. Traffic % and health score display. Empty state. |
| `deployment-detail.spec.ts` | DeploymentDetailPage | Header with status badge. Info cards (traffic, health, duration). Action buttons based on status: pending (Start, Cancel), running (Promote, Pause, Rollback), paused (Resume, Rollback), failed (Rollback). Button click calls correct API endpoint. |
| `releases.spec.ts` | ReleasesPage | Status tab filter (all/draft/rolling_out/paused/completed/rolled_back). Release list rows. Traffic % display. Empty state per tab. |
| `release-detail.spec.ts` | ReleaseDetailPage | Header with status badge and traffic. Action buttons per status: draft (Start Rollout, Delete), rolling_out (Promote, Pause, Rollback), paused (Resume, Rollback). Flag changes section. |
| `settings-envs.spec.ts` | SettingsPage (envs) | Environment list renders from API. Add form: name input, auto-slug, production checkbox. Submit creates → list updates. Delete with confirm → list updates. Loading spinner. API error → error banner with retry. |
| `settings-webhooks.spec.ts` | SettingsPage (webhooks) | Webhook list from API. "Add Webhook" expands inline form. URL, events, active toggle inputs. Save → list updates. Edit existing webhook inline. Test button → success/failure inline. Delete → list updates. Empty state. |
| `settings-notifications.spec.ts` | SettingsPage (notifications) | Channel cards render with source badges (config vs api). Event routing table. Save button calls PUT. Reset button calls DELETE with confirm. Loading/saving states. |
| `settings-general.spec.ts` | SettingsPage (project/app) | Project: name, default env, stale threshold fields. Save calls API. App: name, description, repo URL. Save calls API. Danger zone: delete app button → confirm prompt → API call → redirect. Success feedback on save (auto-dismiss). |
| `members.spec.ts` | MembersPage | Member list renders. Add form: email input, role dropdown (member/admin/viewer). Submit → member appears. Change role via dropdown → API call. Remove member → confirm → removed from list. Owner row is not removable. |
| `api-keys.spec.ts` | APIKeysPage | Key list renders. Create form: name, scope checkboxes (flags:read, flags:write, deploys:read, deploys:write, admin). Submit → key token revealed. Copy button. Key appears in list with prefix. Revoke → confirm → removed. |
| `navigation.spec.ts` | Cross-page | Sidebar reflects current org/project/app. Clicking sidebar items navigates correctly. URL params load correct page state. Deep link to nested URL works (e.g., `/orgs/acme/projects/web/flags`). Browser back/forward works. |

## Configuration

### `playwright.config.ts`

```typescript
export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: false, // controlled per-project

  projects: [
    {
      name: 'smoke',
      testDir: './e2e/smoke',
      use: {
        baseURL: 'http://localhost:3001',
        browserName: 'chromium',
      },
      fullyParallel: false,  // serial — tests share state
      retries: 0,            // flakiness = real bug
    },
    {
      name: 'ui',
      testDir: './e2e/ui',
      use: {
        baseURL: 'http://localhost:3001',
        browserName: 'chromium',
      },
      fullyParallel: true,   // isolated via mocks
      retries: 1,            // timing tolerance
    },
  ],

  webServer: {
    command: 'npm run dev',
    port: 3001,
    reuseExistingServer: true,
  },
});
```

### NPM Scripts

```json
{
  "test:e2e": "npx playwright test",
  "test:e2e:smoke": "npx playwright test --project=smoke",
  "test:e2e:ui": "npx playwright test --project=ui",
  "test:e2e:report": "npx playwright show-report"
}
```

### Dependencies

Add to `web/package.json` devDependencies:
- `@playwright/test` (only dependency needed — Playwright bundles its own browsers)

## Test Conventions

### Selectors

Prefer `data-testid` attributes over CSS selectors or text matching for stability. Key elements should have `data-testid` attributes added to the page components:

```tsx
<button data-testid="flag-toggle" onClick={...}>Toggle</button>
<input data-testid="flag-search" placeholder="Search flags..." />
```

Where `data-testid` isn't practical (e.g., dynamic list items), use role-based selectors:

```typescript
page.getByRole('button', { name: 'Save' })
page.getByRole('textbox', { name: 'Email' })
page.getByRole('row').nth(2)
```

Text matching is a last resort and only for visible, stable UI text.

### Test Isolation (UI suite)

Each UI test file calls `setupMockApi(page)` in `beforeEach` and starts from a known state. Tests never depend on other tests within the same file.

### Naming

Tests use descriptive names following the pattern `should [action] when [condition]`:

```typescript
test('should display error message when login credentials are invalid', ...)
test('should add flag to list when create form is submitted', ...)
test('should show confirmation dialog when delete button is clicked', ...)
```

## Out of Scope

- Cross-browser testing (Chromium only for now)
- Visual regression testing (screenshots/snapshots)
- Performance/load testing
- Mobile viewport testing
- Accessibility auditing (though Playwright supports it — future enhancement)
- AnalyticsPage and SDKsPage tests (mostly static/informational content)

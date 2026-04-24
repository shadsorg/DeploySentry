/**
 * Screenshot capture for pages that have no mockup in newscreens/.
 *
 * Each screenshot is named so it maps 1:1 to the page component and route,
 * making it easy to attach a corresponding mockup later (e.g. drop a file
 * named `release-detail.html` into `newscreens/`).
 *
 * Output: docs/ui-audit/screenshots/<route-key>.png
 *
 * Run:
 *   npx playwright test e2e/screenshots/no-mockup-pages.spec.ts --project=ui
 */
import { test } from '@playwright/test';
import * as path from 'path';
import * as fs from 'fs';
import { fileURLToPath } from 'url';
import { setupMockApi, mockAuthenticatedPage } from '../helpers/mock-api';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const OUT_DIR = path.resolve(__dirname, '../../../docs/ui-audit/screenshots');

test.beforeAll(() => {
  fs.mkdirSync(OUT_DIR, { recursive: true });
});

test.use({ viewport: { width: 1440, height: 900 } });

interface PageCase {
  name: string;        // file stem — also the implicit mockup filename
  component: string;   // for the audit doc cross-ref
  route: string;
  authed: boolean;
  overrides?: Record<string, { status?: number; body: unknown }>;
  fullPage?: boolean;
}

// Reusable mock objects ------------------------------------------------------

const FAKE_ROLLOUT = {
  id: '11111111-2222-3333-4444-555555555555',
  status: 'active',
  target_type: 'deploy',
  target_ref: { deployment_id: '99999999-aaaa-bbbb-cccc-dddddddddddd' },
  current_phase_index: 1,
  strategy_snapshot: {
    name: 'canary-progressive',
    steps: [
      { percent: 5, min_duration: 0, max_duration: 0, bake_time_healthy: 0 },
      { percent: 25, min_duration: 0, max_duration: 0, bake_time_healthy: 0 },
      { percent: 50, min_duration: 0, max_duration: 0, bake_time_healthy: 0 },
      { percent: 100, min_duration: 0, max_duration: 0, bake_time_healthy: 0 },
    ],
  },
  created_at: '2026-04-22T12:34:00Z',
  rollback_reason: '',
};

const FAKE_GROUP = {
  group: {
    id: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
    scope_type: 'org',
    scope_id: 'test-org',
    name: 'Web frontend cluster',
    description: 'Coordinated rollout for the web/frontend stack across regions.',
    coordination_policy: 'pause_on_sibling_abort',
    created_by: 'alice@example.com',
    created_at: '2026-04-20T08:00:00Z',
    updated_at: '2026-04-22T12:00:00Z',
  },
  members: [
    {
      id: '11111111-2222-3333-4444-555555555555',
      status: 'active',
      target_type: 'deploy',
      target_ref: {},
      current_phase_index: 1,
      strategy_snapshot: { name: 'canary-progressive', steps: [] },
      created_at: '2026-04-22T12:00:00Z',
    },
    {
      id: '66666666-7777-8888-9999-000000000000',
      status: 'succeeded',
      target_type: 'config',
      target_ref: {},
      current_phase_index: 0,
      strategy_snapshot: { name: 'config-fast', steps: [] },
      created_at: '2026-04-21T08:00:00Z',
    },
  ],
};

const FAKE_RELEASE = {
  id: 'rel-1',
  app_id: 'app-1',
  name: 'v2.4.0 — homepage redesign',
  description: 'Roll out the new marketing homepage and updated nav.',
  status: 'rolling_out',
  traffic_percent: 25,
  session_sticky: true,
  sticky_header: 'X-DS-Session',
  created_by: 'alice@example.com',
  created_at: '2026-04-22T08:00:00Z',
};

const FAKE_DEPLOYMENT_DETAIL = {
  id: 'dep-1',
  app_id: 'app-1',
  status: 'completed',
  artifact: 'app:v2.4.0',
  version: 'v2.4.0',
  environment: 'production',
  flag_test_key: 'homepage_redesign_v2',
  agents: [],
  heartbeats: [],
  created_at: '2026-04-22T08:00:00Z',
  updated_at: '2026-04-22T09:00:00Z',
};

// Cases ----------------------------------------------------------------------

const CASES: PageCase[] = [
  // Public / unauthenticated
  { name: 'landing',           component: 'LandingPage.tsx',           route: '/',         authed: false, fullPage: true },
  { name: 'login',             component: 'LoginPage.tsx',             route: '/login',    authed: false },
  { name: 'register',          component: 'RegisterPage.tsx',          route: '/register', authed: false },

  // Create flows
  { name: 'create-org',        component: 'CreateOrgPage.tsx',         route: '/orgs/new',                                                            authed: true },
  { name: 'create-project',    component: 'CreateProjectPage.tsx',     route: '/orgs/test-org/projects/new',                                          authed: true },
  { name: 'create-app',        component: 'CreateAppPage.tsx',         route: '/orgs/test-org/projects/test-project/apps/new',                        authed: true },

  // List/shell pages
  { name: 'project-list',      component: 'ProjectListPage.tsx',       route: '/orgs/test-org/projects',                                              authed: true, fullPage: true },
  { name: 'project-shell',     component: 'ProjectPage.tsx',           route: '/orgs/test-org/projects/test-project',                                  authed: true, fullPage: true },
  { name: 'flag-create',       component: 'FlagCreatePage.tsx',        route: '/orgs/test-org/projects/test-project/flags/new',                       authed: true },

  // Releases
  { name: 'releases-list',     component: 'ReleasesPage.tsx',          route: '/orgs/test-org/projects/test-project/apps/test-app/releases',          authed: true, fullPage: true },
  {
    name: 'release-detail',
    component: 'ReleaseDetailPage.tsx',
    route: '/orgs/test-org/projects/test-project/apps/test-app/releases/rel-1',
    authed: true,
    overrides: { '/api/v1/releases/rel-1': { body: FAKE_RELEASE } },
  },

  // Deployment detail
  {
    name: 'deployment-detail',
    component: 'DeploymentDetailPage.tsx',
    route: '/orgs/test-org/projects/test-project/apps/test-app/deployments/dep-1',
    authed: true,
    fullPage: true,
    overrides: { '/api/v1/deployments/dep-1': { body: FAKE_DEPLOYMENT_DETAIL } },
  },

  // Rollouts (detail variants — list pages have mockups)
  {
    name: 'rollout-detail',
    component: 'RolloutDetailPage.tsx',
    route: '/orgs/test-org/rollouts/11111111-2222-3333-4444-555555555555',
    authed: true,
    fullPage: true,
    overrides: {
      '/api/v1/orgs/test-org/rollouts/11111111-2222-3333-4444-555555555555': { body: FAKE_ROLLOUT },
      '/api/v1/orgs/test-org/rollouts/11111111-2222-3333-4444-555555555555/events': { body: { items: [] } },
    },
  },
  {
    name: 'rollout-group-detail',
    component: 'RolloutGroupDetailPage.tsx',
    route: '/orgs/test-org/rollout-groups/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
    authed: true,
    fullPage: true,
    overrides: {
      '/api/v1/orgs/test-org/rollout-groups/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee': { body: FAKE_GROUP },
    },
  },

  // Settings (app-level variant — org/project levels are covered by mockups)
  {
    name: 'settings-app',
    component: 'SettingsPage.tsx (level=app)',
    route: '/orgs/test-org/projects/test-project/apps/test-app/settings',
    authed: true,
    fullPage: true,
  },
];

for (const c of CASES) {
  test(`screenshot: ${c.name}  (${c.component})`, async ({ page }) => {
    if (c.authed) {
      await mockAuthenticatedPage(page, c.overrides);
    } else {
      await setupMockApi(page, c.overrides);
    }
    await page.goto(c.route, { waitUntil: 'networkidle' });
    // Give late-arriving renders (e.g. SSE-driven detail pages) a beat to settle
    await page.waitForTimeout(500);
    await page.screenshot({
      path: path.join(OUT_DIR, `${c.name}.png`),
      fullPage: c.fullPage ?? false,
    });
  });
}

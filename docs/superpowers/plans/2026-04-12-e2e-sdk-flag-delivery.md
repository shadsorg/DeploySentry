# End-to-End SDK Flag Delivery Test Suite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a hermetic end-to-end test suite that creates a flag in the web dashboard and asserts that Node and React SDK probes observe the propagated value within a latency budget, covering the full UI → API → Sentinel SSE → SDK chain.

**Architecture:** A new `sdk` Playwright project runs one spec file containing three scenarios (baseline propagation, targeting correctness, variant delivery). The spec drives the React dashboard, then launches SDK probes — a Node subprocess reading observations from a JSONL file, and a second Playwright browser context loading a minimal React harness page that exposes observed flag values on `window.__ds_observations`. The whole suite runs against a hermetic `docker-compose.e2e.yml` stack spun up per CI run on a disjoint port range so `make dev-up` stays usable.

**Tech Stack:** Playwright (TypeScript), Node 18+, Docker Compose, existing `sdk/node` and `sdk/react` packages consumed via local path references, Vite for the React harness build, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-04-12-e2e-sdk-flag-delivery-design.md`

**Scope:** Phases 1–4 from the spec (infra → scenario A → B+C → required gate). Flutter probe (Phase 5) is out of scope for this plan and will be covered by a follow-up plan.

---

## Task 1: Hermetic compose stack

**Files:**
- Create: `docker-compose.e2e.yml` (repo root)
- Create: `deploy/e2e/api.env`

**Responsibility:** Stand up postgres, redis, nats, deploysentry-api, and sentinel (if it is a separate binary — otherwise the api container serves both REST and SSE) on disjoint ports from `make dev-up`, with healthchecks so `docker compose up --wait` returns only when everything is ready.

- [ ] **Step 1: Inspect the existing dev compose file**

Run: `cat deploy/docker-compose.yml`
Expected: shows the services dev-up uses; note their images, env vars, and ports.

- [ ] **Step 2: Inspect whether sentinel is a separate binary**

Run: `ls cmd/ && grep -rn "sentinel" cmd/ internal/ | head -20`
Expected: confirms whether sentinel runs in-process with the API or as a separate binary. The compose file in the next step must match.

- [ ] **Step 3: Write `docker-compose.e2e.yml`**

Create at repo root. Use port `15432` (postgres), `16379` (redis), `14222` (nats), `18080` (api), `18090` (sentinel, if separate). All volumes must be named `deploysentry-e2e-*` and declared at the bottom so `docker compose down -v` wipes them cleanly.

```yaml
name: deploysentry-e2e

services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: deploysentry
      POSTGRES_PASSWORD: deploysentry
      POSTGRES_DB: deploysentry
    ports:
      - "15432:5432"
    volumes:
      - deploysentry-e2e-pg:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U deploysentry"]
      interval: 2s
      timeout: 3s
      retries: 15

  redis:
    image: redis:7
    ports:
      - "16379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 2s
      timeout: 3s
      retries: 15

  nats:
    image: nats:2.10
    command: ["-js"]
    ports:
      - "14222:4222"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8222/healthz"]
      interval: 2s
      timeout: 3s
      retries: 15

  api:
    build:
      context: .
      dockerfile: deploy/Dockerfile.api
    env_file: deploy/e2e/api.env
    ports:
      - "18080:8080"
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
      nats:     { condition: service_healthy }
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]
      interval: 2s
      timeout: 3s
      retries: 30

volumes:
  deploysentry-e2e-pg:
```

(If sentinel is a separate binary, add a second `sentinel` service mirroring the `api` block and exposing `18090:8090`.)

- [ ] **Step 4: Write `deploy/e2e/api.env`**

```
DS_DATABASE_DSN=postgres://deploysentry:deploysentry@postgres:5432/deploysentry?sslmode=disable&search_path=deploy
DS_DATABASE_SCHEMA=deploy
DS_REDIS_URL=redis://redis:6379
DS_NATS_URL=nats://nats:4222
DS_HTTP_ADDR=:8080
DS_LOG_LEVEL=info
DS_AUTO_MIGRATE=true
```

(If `DS_AUTO_MIGRATE=true` is not supported by the API, remove that line — migrations will be run as a separate step in task 2.)

- [ ] **Step 5: Run compose up, verify healthy, tear down**

```bash
docker compose -f docker-compose.e2e.yml up -d --wait
curl -sf http://localhost:18080/healthz
docker compose -f docker-compose.e2e.yml down -v
```

Expected: `--wait` returns 0, `/healthz` returns 200, `down -v` cleans up volumes. If `--wait` hangs, inspect `docker compose logs` and fix the failing healthcheck or env var before proceeding.

- [ ] **Step 6: Commit**

```bash
git add docker-compose.e2e.yml deploy/e2e/api.env
git commit -m "feat(e2e): add hermetic compose stack for SDK e2e tests"
```

---

## Task 2: Makefile targets for the e2e stack

**Files:**
- Modify: `Makefile`

**Responsibility:** Add `e2e-sdk-up`, `e2e-sdk-down`, `e2e-sdk`, and `e2e-sdk-debug` targets that wrap compose and Playwright so developers and CI share one command.

- [ ] **Step 1: Inspect existing Makefile targets**

Run: `grep -n "^[a-z].*:" Makefile | head -30`
Expected: shows existing targets; note the style (e.g., `.PHONY` declarations, variable conventions).

- [ ] **Step 2: Add e2e-sdk targets to the Makefile**

Append the following (matching the existing style — adjust `.PHONY` placement as appropriate):

```make
E2E_COMPOSE := docker compose -f docker-compose.e2e.yml
E2E_MIGRATE_DSN := postgres://deploysentry:deploysentry@localhost:15432/deploysentry?sslmode=disable&search_path=deploy

.PHONY: e2e-sdk-up e2e-sdk-down e2e-sdk e2e-sdk-debug

e2e-sdk-up:
	$(E2E_COMPOSE) up -d --wait
	migrate -path ./migrations -database "$(E2E_MIGRATE_DSN)" up

e2e-sdk-down:
	$(E2E_COMPOSE) down -v

e2e-sdk: e2e-sdk-up
	cd web && npx playwright test --project=sdk
	$(MAKE) e2e-sdk-down

e2e-sdk-debug: e2e-sdk-up
	cd web && npx playwright test --project=sdk --headed --debug
```

- [ ] **Step 3: Run `make e2e-sdk-up` then `make e2e-sdk-down`**

```bash
make e2e-sdk-up
curl -sf http://localhost:18080/healthz && echo OK
make e2e-sdk-down
```

Expected: stack comes up, `/healthz` returns 200, migrations apply cleanly, teardown completes. If `migrate` CLI isn't installed, note it in the plan's completion record — we'll add an install step to the CI workflow in Task 11.

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -m "feat(e2e): add make targets for SDK e2e stack"
```

---

## Task 3: Playwright `sdk` project configuration

**Files:**
- Modify: `web/playwright.config.ts`
- Create: `web/e2e/sdk/global-setup.ts`

**Responsibility:** Register a third Playwright project `sdk` with serial execution, 90s timeout, and a global setup that asserts the compose stack is healthy before any spec runs.

- [ ] **Step 1: Write the global setup**

Create `web/e2e/sdk/global-setup.ts`:

```typescript
import { request } from '@playwright/test';

const API_URL = process.env.DS_E2E_API_URL ?? 'http://localhost:18080';

export default async function globalSetup() {
  const ctx = await request.newContext();
  const res = await ctx.get(`${API_URL}/healthz`);
  if (!res.ok()) {
    throw new Error(
      `e2e stack not healthy at ${API_URL}/healthz (status ${res.status()}). ` +
      `Run 'make e2e-sdk-up' before 'npx playwright test --project=sdk'.`
    );
  }
  await ctx.dispose();
}
```

- [ ] **Step 2: Modify `web/playwright.config.ts` to add the `sdk` project**

Replace the existing `projects` array with:

```typescript
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
    {
      name: 'sdk',
      testDir: './e2e/sdk',
      timeout: 90_000,
      use: {
        baseURL: 'http://localhost:3001',
        browserName: 'chromium',
        trace: 'on-first-retry',
      },
      fullyParallel: false,
      workers: 1,
      retries: 2,
    },
  ],
```

At the top of the config, add:

```typescript
  globalSetup: require.resolve('./e2e/sdk/global-setup.ts'),
```

(Only runs when any project is executed, but the `sdk` project is the one that actually needs it. The `smoke` and `ui` projects are unaffected because the healthz check also returns 200 for the dev stack — verify this assumption in step 3 and, if the dev stack doesn't expose `/healthz`, gate the check on `process.env.PLAYWRIGHT_PROJECT === 'sdk'` or skip the global setup when `DS_E2E_API_URL` is unset.)

- [ ] **Step 3: Verify the sdk project is registered**

```bash
cd web && npx playwright test --project=sdk --list
```

Expected: lists zero tests (the `e2e/sdk` directory is empty), exits 0. If global setup fails because the stack isn't up, run `make e2e-sdk-up` first.

- [ ] **Step 4: Commit**

```bash
git add web/playwright.config.ts web/e2e/sdk/global-setup.ts
git commit -m "feat(e2e): register sdk Playwright project with health gate"
```

---

## Task 4: Node SDK probe harness

**Files:**
- Create: `web/e2e/sdk-probes/node-probe/package.json`
- Create: `web/e2e/sdk-probes/node-probe/index.js`

**Responsibility:** A standalone Node script that connects via `@deploysentry/sdk` using env-provided config, appends every flag observation as a JSON line to `OBSERVATIONS_FILE`, and exits cleanly on SIGTERM.

- [ ] **Step 1: Confirm the Node SDK's public API for streaming and evaluation**

Run: `grep -nE "export|isEnabled|getString|onFlagUpdate" sdk/node/src/index.ts sdk/node/src/client.ts | head -30`
Expected: note the exact exported class name (e.g., `DeploySentryClient`), the `init()` method, and the getters for flag values (`isEnabled`, `getString`, `getVariant`, etc.). The probe code below uses `DeploySentryClient` and `isEnabled` / `getString` — rename in step 3 if the actual exports differ.

- [ ] **Step 2: Write `web/e2e/sdk-probes/node-probe/package.json`**

```json
{
  "name": "ds-e2e-node-probe",
  "private": true,
  "type": "module",
  "dependencies": {
    "@deploysentry/sdk": "file:../../../../sdk/node"
  }
}
```

- [ ] **Step 3: Write `web/e2e/sdk-probes/node-probe/index.js`**

```javascript
import fs from 'node:fs';
import { DeploySentryClient } from '@deploysentry/sdk';

const {
  DS_API_URL,
  DS_API_KEY,
  DS_PROJECT,
  DS_ENVIRONMENT,
  DS_CONTEXT_JSON,
  DS_FLAG_KEYS,
  OBSERVATIONS_FILE,
  POLL_MS = '50',
} = process.env;

if (!DS_API_URL || !DS_API_KEY || !DS_FLAG_KEYS || !OBSERVATIONS_FILE) {
  console.error('node-probe: missing required env vars');
  process.exit(2);
}

const flagKeys = DS_FLAG_KEYS.split(',').map((k) => k.trim()).filter(Boolean);
const context = DS_CONTEXT_JSON ? JSON.parse(DS_CONTEXT_JSON) : {};
const client = new DeploySentryClient({
  apiUrl: DS_API_URL,
  apiKey: DS_API_KEY,
  project: DS_PROJECT,
  environment: DS_ENVIRONMENT,
});

const last = new Map();

function record(flagKey, value) {
  const prev = last.get(flagKey);
  const serialized = JSON.stringify(value);
  if (prev === serialized) return;
  last.set(flagKey, serialized);
  fs.appendFileSync(
    OBSERVATIONS_FILE,
    JSON.stringify({ flagKey, value, ts: Date.now() }) + '\n'
  );
}

async function tick() {
  for (const key of flagKeys) {
    try {
      const value = await client.isEnabled(key, false, context);
      record(key, value);
    } catch (err) {
      fs.appendFileSync(
        OBSERVATIONS_FILE,
        JSON.stringify({ flagKey: key, error: String(err), ts: Date.now() }) + '\n'
      );
    }
  }
}

await client.init();
await tick();
const interval = setInterval(tick, Number(POLL_MS));

function shutdown() {
  clearInterval(interval);
  client.close?.();
  process.exit(0);
}

process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);
```

Note the polling pattern: the Node SDK already holds a locally cached flag state that streaming updates, so `isEnabled` is a cache read and polling it every 50ms is cheap. If the SDK exposes a proper `onFlagUpdate` callback, replace the `setInterval` with event subscription in step 4.

- [ ] **Step 4: Install probe deps and smoke-test the probe standalone**

```bash
cd web/e2e/sdk-probes/node-probe && npm install
# Smoke: no API, expect the probe to log a connect error and exit non-zero after a short time
OBSERVATIONS_FILE=/tmp/obs.jsonl \
DS_API_URL=http://localhost:18080 \
DS_API_KEY=invalid \
DS_PROJECT=p \
DS_ENVIRONMENT=e \
DS_FLAG_KEYS=e2e-baseline \
timeout 5 node index.js || true
cat /tmp/obs.jsonl 2>/dev/null || echo "no observations — expected if API not up"
```

Expected: the probe runs without syntax errors; observations file is either empty or contains error lines (stack must be up for real observations, which happens in task 8).

- [ ] **Step 5: Commit**

```bash
git add web/e2e/sdk-probes/node-probe/
git commit -m "feat(e2e): add Node SDK probe harness"
```

---

## Task 5: React SDK probe harness

**Files:**
- Create: `web/e2e/sdk-probes/react-harness/package.json`
- Create: `web/e2e/sdk-probes/react-harness/vite.config.ts`
- Create: `web/e2e/sdk-probes/react-harness/index.html`
- Create: `web/e2e/sdk-probes/react-harness/src/main.tsx`

**Responsibility:** A tiny Vite-built static page that mounts `<DeploySentryProvider>` from `sdk/react`, subscribes to flag keys passed via query params, and appends every observed value to `window.__ds_observations` for Playwright to read.

- [ ] **Step 1: Confirm the React SDK's provider + hook API**

Run: `grep -nE "Provider|useFlag" sdk/react/src/index.ts sdk/react/src/provider.tsx sdk/react/src/hooks.ts | head -20`
Expected: confirms `DeploySentryProvider` props shape (apiUrl, apiKey, project, environment, context) and `useFlag(key, defaultValue)` signature. Update step 4 accordingly if props differ.

- [ ] **Step 2: Write `package.json`**

```json
{
  "name": "ds-e2e-react-harness",
  "private": true,
  "type": "module",
  "scripts": {
    "build": "vite build",
    "preview": "vite preview --port 4310 --strictPort"
  },
  "dependencies": {
    "@deploysentry/react": "file:../../../../sdk/react",
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.3.1",
    "typescript": "^5.4.0",
    "vite": "^5.4.0"
  }
}
```

- [ ] **Step 3: Write `vite.config.ts`**

```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  build: { outDir: 'dist', emptyOutDir: true },
  server: { port: 4310, strictPort: true },
  preview: { port: 4310, strictPort: true },
});
```

- [ ] **Step 4: Write `index.html`**

```html
<!doctype html>
<html>
  <head><meta charset="utf-8" /><title>DS React Probe</title></head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 5: Write `src/main.tsx`**

```typescript
import React, { useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { DeploySentryProvider, useFlag } from '@deploysentry/react';

declare global {
  interface Window {
    __ds_observations: Array<{ flagKey: string; value: unknown; ts: number }>;
  }
}
window.__ds_observations = [];

function parseQuery() {
  const q = new URLSearchParams(window.location.search);
  return {
    apiUrl: q.get('apiUrl') ?? '',
    apiKey: q.get('apiKey') ?? '',
    project: q.get('project') ?? '',
    environment: q.get('environment') ?? '',
    flagKeys: (q.get('flagKeys') ?? '').split(',').filter(Boolean),
    context: q.get('context') ? JSON.parse(q.get('context')!) : {},
  };
}

function Observer({ flagKey }: { flagKey: string }) {
  const value = useFlag(flagKey, false);
  useEffect(() => {
    window.__ds_observations.push({ flagKey, value, ts: Date.now() });
  }, [flagKey, value]);
  return null;
}

function App() {
  const cfg = parseQuery();
  return (
    <DeploySentryProvider
      apiUrl={cfg.apiUrl}
      apiKey={cfg.apiKey}
      project={cfg.project}
      environment={cfg.environment}
      context={cfg.context}
    >
      {cfg.flagKeys.map((k) => (
        <Observer key={k} flagKey={k} />
      ))}
    </DeploySentryProvider>
  );
}

createRoot(document.getElementById('root')!).render(<App />);
```

- [ ] **Step 6: Build the harness to verify it compiles**

```bash
cd web/e2e/sdk-probes/react-harness
npm install
npm run build
ls dist/index.html
```

Expected: `dist/index.html` exists, no build errors. If the `@deploysentry/react` package's `DeploySentryProvider` has different prop names, update `src/main.tsx` to match before rerunning.

- [ ] **Step 7: Commit**

```bash
git add web/e2e/sdk-probes/react-harness/
git commit -m "feat(e2e): add React SDK probe harness"
```

---

## Task 6: `sdk-driver.ts` helper

**Files:**
- Create: `web/e2e/helpers/sdk-driver.ts`

**Responsibility:** A single TypeScript module exposing `startNodeProbe`, `startReactProbe`, and `waitForValue` with a uniform `Probe` interface, so specs don't branch on SDK runtime.

- [ ] **Step 1: Write `web/e2e/helpers/sdk-driver.ts`**

```typescript
import { spawn, type ChildProcess } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { type Browser, type BrowserContext, type Page } from '@playwright/test';

export interface Observation {
  flagKey: string;
  value: unknown;
  ts: number;
}

export interface ProbeContext {
  apiUrl: string;
  apiKey: string;
  project: string;
  environment: string;
  flagKeys: string[];
  context: Record<string, unknown>;
}

export interface Probe {
  name: string;
  observations(): Observation[];
  stop(): Promise<void>;
}

export async function startNodeProbe(ctx: ProbeContext): Promise<Probe> {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ds-node-probe-'));
  const obsFile = path.join(dir, 'observations.jsonl');
  fs.writeFileSync(obsFile, '');
  const probeDir = path.resolve(__dirname, '../sdk-probes/node-probe');
  const child: ChildProcess = spawn('node', ['index.js'], {
    cwd: probeDir,
    env: {
      ...process.env,
      OBSERVATIONS_FILE: obsFile,
      DS_API_URL: ctx.apiUrl,
      DS_API_KEY: ctx.apiKey,
      DS_PROJECT: ctx.project,
      DS_ENVIRONMENT: ctx.environment,
      DS_FLAG_KEYS: ctx.flagKeys.join(','),
      DS_CONTEXT_JSON: JSON.stringify(ctx.context),
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  child.stderr?.on('data', (b) => process.stderr.write(`[node-probe] ${b}`));

  return {
    name: 'node',
    observations() {
      const raw = fs.readFileSync(obsFile, 'utf8');
      return raw
        .split('\n')
        .filter(Boolean)
        .map((line) => JSON.parse(line) as Observation);
    },
    async stop() {
      child.kill('SIGTERM');
      await new Promise((r) => child.once('exit', r));
      fs.rmSync(dir, { recursive: true, force: true });
    },
  };
}

export async function startReactProbe(
  browser: Browser,
  harnessBaseUrl: string,
  ctx: ProbeContext,
): Promise<Probe & { page: Page; browserContext: BrowserContext }> {
  const bctx = await browser.newContext();
  const page = await bctx.newPage();
  const qs = new URLSearchParams({
    apiUrl: ctx.apiUrl,
    apiKey: ctx.apiKey,
    project: ctx.project,
    environment: ctx.environment,
    flagKeys: ctx.flagKeys.join(','),
    context: JSON.stringify(ctx.context),
  }).toString();
  await page.goto(`${harnessBaseUrl}/?${qs}`);

  return {
    name: 'react',
    page,
    browserContext: bctx,
    observations() {
      throw new Error('Use observationsAsync() on the react probe');
    },
    async stop() {
      await bctx.close();
    },
  } as Probe & {
    page: Page;
    browserContext: BrowserContext;
    observationsAsync(): Promise<Observation[]>;
  };
}

export async function reactObservations(probe: Probe & { page: Page }): Promise<Observation[]> {
  return probe.page.evaluate(() => (window as any).__ds_observations ?? []);
}

export async function waitForValue(
  probe: Probe,
  flagKey: string,
  expected: unknown,
  opts: { timeoutMs?: number; getObservations?: () => Promise<Observation[]> } = {},
): Promise<number> {
  const timeoutMs = opts.timeoutMs ?? 3_000;
  const deadline = Date.now() + timeoutMs;
  const start = Date.now();
  while (Date.now() < deadline) {
    const observed = opts.getObservations
      ? await opts.getObservations()
      : probe.observations();
    const match = observed.find(
      (o) => o.flagKey === flagKey && JSON.stringify(o.value) === JSON.stringify(expected),
    );
    if (match) return Date.now() - start;
    await new Promise((r) => setTimeout(r, 25));
  }
  const dump = opts.getObservations ? await opts.getObservations() : probe.observations();
  throw new Error(
    `probe="${probe.name}" flag="${flagKey}" never observed value=${JSON.stringify(expected)} ` +
    `within ${timeoutMs}ms. observations: ${JSON.stringify(dump, null, 2)}`,
  );
}
```

Note the two-path observations API: Node reads synchronously from a file, React reads asynchronously via `page.evaluate`. The `waitForValue` helper accepts a `getObservations` callback so the React path can pass `() => reactObservations(probe)`.

- [ ] **Step 2: Type-check the helper**

```bash
cd web && npx tsc --noEmit -p tsconfig.json
```

Expected: passes. If it reports missing types for `node:child_process` etc., add `@types/node` to `web/package.json` devDependencies and rerun.

- [ ] **Step 3: Commit**

```bash
git add web/e2e/helpers/sdk-driver.ts web/package.json web/package-lock.json
git commit -m "feat(e2e): add SDK probe driver helper"
```

---

## Task 7: Spec scaffolding with `beforeAll` seeding

**Files:**
- Create: `web/e2e/sdk/flag-delivery.spec.ts`
- Create: `web/e2e/helpers/seed-via-ui.ts`

**Responsibility:** Seed an org/project/app and capture an API key **by driving the dashboard UI** (using the existing UI helpers in `web/e2e/helpers`), then expose that context to every scenario. This is the UI leg of the chain: if the dashboard breaks, `beforeAll` fails and nothing else runs.

- [ ] **Step 1: Inspect existing UI seeding helpers**

Run: `ls web/e2e/helpers && grep -l "login\|register\|createOrg" web/e2e/helpers/*.ts`
Expected: lists the helpers created in the existing Playwright work. Reuse `auth.ts` if it exists; otherwise the seed helper below must handle login directly.

- [ ] **Step 2: Write `web/e2e/helpers/seed-via-ui.ts`**

```typescript
import { type Page, expect } from '@playwright/test';

export interface SeededContext {
  orgSlug: string;
  projectSlug: string;
  appSlug: string;
  environment: string;
  apiKey: string;
  apiUrl: string;
}

export async function seedOrgProjectAppViaUI(page: Page): Promise<SeededContext> {
  const apiUrl = process.env.DS_E2E_API_URL ?? 'http://localhost:18080';
  const suffix = Date.now().toString(36);
  const email = `e2e-${suffix}@deploysentry.test`;
  const password = 'Passw0rd!';
  const orgSlug = `e2e-org-${suffix}`;
  const projectSlug = `e2e-proj-${suffix}`;
  const appSlug = `e2e-app-${suffix}`;
  const environment = 'development';

  await page.goto('/register');
  await page.getByLabel(/email/i).fill(email);
  await page.getByLabel(/password/i).first().fill(password);
  await page.getByRole('button', { name: /register|sign up/i }).click();
  await expect(page).toHaveURL(/\/orgs\/new|\/onboard/);

  await page.getByLabel(/organization name/i).fill(`E2E Org ${suffix}`);
  await page.getByLabel(/slug/i).fill(orgSlug);
  await page.getByRole('button', { name: /create org/i }).click();

  await page.getByRole('link', { name: /new project/i }).click();
  await page.getByLabel(/project name/i).fill(`E2E Project ${suffix}`);
  await page.getByLabel(/slug/i).fill(projectSlug);
  await page.getByRole('button', { name: /create project/i }).click();

  await page.getByRole('link', { name: /new application/i }).click();
  await page.getByLabel(/application name/i).fill(`E2E App ${suffix}`);
  await page.getByLabel(/slug/i).fill(appSlug);
  await page.getByRole('button', { name: /create app/i }).click();

  await page.getByRole('link', { name: /api keys/i }).click();
  await page.getByRole('button', { name: /create key/i }).click();
  await page.getByLabel(/name/i).fill('e2e-sdk');
  await page.getByLabel(/environment/i).selectOption(environment);
  await page.getByRole('button', { name: /create/i }).click();
  const apiKey = (await page.getByTestId('api-key-value').textContent())?.trim() ?? '';
  if (!apiKey) throw new Error('failed to capture API key from UI');

  return { orgSlug, projectSlug, appSlug, environment, apiKey, apiUrl };
}
```

Note: the exact selectors above are best-guesses based on the existing smoke specs. During implementation, open `web/e2e/smoke/02-org-setup.spec.ts` and `web/e2e/smoke/05-api-keys.spec.ts` and copy their selector patterns verbatim — the goal is to reuse known-working selectors, not reinvent them.

- [ ] **Step 3: Write the spec scaffold**

Create `web/e2e/sdk/flag-delivery.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';
import {
  startNodeProbe,
  startReactProbe,
  reactObservations,
  waitForValue,
  type Probe,
} from '../helpers/sdk-driver';
import { seedOrgProjectAppViaUI, type SeededContext } from '../helpers/seed-via-ui';

const HARNESS_URL = process.env.DS_E2E_REACT_HARNESS_URL ?? 'http://localhost:4310';

let seeded: SeededContext;

test.beforeAll(async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  seeded = await seedOrgProjectAppViaUI(page);
  await ctx.close();
});

test('environment seeded', () => {
  expect(seeded.apiKey).toBeTruthy();
  expect(seeded.orgSlug).toMatch(/^e2e-org-/);
});
```

This tiny first test proves the seeding flow works before any scenario touches SDKs.

- [ ] **Step 4: Add a web server entry for the React harness to `playwright.config.ts`**

In the existing `webServer` field, replace the single-object value with an array so both the dashboard dev server and the harness preview are started:

```typescript
  webServer: [
    { command: 'npm run dev', port: 3001, reuseExistingServer: true },
    {
      command: 'npm run --prefix e2e/sdk-probes/react-harness preview',
      port: 4310,
      reuseExistingServer: true,
      cwd: '.',
    },
  ],
```

Note: `npm run preview` requires the harness to have been built (Task 5 step 6). Add a `pretest` step in the Makefile target to build it:

```make
e2e-sdk: e2e-sdk-up
	npm --prefix web/e2e/sdk-probes/react-harness install
	npm --prefix web/e2e/sdk-probes/react-harness run build
	cd web && npx playwright test --project=sdk
	$(MAKE) e2e-sdk-down
```

- [ ] **Step 5: Run the scaffold**

```bash
make e2e-sdk-up
cd web && npx playwright test --project=sdk
```

Expected: one test passes (`environment seeded`). If seeding fails, fix selectors in `seed-via-ui.ts` by copying from the existing smoke specs.

- [ ] **Step 6: Commit**

```bash
make e2e-sdk-down
git add web/e2e/sdk/flag-delivery.spec.ts web/e2e/helpers/seed-via-ui.ts web/playwright.config.ts Makefile
git commit -m "feat(e2e): scaffold flag-delivery spec with UI seeding"
```

---

## Task 8: Scenario A — baseline propagation

**Files:**
- Modify: `web/e2e/sdk/flag-delivery.spec.ts`
- Create: `web/e2e/helpers/flag-ui.ts`

**Responsibility:** Drive the dashboard to create a boolean flag `e2e-baseline-{suffix}` defaulting to `false`, start Node + React probes, assert both observe `false`, toggle the flag to `true` via the UI, assert both observe `true` within 2s.

- [ ] **Step 1: Write `web/e2e/helpers/flag-ui.ts` with `createBooleanFlag` and `toggleFlag` helpers**

```typescript
import { type Page } from '@playwright/test';
import { type SeededContext } from './seed-via-ui';

export async function createBooleanFlag(
  page: Page,
  seeded: SeededContext,
  flagKey: string,
  defaultValue: boolean,
): Promise<void> {
  await page.goto(`/orgs/${seeded.orgSlug}/projects/${seeded.projectSlug}/flags/new`);
  await page.getByLabel(/flag key/i).fill(flagKey);
  await page.getByLabel(/category/i).selectOption('ops');
  await page.getByLabel(/type/i).selectOption('boolean');
  await page
    .getByLabel(/default value/i)
    .selectOption(defaultValue ? 'true' : 'false');
  await page.getByRole('button', { name: /create flag/i }).click();
  await page.waitForURL(new RegExp(`/flags/${flagKey}`));
}

export async function toggleFlag(
  page: Page,
  seeded: SeededContext,
  flagKey: string,
  enabled: boolean,
): Promise<number> {
  await page.goto(`/orgs/${seeded.orgSlug}/projects/${seeded.projectSlug}/flags/${flagKey}`);
  const toggle = page.getByRole('switch', { name: /enabled/i });
  const before = performance.now();
  if (enabled) {
    if (!(await toggle.isChecked())) await toggle.click();
  } else {
    if (await toggle.isChecked()) await toggle.click();
  }
  await page.getByText(/saved|updated/i).waitFor({ state: 'visible', timeout: 3_000 });
  return before;
}
```

Again, copy exact selectors from the existing smoke specs' flag-lifecycle tests rather than guessing.

- [ ] **Step 2: Add scenario A to `flag-delivery.spec.ts`**

Append to the spec file after the scaffolding test:

```typescript
import { createBooleanFlag, toggleFlag } from '../helpers/flag-ui';

test('Scenario A: baseline propagation — Node + React observe toggle within 2s', async ({
  browser,
  page,
}) => {
  const flagKey = `e2e-baseline-${Date.now().toString(36)}`;
  await createBooleanFlag(page, seeded, flagKey, false);

  const probeCtx = {
    apiUrl: seeded.apiUrl,
    apiKey: seeded.apiKey,
    project: seeded.projectSlug,
    environment: seeded.environment,
    flagKeys: [flagKey],
    context: { userId: 'u1' },
  };
  const nodeProbe = await startNodeProbe(probeCtx);
  const reactProbe = await startReactProbe(browser, HARNESS_URL, probeCtx);

  try {
    await waitForValue(nodeProbe, flagKey, false, { timeoutMs: 3_000 });
    await waitForValue(reactProbe, flagKey, false, {
      timeoutMs: 3_000,
      getObservations: () => reactObservations(reactProbe as any),
    });

    const clickAt = await toggleFlag(page, seeded, flagKey, true);

    const nodeLatency = await waitForValue(nodeProbe, flagKey, true, { timeoutMs: 3_000 });
    const reactLatency = await waitForValue(reactProbe, flagKey, true, {
      timeoutMs: 3_000,
      getObservations: () => reactObservations(reactProbe as any),
    });

    console.log(
      `scenario-A latency: node=${nodeLatency}ms react=${reactLatency}ms clickAt=${clickAt.toFixed(0)}`,
    );
    expect(nodeLatency).toBeLessThan(2_000);
    expect(reactLatency).toBeLessThan(2_000);
  } finally {
    await nodeProbe.stop();
    await reactProbe.stop();
  }
});
```

- [ ] **Step 3: Run the scenario**

```bash
make e2e-sdk-up
cd web && npx playwright test --project=sdk -g "Scenario A"
```

Expected: passes. If it fails on the baseline observation, the probe isn't connecting — check env vars. If it fails on the toggle observation, either the UI toggle isn't saving (inspect trace) or SSE isn't fanning out (check api logs via `docker compose -f ../docker-compose.e2e.yml logs api`).

- [ ] **Step 4: Commit**

```bash
make e2e-sdk-down
git add web/e2e/sdk/flag-delivery.spec.ts web/e2e/helpers/flag-ui.ts
git commit -m "feat(e2e): add scenario A baseline propagation test"
```

---

## Task 9: Scenario B — targeting correctness

**Files:**
- Modify: `web/e2e/sdk/flag-delivery.spec.ts`
- Modify: `web/e2e/helpers/flag-ui.ts`

**Responsibility:** Create a flag with a rule `user.plan == "pro" → true`, run two Node probes with different contexts, assert the `pro` probe sees `true` and the `free` probe sees `false`, then edit the rule to `user.plan == "enterprise"` and assert both now see `false`.

- [ ] **Step 1: Add `addEqualsRule` and `updateEqualsRule` to `flag-ui.ts`**

```typescript
export async function addEqualsRule(
  page: Page,
  seeded: SeededContext,
  flagKey: string,
  attribute: string,
  value: string,
  variant: boolean,
): Promise<void> {
  await page.goto(`/orgs/${seeded.orgSlug}/projects/${seeded.projectSlug}/flags/${flagKey}`);
  await page.getByRole('button', { name: /add rule/i }).click();
  await page.getByLabel(/attribute/i).fill(attribute);
  await page.getByLabel(/operator/i).selectOption('equals');
  await page.getByLabel(/value/i).fill(value);
  await page.getByLabel(/then serve/i).selectOption(variant ? 'true' : 'false');
  await page.getByRole('button', { name: /save rule/i }).click();
  await page.getByText(/saved|updated/i).waitFor({ state: 'visible', timeout: 3_000 });
}

export async function updateFirstRuleValue(
  page: Page,
  seeded: SeededContext,
  flagKey: string,
  newValue: string,
): Promise<number> {
  await page.goto(`/orgs/${seeded.orgSlug}/projects/${seeded.projectSlug}/flags/${flagKey}`);
  await page.getByRole('button', { name: /edit rule/i }).first().click();
  await page.getByLabel(/value/i).fill(newValue);
  const t = performance.now();
  await page.getByRole('button', { name: /save rule/i }).click();
  await page.getByText(/saved|updated/i).waitFor({ state: 'visible', timeout: 3_000 });
  return t;
}
```

- [ ] **Step 2: Add scenario B to the spec**

```typescript
import { addEqualsRule, updateFirstRuleValue } from '../helpers/flag-ui';

test('Scenario B: targeting correctness — two Node probes see different values', async ({
  page,
}) => {
  const flagKey = `e2e-targeting-${Date.now().toString(36)}`;
  await createBooleanFlag(page, seeded, flagKey, false);
  await addEqualsRule(page, seeded, flagKey, 'user.plan', 'pro', true);

  const base = {
    apiUrl: seeded.apiUrl,
    apiKey: seeded.apiKey,
    project: seeded.projectSlug,
    environment: seeded.environment,
    flagKeys: [flagKey],
  };
  const freeProbe = await startNodeProbe({
    ...base,
    context: { userId: 'u1', plan: 'free' },
  });
  const proProbe = await startNodeProbe({
    ...base,
    context: { userId: 'u2', plan: 'pro' },
  });

  try {
    await waitForValue(freeProbe, flagKey, false, { timeoutMs: 3_000 });
    await waitForValue(proProbe, flagKey, true, { timeoutMs: 3_000 });

    await updateFirstRuleValue(page, seeded, flagKey, 'enterprise');

    await waitForValue(freeProbe, flagKey, false, { timeoutMs: 3_000 });
    await waitForValue(proProbe, flagKey, false, { timeoutMs: 3_000 });
  } finally {
    await freeProbe.stop();
    await proProbe.stop();
  }
});
```

- [ ] **Step 3: Run the scenario**

```bash
make e2e-sdk-up
cd web && npx playwright test --project=sdk -g "Scenario B"
```

Expected: passes. If the `pro` probe never sees `true`, targeting isn't evaluating — inspect the API's evaluation response directly: `curl -H "Authorization: Bearer $KEY" http://localhost:18080/api/v1/flags/evaluate -d '{"key":"...","context":{"plan":"pro"}}'`.

- [ ] **Step 4: Commit**

```bash
make e2e-sdk-down
git add web/e2e/sdk/flag-delivery.spec.ts web/e2e/helpers/flag-ui.ts
git commit -m "feat(e2e): add scenario B targeting correctness test"
```

---

## Task 10: Scenario C — variant delivery

**Files:**
- Modify: `web/e2e/sdk/flag-delivery.spec.ts`
- Modify: `web/e2e/helpers/flag-ui.ts`
- Modify: `web/e2e/sdk-probes/node-probe/index.js`
- Modify: `web/e2e/sdk-probes/react-harness/src/main.tsx`

**Responsibility:** Create a multivariate flag with `control|treatment` variants and a 100/0 split, assert both probes see `control`, change the split to 0/100, assert both see `treatment`.

- [ ] **Step 1: Extend the Node probe to call `getVariant`/`getString` instead of `isEnabled` when the flag key has the `variant:` prefix**

Modify `web/e2e/sdk-probes/node-probe/index.js` inside `tick()`:

```javascript
async function tick() {
  for (const key of flagKeys) {
    try {
      let value;
      if (key.startsWith('variant:')) {
        const real = key.slice('variant:'.length);
        value = await client.getString(real, 'control', context);
        record(key, value);
      } else {
        value = await client.isEnabled(key, false, context);
        record(key, value);
      }
    } catch (err) {
      fs.appendFileSync(
        OBSERVATIONS_FILE,
        JSON.stringify({ flagKey: key, error: String(err), ts: Date.now() }) + '\n'
      );
    }
  }
}
```

- [ ] **Step 2: Extend the React harness similarly**

In `src/main.tsx`, replace the `Observer` component with:

```typescript
function Observer({ flagKey }: { flagKey: string }) {
  const isVariant = flagKey.startsWith('variant:');
  const realKey = isVariant ? flagKey.slice('variant:'.length) : flagKey;
  const value = useFlag<string | boolean>(realKey, isVariant ? 'control' : false);
  useEffect(() => {
    window.__ds_observations.push({ flagKey, value, ts: Date.now() });
  }, [flagKey, value]);
  return null;
}
```

- [ ] **Step 3: Add `createVariantFlag` and `setVariantSplit` to `flag-ui.ts`**

```typescript
export async function createVariantFlag(
  page: Page,
  seeded: SeededContext,
  flagKey: string,
  variants: string[],
  initialVariant: string,
): Promise<void> {
  await page.goto(`/orgs/${seeded.orgSlug}/projects/${seeded.projectSlug}/flags/new`);
  await page.getByLabel(/flag key/i).fill(flagKey);
  await page.getByLabel(/category/i).selectOption('experiment');
  await page.getByLabel(/type/i).selectOption('variant');
  for (const v of variants) {
    await page.getByRole('button', { name: /add variant/i }).click();
    await page.getByLabel(/variant name/i).last().fill(v);
  }
  await page.getByLabel(/default variant/i).selectOption(initialVariant);
  await page.getByRole('button', { name: /create flag/i }).click();
  await page.waitForURL(new RegExp(`/flags/${flagKey}`));
}

export async function setDefaultVariant(
  page: Page,
  seeded: SeededContext,
  flagKey: string,
  variant: string,
): Promise<void> {
  await page.goto(`/orgs/${seeded.orgSlug}/projects/${seeded.projectSlug}/flags/${flagKey}`);
  await page.getByLabel(/default variant/i).selectOption(variant);
  await page.getByRole('button', { name: /save/i }).click();
  await page.getByText(/saved|updated/i).waitFor({ state: 'visible', timeout: 3_000 });
}
```

Note: if the dashboard represents variant splits as percentages rather than a single "default variant" selector, replace `setDefaultVariant` with a helper that sets `control=0, treatment=100`. Inspect the UI in development mode before implementing.

- [ ] **Step 4: Add scenario C to the spec**

```typescript
import { createVariantFlag, setDefaultVariant } from '../helpers/flag-ui';

test('Scenario C: variant delivery — Node + React observe variant change', async ({
  browser,
  page,
}) => {
  const realKey = `e2e-variant-${Date.now().toString(36)}`;
  const probeKey = `variant:${realKey}`;
  await createVariantFlag(page, seeded, realKey, ['control', 'treatment'], 'control');

  const probeCtx = {
    apiUrl: seeded.apiUrl,
    apiKey: seeded.apiKey,
    project: seeded.projectSlug,
    environment: seeded.environment,
    flagKeys: [probeKey],
    context: { userId: 'u1' },
  };
  const nodeProbe = await startNodeProbe(probeCtx);
  const reactProbe = await startReactProbe(browser, HARNESS_URL, probeCtx);

  try {
    await waitForValue(nodeProbe, probeKey, 'control', { timeoutMs: 3_000 });
    await waitForValue(reactProbe, probeKey, 'control', {
      timeoutMs: 3_000,
      getObservations: () => reactObservations(reactProbe as any),
    });

    await setDefaultVariant(page, seeded, realKey, 'treatment');

    await waitForValue(nodeProbe, probeKey, 'treatment', { timeoutMs: 3_000 });
    await waitForValue(reactProbe, probeKey, 'treatment', {
      timeoutMs: 3_000,
      getObservations: () => reactObservations(reactProbe as any),
    });
  } finally {
    await nodeProbe.stop();
    await reactProbe.stop();
  }
});
```

- [ ] **Step 5: Run the scenario**

```bash
make e2e-sdk-up
cd web && npx playwright test --project=sdk -g "Scenario C"
```

Expected: passes.

- [ ] **Step 6: Run all three scenarios together**

```bash
cd web && npx playwright test --project=sdk
make e2e-sdk-down
```

Expected: all tests pass. Note observed latencies in the console output for the calibration step in Task 12.

- [ ] **Step 7: Commit**

```bash
git add web/e2e/sdk/flag-delivery.spec.ts web/e2e/helpers/flag-ui.ts web/e2e/sdk-probes/
git commit -m "feat(e2e): add scenario C variant delivery test"
```

---

## Task 11: Per-PR CI workflow

**Files:**
- Create: `.github/workflows/e2e-sdk.yml`

**Responsibility:** Run `make e2e-sdk` on PRs that touch the relevant paths, upload artifacts on failure.

- [ ] **Step 1: Write the workflow**

```yaml
name: e2e-sdk

on:
  pull_request:
    paths:
      - 'sdk/node/**'
      - 'sdk/react/**'
      - 'internal/flags/**'
      - 'cmd/api/**'
      - 'web/e2e/sdk/**'
      - 'web/e2e/sdk-probes/**'
      - 'web/e2e/helpers/sdk-driver.ts'
      - 'web/e2e/helpers/seed-via-ui.ts'
      - 'web/e2e/helpers/flag-ui.ts'
      - 'docker-compose.e2e.yml'
      - 'deploy/e2e/**'
      - '.github/workflows/e2e-sdk.yml'

jobs:
  e2e-sdk:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }

      - uses: actions/setup-node@v4
        with: { node-version: '20' }

      - name: Install migrate CLI
        run: |
          curl -fsSL https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xz
          sudo mv migrate /usr/local/bin/

      - name: Install web deps
        run: cd web && npm ci

      - name: Install Playwright browsers
        run: cd web && npx playwright install --with-deps chromium

      - name: Install node-probe deps
        run: cd web/e2e/sdk-probes/node-probe && npm install

      - name: Build react-harness
        run: |
          cd web/e2e/sdk-probes/react-harness
          npm install
          npm run build

      - name: Run e2e-sdk
        run: make e2e-sdk

      - name: Upload Playwright artifacts on failure
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-sdk-report
          path: |
            web/playwright-report/
            web/test-results/

      - name: Dump compose logs on failure
        if: failure()
        run: docker compose -f docker-compose.e2e.yml logs --no-color > compose-logs.txt || true

      - name: Upload compose logs on failure
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: compose-logs
          path: compose-logs.txt
```

- [ ] **Step 2: Push the branch and open a draft PR to verify the workflow runs**

```bash
git add .github/workflows/e2e-sdk.yml
git commit -m "ci: add e2e-sdk per-PR workflow"
```

(The workflow runs on PR; verify in the GitHub UI that it triggers and completes green.)

---

## Task 12: Latency calibration + soak

**Files:**
- Create: `docs/superpowers/plans/2026-04-12-e2e-sdk-flag-delivery-calibration.md`

**Responsibility:** Measure p95 SSE propagation latency with scenario A running 50× in the compose stack, set the `waitForValue` ceiling to `max(1000, 2 * p95)`, and record the measurement. Run the full suite 20× on `main` to verify zero flakes before marking the job required.

- [ ] **Step 1: Run scenario A 50 times and collect latencies**

```bash
make e2e-sdk-up
for i in $(seq 1 50); do
  cd web && npx playwright test --project=sdk -g "Scenario A" --reporter=line 2>&1 \
    | grep "scenario-A latency"
  cd ..
done | tee /tmp/scenario-a-latencies.txt
make e2e-sdk-down
```

Expected: 50 lines of `scenario-A latency: node=Xms react=Yms ...`. Feed them through a quick percentile calc:

```bash
awk -F'[= ]' '{print $3}' /tmp/scenario-a-latencies.txt | sort -n | awk '{a[NR]=$1} END {print "p95="a[int(NR*0.95)]}'
awk -F'[= ]' '{print $5}' /tmp/scenario-a-latencies.txt | sort -n | awk '{a[NR]=$1} END {print "p95="a[int(NR*0.95)]}'
```

- [ ] **Step 2: Adjust latency ceilings in `flag-delivery.spec.ts`**

If measured node p95 < 500 and react p95 < 500, keep the 2000ms ceiling. Otherwise set each to `max(1000, 2 * p95)`. Update the two `expect(...).toBeLessThan(2_000)` lines in scenario A.

- [ ] **Step 3: Soak: run the full `sdk` project 20 times on an unchanged `main`**

```bash
for i in $(seq 1 20); do
  make e2e-sdk || { echo "run $i failed"; exit 1; }
done
```

Expected: all 20 runs green. If any run flakes, root-cause it (inspect artifact bundle, compose logs) — do not raise timeouts.

- [ ] **Step 4: Record results in `docs/superpowers/plans/2026-04-12-e2e-sdk-flag-delivery-calibration.md`**

```markdown
# e2e-sdk calibration record

Date: <fill in>

## Latency measurements (scenario A, n=50)

| Probe | p50 (ms) | p95 (ms) | p99 (ms) |
|---|---|---|---|
| Node |  |  |  |
| React |  |  |  |

Ceiling set to: <value>

## Soak results (n=20)

Pass: X/20. Failures (if any): <links to artifacts>

## Fault injection validation

| Fault | Scenario expected to fail | Actual result |
|---|---|---|
| Stop sentinel mid-test | A | |
| API no-op flag update | A | |
| Node SDK drops SSE events | A (Node only) | |
| Targeting ignores `plan` | B | |
```

- [ ] **Step 5: Perform the four fault injections manually and fill in the table**

For each:
1. Create a throwaway local diff that introduces the fault.
2. Run `make e2e-sdk`.
3. Confirm the expected scenario fails with an attributable error.
4. Revert the diff.
5. Record the result in the table.

This step is not committed — it's a throwaway validation — but the table in the calibration doc *is* committed.

- [ ] **Step 6: Commit the calibration record**

```bash
git add docs/superpowers/plans/2026-04-12-e2e-sdk-flag-delivery-calibration.md web/e2e/sdk/flag-delivery.spec.ts
git commit -m "test(e2e): calibrate latency ceilings and record soak results"
```

---

## Task 13: Mark the job required and update docs

**Files:**
- Modify: `docs/Current_Initiatives.md`
- Modify: `docs/superpowers/specs/2026-04-12-e2e-sdk-flag-delivery-design.md` (update phase)
- Modify: branch protection on `main` (manual via GitHub UI)

- [ ] **Step 1: Mark `e2e-sdk` as a required check on `main`**

Via GitHub settings → Branches → `main` → Require status checks → add `e2e-sdk`. (Manual step — no file change.)

- [ ] **Step 2: Update `docs/Current_Initiatives.md`**

Add a row pointing to this plan with phase `Complete` (or remove if the project template moves completed items elsewhere — follow the convention already in the file).

- [ ] **Step 3: Update the spec's phase to Complete and fill in the Completion Record**

- [ ] **Step 4: Commit**

```bash
git add docs/Current_Initiatives.md docs/superpowers/specs/2026-04-12-e2e-sdk-flag-delivery-design.md
git commit -m "docs: mark e2e-sdk initiative complete"
```

---

## Out of Scope (Follow-up Plans)

- **Flutter probe + nightly workflow** — Task deferred to a follow-up plan because Flutter toolchain setup doubles CI time and should not gate per-PR merges. The follow-up will add `flutter-probe/`, `RUN_FLUTTER_PROBE=1` handling in `sdk-driver.ts`, and `.github/workflows/e2e-sdk-nightly.yml`.
- **Reconnection / network-partition resilience** — out of scope per spec non-goals.
- **Go / Python / Java / Ruby SDK coverage** — contract tests in `sdk/*/` cover those.

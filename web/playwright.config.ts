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
    {
      name: 'sdk-setup',
      testDir: './e2e/sdk',
      testMatch: /global-setup\.ts/,
    },
    {
      name: 'sdk',
      testDir: './e2e/sdk',
      testIgnore: /global-setup\.ts/,
      dependencies: ['sdk-setup'],
      timeout: 90_000,
      use: {
        baseURL: 'http://localhost:13002',
        browserName: 'chromium',
        trace: 'retain-on-failure',
        screenshot: 'only-on-failure',
      },
      fullyParallel: false,
      workers: 1,
      // Hermetic e2e API has a 100 req/min rate limiter; retries blow it
      // out and create cascading failures. Better to fail fast.
      retries: 0,
    },
  ],

  // Two Vite dev servers run side-by-side so smoke/ui and sdk projects can
  // each talk to their own backend. Playwright starts every entry on every
  // `playwright test` invocation; the :3002 instance is lightweight enough
  // that always-on is simpler than gating via env var.
  webServer: [
    {
      command: 'npm run dev',
      port: 3001,
      reuseExistingServer: true,
      timeout: 60_000,
    },
    {
      // Hermetic SDK suite. Uses a port in the e2e 1xxxx range to avoid
      // colliding with developer-machine services (e.g. VNC) on 3xxx.
      command: 'VITE_API_PROXY_TARGET=http://localhost:18080 VITE_DEV_PORT=13002 npm run dev',
      port: 13002,
      reuseExistingServer: true,
      timeout: 60_000,
    },
    {
      // Build the React harness before previewing — vite preview serves
      // dist/ which doesn't exist on a fresh checkout.
      command:
        'npm install --silent --prefix e2e/sdk-probes/react-harness && ' +
        'npm run --prefix e2e/sdk-probes/react-harness build && ' +
        'npm run --prefix e2e/sdk-probes/react-harness preview',
      port: 4310,
      reuseExistingServer: true,
      timeout: 90_000,
    },
  ],
});

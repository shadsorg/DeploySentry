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

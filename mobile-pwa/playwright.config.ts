import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: process.env.MOBILE_PWA_BASE_URL ?? 'http://localhost:3002',
    trace: 'on-first-retry',
  },
  projects: [{ name: 'iphone-13', use: { ...devices['iPhone 13'] } }],
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:3002/m/',
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});

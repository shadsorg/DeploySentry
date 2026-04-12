import { test, request } from '@playwright/test';

const API_URL = process.env.DS_E2E_API_URL ?? 'http://localhost:18080';

test('e2e stack is healthy', async () => {
  const ctx = await request.newContext();
  try {
    const res = await ctx.get(`${API_URL}/ready`);
    if (!res.ok()) {
      throw new Error(
        `e2e stack not healthy at ${API_URL}/ready (status ${res.status()}). ` +
        `Run 'make e2e-sdk-up' before 'npx playwright test --project=sdk'.`
      );
    }
  } finally {
    await ctx.dispose();
  }
});

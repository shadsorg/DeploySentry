// import { type Page, type APIRequestContext } from '@playwright/test';
import { createHmac, randomUUID } from 'crypto';

/**
 * SeededContext holds the identifiers and credentials required by SDK probes
 * to authenticate with and subscribe to the DeploySentry flag delivery stream.
 *
 * The SDK SSE stream endpoint requires UUIDs (project_id / environment_id),
 * while the dashboard UI routes by slug. This context carries both so later
 * test scenarios can drive the UI to toggle flags while SDK probes subscribe
 * via the streaming endpoint.
 */
export interface SeededContext {
  // Slugs (used for UI navigation in later tasks)
  orgSlug: string;
  projectSlug: string;
  appSlug: string;

  // UUIDs (used by SDK probes for the flag stream)
  orgId: string;
  projectId: string;
  appId: string;
  environmentId: string;

  // Probe configuration
  environment: string; // environment slug, e.g. "development"
  apiKey: string; // plaintext key (ds_...)
  apiUrl: string;

  // User credentials (for relogin in later scenarios if needed)
  email: string;
  password: string;
  name: string;
}

interface AuthResponse {
  token: string;
  user: { id: string; email: string; name: string };
}

/**
 * seedOrgProjectAppViaUI creates a fresh user, org, project, app, environment,
 * and API key suitable for driving SDK probe scenarios against the e2e stack.
 *
 * Strategy (hybrid, leaning API-first):
 *
 *   All entity creation (register, org, project, app, environment, api key)
 *   is done via direct HTTP calls against the hermetic e2e API on :18080.
 *   The dashboard UI creation flows are already covered by the smoke tests
 *   (02-org-setup, 05-api-keys, 03-flag-lifecycle), so this helper focuses
 *   on speed and reliability over re-exercising those flows.
 *
 *   Importantly, we CANNOT use the dashboard UI to register here because
 *   the Vite dev server proxies `/api` to http://localhost:8080 (the dev
 *   API), while the hermetic e2e stack runs on :18080. Driving the UI
 *   would register the user against the wrong backend. See vite.config.ts.
 *
 *   The "UI-driven" aspect of the overall SDK test plan is satisfied in
 *   later tasks (flag toggles through the dashboard); Task 7 only needs
 *   to produce a correct (slugs + UUIDs + API key) context.
 *
 *   `page` is still accepted for parity with later helpers that may add UI
 *   interactions (e.g. first-login consent flows). It's currently unused.
 */
export async function seedOrgProjectAppViaUI(
  // _page: Page,
  // _apiRequest: APIRequestContext,
): Promise<SeededContext> {
  const apiUrl = process.env.DS_E2E_API_URL ?? 'http://localhost:18080';
  const apiBase = `${apiUrl}/api/v1`;

  const suffix = Date.now().toString(36) + Math.random().toString(36).slice(2, 6);
  const email = `e2e-${suffix}@deploysentry.test`;
  const password = 'Passw0rd!2026';
  const name = `E2E User ${suffix}`;
  const orgName = `E2E Org ${suffix}`;
  const projectName = `E2E Project ${suffix}`;
  const appName = `E2E App ${suffix}`;
  const orgSlug = `e2e-org-${suffix}`;
  const projectSlug = `e2e-proj-${suffix}`;
  const appSlug = `e2e-app-${suffix}`;
  const environment = 'development';

  // ---------------------------------------------------------------------------
  // Step 1: Register the user directly against the e2e stack at :18080.
  // We cannot reuse helpers/api-client.ts because it hardcodes :8080.
  // ---------------------------------------------------------------------------
  const token = await registerForToken(apiBase, email, password, name);

  // ---------------------------------------------------------------------------
  // Step 3: Create org, project, app, environment, and API key via API.
  // ---------------------------------------------------------------------------
  const headers = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  } as const;

  const org = await postJson<{ id: string; slug: string }>(
    `${apiBase}/orgs`,
    headers,
    { name: orgName, slug: orgSlug },
  );

  const project = await postJson<{ id: string; slug: string }>(
    `${apiBase}/orgs/${orgSlug}/projects`,
    headers,
    { name: projectName, slug: projectSlug },
  );

  const app = await postJson<{ id: string; slug: string }>(
    `${apiBase}/orgs/${orgSlug}/projects/${projectSlug}/apps`,
    headers,
    { name: appName, slug: appSlug },
  );

  // Environments are org-level (see docs/CLAUDE.md "environments are org-level"
  // and migration 035_create_environments). No defaults are auto-seeded, so we
  // explicitly create "development" for this org.
  const env = await postJson<{ id: string; slug: string }>(
    `${apiBase}/orgs/${orgSlug}/environments`,
    headers,
    { name: 'Development', slug: environment, is_production: false },
  );

  // API keys: POST /api-keys requires `org_id` in the Gin context. The
  // ResolveOrgRole middleware only sets `org_id` when `:orgSlug` is present
  // in the URL path (internal/auth/rbac.go), and the `/api-keys` group is
  // mounted at the root (internal/auth/apikey_handler.go). The JWT returned
  // by /auth/register does not carry an OrgID claim either
  // (internal/auth/jwt.go::GenerateJWT), so a normal register-then-POST
  // flow cannot create an API key — this is a latent backend gap.
  //
  // Workaround for the hermetic e2e stack: mint our own JWT that includes
  // org_id in the claims. The middleware (internal/auth/middleware.go
  // lines 182-184) will then copy it into the Gin context. The e2e stack
  // uses a fixed JWT secret from deploy/e2e/api.env which we read from
  // DS_E2E_JWT_SECRET (default matches that file).
  const jwtSecret =
    process.env.DS_E2E_JWT_SECRET ?? 'e2e-test-jwt-secret-not-for-production-use';
  const orgScopedToken = mintOrgScopedJWT(jwtSecret, {
    userId: decodeJwtSub(token),
    email,
    orgId: org.id,
  });

  const apiKeyResp = await postJson<{
    api_key: { id: string };
    plaintext_key: string;
  }>(
    `${apiBase}/api-keys`,
    { 'Content-Type': 'application/json', Authorization: `Bearer ${orgScopedToken}` },
    {
      name: `E2E SDK Probe ${suffix}`,
      scopes: ['flags:read'],
    },
  );

  return {
    orgSlug,
    projectSlug,
    appSlug,
    orgId: org.id,
    projectId: project.id,
    appId: app.id,
    environmentId: env.id,
    environment,
    apiKey: apiKeyResp.plaintext_key,
    apiUrl,
    email,
    password,
    name,
  };
}

async function registerForToken(
  apiBase: string,
  email: string,
  password: string,
  name: string,
): Promise<string> {
  const res = await fetch(`${apiBase}/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, name }),
  });
  if (!res.ok) {
    throw new Error(`seed-via-ui: register failed (${res.status}): ${await res.text()}`);
  }
  const body = (await res.json()) as AuthResponse;
  return body.token;
}

/**
 * Decode a JWT's `sub` (registered claim) field without verifying the
 * signature. Used here to echo the same user ID back into the org-scoped
 * JWT we mint locally. We only ever use the result against the hermetic
 * e2e stack with a known secret — DO NOT use elsewhere.
 */
function decodeJwtSub(jwt: string): string {
  const parts = jwt.split('.');
  if (parts.length !== 3) {
    throw new Error(`seed-via-ui: malformed JWT (expected 3 parts, got ${parts.length})`);
  }
  const payload = JSON.parse(Buffer.from(parts[1], 'base64url').toString('utf-8'));
  if (typeof payload.user_id === 'string') return payload.user_id;
  if (typeof payload.sub === 'string') return payload.sub;
  throw new Error('seed-via-ui: JWT missing user_id/sub');
}

/**
 * Mint an HS256-signed JWT matching internal/auth/oauth.go::TokenClaims,
 * with the org_id claim populated. This lets the handler at POST /api-keys
 * succeed without requiring the org slug in the URL path.
 *
 * Only valid against the hermetic e2e stack where the JWT secret is known.
 */
function mintOrgScopedJWT(
  secret: string,
  claims: { userId: string; email: string; orgId: string },
): string {
  const now = Math.floor(Date.now() / 1000);
  const header = { alg: 'HS256', typ: 'JWT' };
  const payload = {
    iss: 'deploysentry',
    sub: claims.userId,
    iat: now,
    exp: now + 3600,
    jti: randomUUID(),
    user_id: claims.userId,
    email: claims.email,
    org_id: claims.orgId,
  };
  const encodedHeader = Buffer.from(JSON.stringify(header)).toString('base64url');
  const encodedPayload = Buffer.from(JSON.stringify(payload)).toString('base64url');
  const signingInput = `${encodedHeader}.${encodedPayload}`;
  const signature = createHmac('sha256', secret).update(signingInput).digest('base64url');
  return `${signingInput}.${signature}`;
}

async function postJson<T>(
  url: string,
  headers: Record<string, string>,
  body: unknown,
): Promise<T> {
  const res = await fetch(url, {
    method: 'POST',
    headers,
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`seed-via-ui: POST ${url} failed (${res.status}): ${await res.text()}`);
  }
  return (await res.json()) as T;
}

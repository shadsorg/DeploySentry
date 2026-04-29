# Mobile PWA — Phase 1: Scaffolding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up a new `mobile-pwa/` package with login, org picker, bottom-tab shell, account page, and PWA install metadata — serving placeholder content on the three tabs. No business functionality yet.

**Architecture:** New top-level Vite + React + TypeScript package sibling to `web/`, served at `/m` on the existing API origin. Service worker via `vite-plugin-pwa` in auto-update mode. Auth mirrors `web/src/auth.tsx` (JWT in `localStorage.ds_token`). Styling shares CSS variable tokens with `web/src/styles/globals.css`.

**Tech Stack:** Vite 5, React 18, TypeScript 5, react-router-dom 6, vite-plugin-pwa 0.20+, Workbox, vitest + @testing-library/react, Playwright (mobile viewport).

**Scope of this plan:** Phase 1 only (from `docs/superpowers/specs/2026-04-24-mobile-pwa-design.md`). Phases 2–6 (Status tab, History tab, Flags read, Flags writes, offline polish) each get their own follow-up plan.

---

## File Structure

```
mobile-pwa/
├── package.json                     # vite scripts, deps
├── tsconfig.json                    # strict TS + react-jsx
├── tsconfig.node.json               # for vite.config.ts
├── vite.config.ts                   # vite + vite-plugin-pwa
├── index.html                       # viewport, theme-color, manifest link
├── .eslintrc.cjs                    # mirrors web
├── vitest.config.ts                 # jsdom env
├── public/
│   ├── icon-192.png                 # 192x192 PWA icon (placeholder in plan, asset added in Step X)
│   ├── icon-512.png                 # 512x512 PWA icon
│   └── icon-maskable-512.png        # maskable variant
└── src/
    ├── main.tsx                     # ReactDOM.createRoot + <App/>
    ├── App.tsx                      # routes + providers
    ├── api.ts                       # request() + authApi + orgsApi (list only)
    ├── auth.tsx                     # AuthProvider, RequireAuth, RedirectIfAuth
    ├── authContext.ts               # createContext
    ├── authHooks.ts                 # useAuth
    ├── authTypes.ts                 # AuthContextValue
    ├── authJwt.ts                   # getTokenExpiryMs
    ├── types.ts                     # AuthUser, Organization (only what Phase 1 needs)
    ├── registerSW.ts                # SW registration + update prompt hook
    ├── layout/
    │   ├── MobileLayout.tsx         # <TopBar/> <Outlet/> <TabBar/>
    │   ├── TabBar.tsx               # 3 tabs, active-tab detection
    │   └── TopBar.tsx               # org chip + back chevron + refresh slot
    ├── pages/
    │   ├── LoginPage.tsx            # email/password form + OAuth buttons
    │   ├── OrgPickerPage.tsx        # list orgs; auto-redirect if exactly 1
    │   ├── StatusPage.tsx           # <div>Status — coming soon</div> placeholder
    │   ├── HistoryPage.tsx          # placeholder
    │   ├── FlagProjectPickerPage.tsx# placeholder
    │   └── SettingsPage.tsx         # user email, expiry, sign out
    ├── components/
    │   └── SessionExpiryWarning.tsx # modal from AuthContext
    └── styles/
        ├── tokens.css               # CSS variables copied from web globals.css
        └── mobile.css               # layout primitives (.m-screen, .m-tab-bar…)
```

**Makefile:**
- Modify `Makefile` — add `run-mobile` target.

**API server:**
- Modify `cmd/api/main.go` (or the static-serving file) — add `/m` static route serving `mobile-pwa/dist` in prod. Deferred to Step 12 below.

---

## Task 1: Create package skeleton

**Files:**
- Create: `mobile-pwa/package.json`
- Create: `mobile-pwa/tsconfig.json`
- Create: `mobile-pwa/tsconfig.node.json`
- Create: `mobile-pwa/.gitignore`
- Create: `mobile-pwa/index.html`

- [ ] **Step 1: Write `mobile-pwa/package.json`**

```json
{
  "name": "@deploysentry/mobile-pwa",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "lint": "eslint . --ext ts,tsx --report-unused-disable-directives --max-warnings 0",
    "test": "vitest run",
    "test:watch": "vitest"
  },
  "dependencies": {
    "react": "^18.3.0",
    "react-dom": "^18.3.0",
    "react-router-dom": "^6.26.0"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "^6.9.1",
    "@testing-library/react": "^16.3.2",
    "@testing-library/user-event": "^14.6.1",
    "@types/node": "^25.6.0",
    "@types/react": "^18.3.0",
    "@types/react-dom": "^18.3.0",
    "@typescript-eslint/eslint-plugin": "^7.0.0",
    "@typescript-eslint/parser": "^7.0.0",
    "@vitejs/plugin-react": "^4.7.0",
    "eslint": "^8.57.0",
    "eslint-plugin-react-hooks": "^4.6.0",
    "eslint-plugin-react-refresh": "^0.4.0",
    "jsdom": "^29.0.2",
    "typescript": "^5.5.0",
    "vite": "^5.4.0",
    "vite-plugin-pwa": "^0.20.5",
    "vitest": "^4.1.4",
    "workbox-window": "^7.1.0"
  }
}
```

- [ ] **Step 2: Write `mobile-pwa/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable", "WebWorker"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "types": ["vite/client", "vite-plugin-pwa/client"]
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 3: Write `mobile-pwa/tsconfig.node.json`**

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true,
    "strict": true
  },
  "include": ["vite.config.ts", "vitest.config.ts"]
}
```

- [ ] **Step 4: Write `mobile-pwa/.gitignore`**

```
node_modules
dist
dist-ssr
*.local
.vite
coverage
```

- [ ] **Step 5: Write `mobile-pwa/index.html`**

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover" />
    <meta name="theme-color" content="#0f1419" />
    <meta name="apple-mobile-web-app-capable" content="yes" />
    <meta name="apple-mobile-web-app-status-bar-style" content="black-translucent" />
    <meta name="apple-mobile-web-app-title" content="Deploy Sentry" />
    <link rel="manifest" href="/m/manifest.webmanifest" />
    <link rel="apple-touch-icon" href="/m/icon-192.png" />
    <title>Deploy Sentry</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 6: Install dependencies and commit**

```bash
cd mobile-pwa && npm install && cd ..
git add mobile-pwa/package.json mobile-pwa/package-lock.json mobile-pwa/tsconfig.json mobile-pwa/tsconfig.node.json mobile-pwa/.gitignore mobile-pwa/index.html
git commit -m "feat(mobile-pwa): scaffold package (phase 1)"
```

---

## Task 2: Vite + PWA config

**Files:**
- Create: `mobile-pwa/vite.config.ts`
- Create: `mobile-pwa/vitest.config.ts`
- Create: `mobile-pwa/.eslintrc.cjs`

- [ ] **Step 1: Write `mobile-pwa/vite.config.ts`**

```ts
import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import { VitePWA } from 'vite-plugin-pwa';
import path from 'path';

export default defineConfig(({ mode }) => {
  const env = { ...process.env, ...loadEnv(mode, process.cwd(), '') };
  const apiTarget = env.VITE_API_PROXY_TARGET ?? 'http://localhost:8080';
  const devPort = Number(env.VITE_DEV_PORT ?? 3002);

  return {
    base: '/m/',
    plugins: [
      react(),
      VitePWA({
        registerType: 'autoUpdate',
        injectRegister: null, // we call registerSW from src/registerSW.ts
        includeAssets: ['icon-192.png', 'icon-512.png', 'icon-maskable-512.png'],
        manifest: {
          id: '/m/',
          name: 'Deploy Sentry',
          short_name: 'DS',
          description: 'Monitor deployments and manage feature flags.',
          start_url: '/m/',
          scope: '/m/',
          display: 'standalone',
          orientation: 'portrait',
          background_color: '#0f1419',
          theme_color: '#0f1419',
          icons: [
            { src: 'icon-192.png', sizes: '192x192', type: 'image/png' },
            { src: 'icon-512.png', sizes: '512x512', type: 'image/png' },
            {
              src: 'icon-maskable-512.png',
              sizes: '512x512',
              type: 'image/png',
              purpose: 'maskable',
            },
          ],
        },
        workbox: {
          // Phase 1: precache the app shell only. API runtime caching is added in Phase 6.
          globPatterns: ['**/*.{js,css,html,png,svg,ico,webmanifest}'],
          navigateFallback: '/m/index.html',
          navigateFallbackDenylist: [/^\/api\//],
        },
        devOptions: {
          enabled: false, // SW only in prod build to avoid caching dev modules
        },
      }),
    ],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      port: devPort,
      strictPort: true,
      proxy: {
        '/api': { target: apiTarget, changeOrigin: true },
      },
    },
    build: {
      outDir: 'dist',
      sourcemap: true,
    },
  };
});
```

- [ ] **Step 2: Write `mobile-pwa/vitest.config.ts`**

```ts
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
});
```

- [ ] **Step 3: Write `mobile-pwa/.eslintrc.cjs`**

```js
module.exports = {
  root: true,
  env: { browser: true, es2022: true, node: true },
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:react-hooks/recommended',
  ],
  ignorePatterns: ['dist', '.eslintrc.cjs', 'node_modules'],
  parser: '@typescript-eslint/parser',
  plugins: ['react-refresh'],
  rules: {
    'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
  },
};
```

- [ ] **Step 4: Create test setup file**

`mobile-pwa/src/test/setup.ts`:

```ts
import '@testing-library/jest-dom/vitest';
```

- [ ] **Step 5: Verify config parses**

```bash
cd mobile-pwa && npx tsc --noEmit && cd ..
```

Expected: exits 0 with no output (no src files yet but config should type-check).

- [ ] **Step 6: Commit**

```bash
git add mobile-pwa/vite.config.ts mobile-pwa/vitest.config.ts mobile-pwa/.eslintrc.cjs mobile-pwa/src/test/setup.ts
git commit -m "feat(mobile-pwa): add vite + vitest + eslint config with PWA manifest"
```

---

## Task 3: Placeholder PWA icons

**Files:**
- Create: `mobile-pwa/public/icon-192.png`
- Create: `mobile-pwa/public/icon-512.png`
- Create: `mobile-pwa/public/icon-maskable-512.png`

- [ ] **Step 1: Generate placeholder icons**

Use ImageMagick (installed via `brew install imagemagick`) to generate solid-color PNGs that match the reskin theme-color. These are intentional placeholders — final artwork comes from design.

```bash
cd mobile-pwa/public
magick -size 192x192 canvas:'#0f1419' -fill '#3b82f6' -gravity center -font Helvetica -pointsize 96 -annotate 0 'DS' icon-192.png
magick -size 512x512 canvas:'#0f1419' -fill '#3b82f6' -gravity center -font Helvetica -pointsize 240 -annotate 0 'DS' icon-512.png
# Maskable variant: same art but inset so the safe-zone (inner 80%) contains the mark.
magick -size 512x512 canvas:'#0f1419' -fill '#3b82f6' -gravity center -font Helvetica -pointsize 160 -annotate 0 'DS' icon-maskable-512.png
cd ../..
```

If ImageMagick isn't available, generate them in Node with `sharp`:

```bash
cd mobile-pwa && npx --yes sharp-cli --input=<(echo -n '') || true && cd ..
```

(Not required; ImageMagick is the recommended path.)

- [ ] **Step 2: Verify the three files exist**

```bash
ls -la mobile-pwa/public/icon-*.png
```

Expected: three files listed.

- [ ] **Step 3: Commit**

```bash
git add mobile-pwa/public/
git commit -m "feat(mobile-pwa): add placeholder PWA icons"
```

---

## Task 4: Shared CSS tokens + mobile layout primitives

**Files:**
- Create: `mobile-pwa/src/styles/tokens.css`
- Create: `mobile-pwa/src/styles/mobile.css`

- [ ] **Step 1: Copy theme tokens from web**

Identify the `:root` variable block in `web/src/styles/globals.css` (and any `.dark`/`[data-theme]` overrides) and copy ONLY the custom-property declarations into `mobile-pwa/src/styles/tokens.css`. Do not copy layout rules.

Read the source file first:

```bash
grep -n '^\s*--\|^:root\|^\.dark\|^\[data-theme' web/src/styles/globals.css
```

Then produce `mobile-pwa/src/styles/tokens.css` by selecting the `:root { ... }` block (and any matching dark-theme block) verbatim. Leave a header comment:

```css
/* Copied from web/src/styles/globals.css (token block only).
 * Mobile-pwa intentionally duplicates these values to stay decoupled
 * from the web package. When the web theme changes, sync this file. */
```

- [ ] **Step 2: Write `mobile-pwa/src/styles/mobile.css`**

```css
@import './tokens.css';

* {
  box-sizing: border-box;
}

html, body, #root {
  margin: 0;
  padding: 0;
  height: 100%;
  background: var(--bg, #0f1419);
  color: var(--text, #e6edf3);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -webkit-tap-highlight-color: transparent;
}

button {
  font: inherit;
  color: inherit;
}

a { color: inherit; }

/* Screen shell: full-height flex column with room for top+tab bars. */
.m-screen {
  display: flex;
  flex-direction: column;
  height: 100dvh;
  padding-top: env(safe-area-inset-top);
  padding-bottom: env(safe-area-inset-bottom);
}

.m-screen-body {
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  padding: 12px 16px;
}

.m-top-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border, #21262d);
  background: var(--surface, #0f1419);
  min-height: 52px;
}

.m-top-bar .m-org-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  background: var(--surface-2, #161b22);
  border: 1px solid var(--border, #21262d);
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.m-tab-bar {
  display: flex;
  border-top: 1px solid var(--border, #21262d);
  background: var(--surface, #0d1117);
}

.m-tab-bar button {
  flex: 1;
  background: transparent;
  border: none;
  padding: 10px 8px 8px;
  min-height: 52px;
  color: var(--muted, #8b949e);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.m-tab-bar button[aria-current='page'] {
  color: var(--accent, #3b82f6);
  border-top: 2px solid var(--accent, #3b82f6);
  padding-top: 8px;
}

.m-tab-bar button .m-tab-icon {
  font-size: 18px;
  line-height: 1;
}

.m-tab-bar button .m-tab-label {
  font-size: 11px;
  letter-spacing: 0.02em;
}

.m-card {
  background: var(--surface-2, #161b22);
  border: 1px solid var(--border, #21262d);
  border-radius: 10px;
  padding: 14px;
}

.m-list-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid var(--border, #21262d);
  min-height: 44px;
}

.m-list-row:last-child { border-bottom: none; }

.m-button {
  min-height: 44px;
  padding: 10px 16px;
  border-radius: 8px;
  border: 1px solid var(--border, #21262d);
  background: var(--surface-2, #161b22);
  color: var(--text, #e6edf3);
  font-weight: 600;
  cursor: pointer;
}

.m-button-primary {
  background: var(--accent, #3b82f6);
  border-color: var(--accent, #3b82f6);
  color: #fff;
}

.m-input {
  width: 100%;
  min-height: 44px;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--border, #21262d);
  background: var(--surface, #0d1117);
  color: var(--text, #e6edf3);
  font-size: 16px; /* 16px prevents iOS zoom on focus */
}

.m-page-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100dvh;
  color: var(--muted, #8b949e);
  font-size: 14px;
}
```

- [ ] **Step 3: Commit**

```bash
git add mobile-pwa/src/styles/
git commit -m "feat(mobile-pwa): add theme tokens and layout primitives"
```

---

## Task 5: Types subset + minimal api.ts

**Files:**
- Create: `mobile-pwa/src/types.ts`
- Create: `mobile-pwa/src/api.ts`
- Test: `mobile-pwa/src/api.test.ts`

- [ ] **Step 1: Write `mobile-pwa/src/types.ts`** (Phase-1-only subset)

```ts
export interface AuthUser {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
}

export interface Organization {
  id: string;
  name: string;
  slug: string;
  created_at: string;
  updated_at: string;
}
```

- [ ] **Step 2: Write the failing api test**

`mobile-pwa/src/api.test.ts`:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { authApi, orgsApi, setFetch } from './api';

describe('api', () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    setFetch(fetchMock);
    localStorage.clear();
  });

  it('authApi.login POSTs without Authorization header', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ token: 't', user: { id: '1', email: 'a@b.c', name: 'A' } }), { status: 200 }),
    );
    const res = await authApi.login({ email: 'a@b.c', password: 'pw' });
    expect(res.token).toBe('t');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/login',
      expect.objectContaining({ method: 'POST' }),
    );
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBeUndefined();
  });

  it('orgsApi.list includes Bearer token when JWT stored', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ organizations: [] }), { status: 200 }),
    );
    await orgsApi.list();
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer header.payload.sig');
  });

  it('orgsApi.list uses ApiKey scheme when token starts with ds_', async () => {
    localStorage.setItem('ds_token', 'ds_abc123');
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ organizations: [] }), { status: 200 }),
    );
    await orgsApi.list();
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('ApiKey ds_abc123');
  });

  it('401 clears token and redirects', async () => {
    localStorage.setItem('ds_token', 'expired');
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ error: 'nope' }), { status: 401 }));
    const assignMock = vi.fn();
    // jsdom provides a location but not assignable; stub it.
    Object.defineProperty(window, 'location', {
      value: { pathname: '/m/orgs', search: '', assign: assignMock },
      writable: true,
    });
    await expect(orgsApi.list()).rejects.toThrow();
    expect(localStorage.getItem('ds_token')).toBeNull();
    expect(assignMock).toHaveBeenCalledWith('/m/login?next=%2Fm%2Forgs');
  });
});
```

- [ ] **Step 3: Run test — it should fail (imports don't exist)**

```bash
cd mobile-pwa && npx vitest run src/api.test.ts && cd ..
```

Expected: FAIL — "Cannot find module './api'".

- [ ] **Step 4: Write `mobile-pwa/src/api.ts`**

```ts
import type { AuthUser, Organization } from './types';

const BASE = '/api/v1';

type FetchFn = typeof fetch;
let fetchImpl: FetchFn = (...args) => globalThis.fetch(...args);

export function setFetch(impl: FetchFn) {
  fetchImpl = impl;
}

function handleUnauthorized() {
  const path = window.location.pathname;
  if (path === '/m/login' || path.endsWith('/login')) return;
  localStorage.removeItem('ds_token');
  const next = encodeURIComponent(path + window.location.search);
  window.location.assign(`/m/login?next=${next}`);
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem('ds_token') ?? '';
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init?.headers as Record<string, string> | undefined),
  };
  if (token) {
    headers.Authorization = token.startsWith('ds_') ? `ApiKey ${token}` : `Bearer ${token}`;
  }
  const res = await fetchImpl(`${BASE}${path}`, { ...init, headers });
  if (res.status === 401) {
    handleUnauthorized();
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error ?? 'Session expired');
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error ?? `Request failed: ${res.status}`);
  }
  return (await res.json()) as T;
}

// Public (no Authorization header needed)
async function publicPost<T>(path: string, body: unknown): Promise<T> {
  const res = await fetchImpl(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const payload = await res.json().catch(() => ({}));
    throw new Error((payload as { error?: string }).error ?? `Request failed: ${res.status}`);
  }
  return (await res.json()) as T;
}

export const authApi = {
  login: (data: { email: string; password: string }) =>
    publicPost<{ token: string; user: AuthUser }>('/auth/login', data),
  register: (data: { email: string; password: string; name: string }) =>
    publicPost<{ token: string; user: AuthUser }>('/auth/register', data),
  me: () => request<AuthUser>('/users/me'),
  extend: () => request<{ token: string }>('/auth/extend', { method: 'POST' }),
  logout: () => {
    localStorage.removeItem('ds_token');
  },
};

export const orgsApi = {
  list: () => request<{ organizations: Organization[] }>('/orgs'),
};
```

- [ ] **Step 5: Run test — passes**

```bash
cd mobile-pwa && npx vitest run src/api.test.ts && cd ..
```

Expected: 4 passed.

- [ ] **Step 6: Commit**

```bash
git add mobile-pwa/src/types.ts mobile-pwa/src/api.ts mobile-pwa/src/api.test.ts
git commit -m "feat(mobile-pwa): add api client with auth + orgs.list"
```

---

## Task 6: JWT expiry helper

**Files:**
- Create: `mobile-pwa/src/authJwt.ts`
- Test: `mobile-pwa/src/authJwt.test.ts`

- [ ] **Step 1: Write the failing test**

`mobile-pwa/src/authJwt.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { getTokenExpiryMs } from './authJwt';

function makeJwt(payload: Record<string, unknown>): string {
  const toB64u = (s: string) => btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  return `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify(payload))}.sig`;
}

describe('getTokenExpiryMs', () => {
  it('returns null for null/undefined/empty input', () => {
    expect(getTokenExpiryMs(null)).toBeNull();
    expect(getTokenExpiryMs(undefined)).toBeNull();
    expect(getTokenExpiryMs('')).toBeNull();
  });

  it('returns null for API-key tokens (ds_ prefix)', () => {
    expect(getTokenExpiryMs('ds_abc123')).toBeNull();
  });

  it('returns null for malformed JWTs', () => {
    expect(getTokenExpiryMs('not-a-jwt')).toBeNull();
    expect(getTokenExpiryMs('a.b')).toBeNull();
  });

  it('returns ms-since-epoch for a JWT with exp', () => {
    const expSec = 1_800_000_000;
    expect(getTokenExpiryMs(makeJwt({ exp: expSec }))).toBe(expSec * 1000);
  });

  it('returns null when exp is missing', () => {
    expect(getTokenExpiryMs(makeJwt({ sub: 'u1' }))).toBeNull();
  });
});
```

- [ ] **Step 2: Run — fails**

```bash
cd mobile-pwa && npx vitest run src/authJwt.test.ts && cd ..
```

Expected: FAIL — module not found.

- [ ] **Step 3: Write `mobile-pwa/src/authJwt.ts`**

Copy verbatim from `web/src/authJwt.ts` (it's a pure function with no external deps and is already tested by the failing suite above):

```ts
interface JwtPayload {
  exp?: number;
}

function base64UrlDecode(input: string): string {
  const padded = input
    .replace(/-/g, '+')
    .replace(/_/g, '/')
    .padEnd(input.length + ((4 - (input.length % 4)) % 4), '=');
  return atob(padded);
}

export function getTokenExpiryMs(token: string | null | undefined): number | null {
  if (!token) return null;
  if (token.startsWith('ds_')) return null;
  const parts = token.split('.');
  if (parts.length !== 3) return null;
  try {
    const payload = JSON.parse(base64UrlDecode(parts[1])) as JwtPayload;
    if (typeof payload.exp !== 'number') return null;
    return payload.exp * 1000;
  } catch {
    return null;
  }
}
```

- [ ] **Step 4: Run — passes**

```bash
cd mobile-pwa && npx vitest run src/authJwt.test.ts && cd ..
```

Expected: 5 passed.

- [ ] **Step 5: Commit**

```bash
git add mobile-pwa/src/authJwt.ts mobile-pwa/src/authJwt.test.ts
git commit -m "feat(mobile-pwa): add JWT expiry helper"
```

---

## Task 7: Auth context, provider, and guards

**Files:**
- Create: `mobile-pwa/src/authTypes.ts`
- Create: `mobile-pwa/src/authContext.ts`
- Create: `mobile-pwa/src/authHooks.ts`
- Create: `mobile-pwa/src/auth.tsx`
- Test: `mobile-pwa/src/auth.test.tsx`

- [ ] **Step 1: Write `mobile-pwa/src/authTypes.ts`**

```ts
import type { AuthUser } from './types';

export interface AuthContextValue {
  user: AuthUser | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  expiresAt: number | null;
  expiryWarningOpen: boolean;
  extendSession: () => Promise<void>;
}
```

- [ ] **Step 2: Write `mobile-pwa/src/authContext.ts`**

```ts
import { createContext } from 'react';
import type { AuthContextValue } from './authTypes';

export type { AuthContextValue };
export const AuthContext = createContext<AuthContextValue | null>(null);
```

- [ ] **Step 3: Write `mobile-pwa/src/authHooks.ts`**

```ts
import { useContext } from 'react';
import { AuthContext, type AuthContextValue } from './authContext';

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
```

- [ ] **Step 4: Write the failing auth test**

`mobile-pwa/src/auth.test.tsx`:

```tsx
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AuthProvider, RequireAuth, RedirectIfAuth } from './auth';
import { useAuth } from './authHooks';
import { setFetch } from './api';

function Protected() {
  return <div>protected</div>;
}
function LoginScreen() {
  return <div>login</div>;
}
function Status() {
  const { user } = useAuth();
  return <div>user:{user?.email ?? 'none'}</div>;
}

function makeJwt(expSec: number): string {
  const toB64u = (s: string) =>
    btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  return `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify({ exp: expSec }))}.sig`;
}

describe('AuthProvider', () => {
  let fetchMock: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    fetchMock = vi.fn();
    setFetch(fetchMock);
    localStorage.clear();
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders loading then redirects unauthenticated to /login', async () => {
    render(
      <MemoryRouter initialEntries={['/status']}>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<LoginScreen />} />
            <Route element={<RequireAuth />}>
              <Route path="/status" element={<Protected />} />
            </Route>
          </Routes>
        </AuthProvider>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText('login')).toBeInTheDocument());
    expect(screen.queryByText('protected')).not.toBeInTheDocument();
  });

  it('restores session from localStorage token on mount', async () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    localStorage.setItem('ds_token', makeJwt(exp));
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }), { status: 200 }),
    );
    render(
      <MemoryRouter initialEntries={['/status']}>
        <AuthProvider>
          <Routes>
            <Route element={<RequireAuth />}>
              <Route path="/status" element={<Status />} />
            </Route>
          </Routes>
        </AuthProvider>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText('user:a@b.c')).toBeInTheDocument());
  });

  it('RedirectIfAuth pushes authed users away from /login', async () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    localStorage.setItem('ds_token', makeJwt(exp));
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }), { status: 200 }),
    );
    render(
      <MemoryRouter initialEntries={['/login']}>
        <AuthProvider>
          <Routes>
            <Route element={<RedirectIfAuth />}>
              <Route path="/login" element={<LoginScreen />} />
            </Route>
            <Route path="/" element={<div>home</div>} />
          </Routes>
        </AuthProvider>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText('home')).toBeInTheDocument());
  });

  it('login() stores token and sets user', async () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    const token = makeJwt(exp);
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith('/auth/login')) {
        return Promise.resolve(
          new Response(JSON.stringify({ token, user: { id: '1', email: 'a@b.c', name: 'A' } }), {
            status: 200,
          }),
        );
      }
      return Promise.reject(new Error('unexpected fetch: ' + url));
    });
    function LoginForm() {
      const { login, user } = useAuth();
      return (
        <div>
          <button onClick={() => login('a@b.c', 'pw')}>go</button>
          <span>email:{user?.email ?? 'none'}</span>
        </div>
      );
    }
    render(
      <MemoryRouter>
        <AuthProvider>
          <LoginForm />
        </AuthProvider>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText('email:none')).toBeInTheDocument());
    await act(async () => {
      screen.getByText('go').click();
    });
    await waitFor(() => expect(screen.getByText('email:a@b.c')).toBeInTheDocument());
    expect(localStorage.getItem('ds_token')).toBe(token);
  });
});
```

- [ ] **Step 5: Run — fails**

```bash
cd mobile-pwa && npx vitest run src/auth.test.tsx && cd ..
```

Expected: FAIL — module not found.

- [ ] **Step 6: Write `mobile-pwa/src/auth.tsx`**

Adapted from `web/src/auth.tsx` — drop `register` (not used in the mobile login page), add a `loginPath` constant pointing at `/m/login`. **Do not** copy `register` — mobile PWA is not a signup surface in v1. Leave a comment noting that OAuth buttons short-circuit around this provider.

```tsx
import { useState, useEffect, useCallback, useRef } from 'react';
import { Navigate, useLocation, Outlet } from 'react-router-dom';
import { authApi } from './api';
import type { AuthUser } from './types';
import { AuthContext } from './authContext';
import { useAuth } from './authHooks';
import { getTokenExpiryMs } from './authJwt';

const WARNING_LEAD_MS = 60_000;

function safeDelay(ms: number): number {
  return Math.min(Math.max(ms, 0), 2_147_483_000);
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [expiresAt, setExpiresAt] = useState<number | null>(null);
  const [expiryWarningOpen, setExpiryWarningOpen] = useState(false);

  const warnTimerRef = useRef<number | null>(null);
  const logoutTimerRef = useRef<number | null>(null);
  const logoutRef = useRef<() => void>(() => {});

  const clearTimers = () => {
    if (warnTimerRef.current !== null) {
      window.clearTimeout(warnTimerRef.current);
      warnTimerRef.current = null;
    }
    if (logoutTimerRef.current !== null) {
      window.clearTimeout(logoutTimerRef.current);
      logoutTimerRef.current = null;
    }
  };

  const scheduleExpiry = useCallback((exp: number | null) => {
    clearTimers();
    setExpiresAt(exp);
    setExpiryWarningOpen(false);
    if (exp == null) return;
    const now = Date.now();
    const msUntilExpiry = exp - now;
    if (msUntilExpiry <= 0) {
      logoutRef.current();
      return;
    }
    const msUntilWarning = msUntilExpiry - WARNING_LEAD_MS;
    if (msUntilWarning <= 0) {
      setExpiryWarningOpen(true);
    } else {
      warnTimerRef.current = window.setTimeout(() => {
        setExpiryWarningOpen(true);
      }, safeDelay(msUntilWarning));
    }
    logoutTimerRef.current = window.setTimeout(() => {
      logoutRef.current();
    }, safeDelay(msUntilExpiry));
  }, []);

  useEffect(() => {
    const token = localStorage.getItem('ds_token');
    if (!token) {
      setLoading(false);
      return;
    }
    authApi
      .me()
      .then((u) => {
        setUser(u);
        scheduleExpiry(getTokenExpiryMs(token));
      })
      .catch(() => localStorage.removeItem('ds_token'))
      .finally(() => setLoading(false));
    return clearTimers;
  }, [scheduleExpiry]);

  const login = useCallback(
    async (email: string, password: string) => {
      const { token, user } = await authApi.login({ email, password });
      localStorage.setItem('ds_token', token);
      setUser(user);
      scheduleExpiry(getTokenExpiryMs(token));
    },
    [scheduleExpiry],
  );

  const logout = useCallback(() => {
    clearTimers();
    setExpiryWarningOpen(false);
    setExpiresAt(null);
    authApi.logout();
    setUser(null);
  }, []);
  logoutRef.current = logout;

  const extendSession = useCallback(async () => {
    const { token } = await authApi.extend();
    localStorage.setItem('ds_token', token);
    scheduleExpiry(getTokenExpiryMs(token));
  }, [scheduleExpiry]);

  return (
    <AuthContext.Provider
      value={{ user, loading, login, logout, expiresAt, expiryWarningOpen, extendSession }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function RequireAuth() {
  const { user, loading } = useAuth();
  const location = useLocation();
  if (loading) return <div className="m-page-loading">Loading…</div>;
  if (!user) {
    const next = location.pathname + location.search;
    return <Navigate to={`/login?next=${encodeURIComponent(next)}`} state={{ from: location }} replace />;
  }
  return <Outlet />;
}

export function RedirectIfAuth() {
  const { user, loading } = useAuth();
  if (loading) return <div className="m-page-loading">Loading…</div>;
  if (user) return <Navigate to="/" replace />;
  return <Outlet />;
}
```

- [ ] **Step 7: Run — passes**

```bash
cd mobile-pwa && npx vitest run src/auth.test.tsx && cd ..
```

Expected: 4 passed.

- [ ] **Step 8: Commit**

```bash
git add mobile-pwa/src/authTypes.ts mobile-pwa/src/authContext.ts mobile-pwa/src/authHooks.ts mobile-pwa/src/auth.tsx mobile-pwa/src/auth.test.tsx
git commit -m "feat(mobile-pwa): add auth provider + guards (mirrors web)"
```

---

## Task 8: SessionExpiryWarning component

**Files:**
- Create: `mobile-pwa/src/components/SessionExpiryWarning.tsx`
- Test: `mobile-pwa/src/components/SessionExpiryWarning.test.tsx`

- [ ] **Step 1: Write the failing test**

`mobile-pwa/src/components/SessionExpiryWarning.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SessionExpiryWarning } from './SessionExpiryWarning';
import { AuthContext } from '../authContext';
import type { AuthContextValue } from '../authTypes';

function Harness({ ctx }: { ctx: Partial<AuthContextValue> }) {
  const value: AuthContextValue = {
    user: { id: '1', email: 'a@b.c', name: 'A' },
    loading: false,
    login: async () => {},
    logout: () => {},
    expiresAt: null,
    expiryWarningOpen: false,
    extendSession: async () => {},
    ...ctx,
  };
  return (
    <AuthContext.Provider value={value}>
      <SessionExpiryWarning />
    </AuthContext.Provider>
  );
}

describe('SessionExpiryWarning', () => {
  it('renders nothing when closed', () => {
    const { container } = render(<Harness ctx={{ expiryWarningOpen: false }} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders a warning and calls extendSession on button click', async () => {
    const extend = vi.fn().mockResolvedValue(undefined);
    render(<Harness ctx={{ expiryWarningOpen: true, expiresAt: Date.now() + 30_000, extendSession: extend }} />);
    expect(screen.getByText(/signing you out/i)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: /stay signed in/i }));
    expect(extend).toHaveBeenCalled();
  });

  it('calls logout when Sign out is clicked', async () => {
    const logout = vi.fn();
    render(<Harness ctx={{ expiryWarningOpen: true, logout }} />);
    await userEvent.click(screen.getByRole('button', { name: /sign out/i }));
    expect(logout).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run — fails**

```bash
cd mobile-pwa && npx vitest run src/components/SessionExpiryWarning.test.tsx && cd ..
```

Expected: FAIL — module not found.

- [ ] **Step 3: Implement**

`mobile-pwa/src/components/SessionExpiryWarning.tsx`:

```tsx
import { useAuth } from '../authHooks';

export function SessionExpiryWarning() {
  const { expiryWarningOpen, extendSession, logout } = useAuth();
  if (!expiryWarningOpen) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.6)',
        display: 'flex',
        alignItems: 'flex-end',
        zIndex: 1000,
      }}
    >
      <div
        style={{
          background: 'var(--surface, #161b22)',
          border: '1px solid var(--border, #21262d)',
          borderRadius: '16px 16px 0 0',
          padding: '16px 20px',
          width: '100%',
          paddingBottom: 'calc(env(safe-area-inset-bottom) + 16px)',
        }}
      >
        <h3 style={{ margin: '0 0 8px' }}>Session expiring</h3>
        <p style={{ margin: '0 0 16px', color: 'var(--muted, #8b949e)' }}>
          We&apos;re signing you out soon. Stay signed in to keep working.
        </p>
        <div style={{ display: 'flex', gap: 8 }}>
          <button
            type="button"
            className="m-button m-button-primary"
            style={{ flex: 1 }}
            onClick={() => {
              void extendSession();
            }}
          >
            Stay signed in
          </button>
          <button type="button" className="m-button" onClick={logout}>
            Sign out
          </button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run — passes**

```bash
cd mobile-pwa && npx vitest run src/components/SessionExpiryWarning.test.tsx && cd ..
```

Expected: 3 passed.

- [ ] **Step 5: Commit**

```bash
git add mobile-pwa/src/components/
git commit -m "feat(mobile-pwa): add SessionExpiryWarning sheet"
```

---

## Task 9: TabBar + TopBar + MobileLayout

**Files:**
- Create: `mobile-pwa/src/layout/TabBar.tsx`
- Create: `mobile-pwa/src/layout/TopBar.tsx`
- Create: `mobile-pwa/src/layout/MobileLayout.tsx`
- Test: `mobile-pwa/src/layout/TabBar.test.tsx`
- Test: `mobile-pwa/src/layout/MobileLayout.test.tsx`

- [ ] **Step 1: Write the failing TabBar test**

`mobile-pwa/src/layout/TabBar.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { TabBar } from './TabBar';

function LocationProbe() {
  const loc = useLocation();
  return <div data-testid="loc">{loc.pathname}</div>;
}

describe('TabBar', () => {
  it('renders three tabs and marks Status active on /orgs/:slug/status', () => {
    render(
      <MemoryRouter initialEntries={['/orgs/acme/status']}>
        <Routes>
          <Route
            path="/orgs/:orgSlug/*"
            element={
              <>
                <TabBar />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByRole('button', { name: /status/i })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('button', { name: /history/i })).not.toHaveAttribute('aria-current');
    expect(screen.getByRole('button', { name: /flags/i })).not.toHaveAttribute('aria-current');
  });

  it('navigates to history when History tab is clicked', async () => {
    render(
      <MemoryRouter initialEntries={['/orgs/acme/status']}>
        <Routes>
          <Route
            path="/orgs/:orgSlug/*"
            element={
              <>
                <TabBar />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>,
    );
    await userEvent.click(screen.getByRole('button', { name: /history/i }));
    expect(screen.getByTestId('loc').textContent).toBe('/orgs/acme/history');
  });

  it('marks History active on any /history/* drill-down', () => {
    render(
      <MemoryRouter initialEntries={['/orgs/acme/history/deploy-123']}>
        <Routes>
          <Route
            path="/orgs/:orgSlug/*"
            element={
              <>
                <TabBar />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByRole('button', { name: /history/i })).toHaveAttribute('aria-current', 'page');
  });
});
```

- [ ] **Step 2: Run — fails**

```bash
cd mobile-pwa && npx vitest run src/layout/TabBar.test.tsx && cd ..
```

Expected: FAIL — module not found.

- [ ] **Step 3: Implement `mobile-pwa/src/layout/TabBar.tsx`**

```tsx
import { useNavigate, useLocation, useParams } from 'react-router-dom';

const TABS = [
  { key: 'status', label: 'Status', icon: '●' },
  { key: 'history', label: 'History', icon: '▦' },
  { key: 'flags', label: 'Flags', icon: '⚑' },
] as const;

type TabKey = (typeof TABS)[number]['key'];

function activeTab(pathname: string, orgSlug?: string): TabKey | null {
  if (!orgSlug) return null;
  const prefix = `/orgs/${orgSlug}/`;
  if (!pathname.startsWith(prefix)) return null;
  const rest = pathname.slice(prefix.length).split('/')[0];
  if (rest === 'status' || rest === 'history' || rest === 'flags') return rest;
  return null;
}

export function TabBar() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const nav = useNavigate();
  const loc = useLocation();
  const current = activeTab(loc.pathname, orgSlug);
  if (!orgSlug) return null;

  return (
    <nav className="m-tab-bar" aria-label="Primary">
      {TABS.map((t) => (
        <button
          key={t.key}
          type="button"
          aria-current={current === t.key ? 'page' : undefined}
          onClick={() => nav(`/orgs/${orgSlug}/${t.key}`)}
        >
          <span className="m-tab-icon" aria-hidden>{t.icon}</span>
          <span className="m-tab-label">{t.label}</span>
        </button>
      ))}
    </nav>
  );
}
```

- [ ] **Step 4: Run — TabBar tests pass**

```bash
cd mobile-pwa && npx vitest run src/layout/TabBar.test.tsx && cd ..
```

Expected: 3 passed.

- [ ] **Step 5: Write failing MobileLayout test**

`mobile-pwa/src/layout/MobileLayout.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { MobileLayout } from './MobileLayout';

describe('MobileLayout', () => {
  it('renders top bar, outlet content, and tab bar', () => {
    render(
      <MemoryRouter initialEntries={['/orgs/acme/status']}>
        <Routes>
          <Route path="/orgs/:orgSlug" element={<MobileLayout />}>
            <Route path="status" element={<div>StatusScreen</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText('acme')).toBeInTheDocument(); // org chip
    expect(screen.getByText('StatusScreen')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /flags/i })).toBeInTheDocument();
  });
});
```

- [ ] **Step 6: Implement TopBar + MobileLayout**

`mobile-pwa/src/layout/TopBar.tsx`:

```tsx
import { Link, useParams } from 'react-router-dom';

export function TopBar() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  return (
    <header className="m-top-bar">
      {orgSlug ? (
        <Link to="/orgs" className="m-org-chip" aria-label="Switch organization">
          <span aria-hidden>●</span>
          {orgSlug}
        </Link>
      ) : null}
    </header>
  );
}
```

`mobile-pwa/src/layout/MobileLayout.tsx`:

```tsx
import { Outlet } from 'react-router-dom';
import { TopBar } from './TopBar';
import { TabBar } from './TabBar';

export function MobileLayout() {
  return (
    <div className="m-screen">
      <TopBar />
      <main className="m-screen-body">
        <Outlet />
      </main>
      <TabBar />
    </div>
  );
}
```

- [ ] **Step 7: Run layout tests — pass**

```bash
cd mobile-pwa && npx vitest run src/layout && cd ..
```

Expected: 4 passed total.

- [ ] **Step 8: Commit**

```bash
git add mobile-pwa/src/layout/
git commit -m "feat(mobile-pwa): add TopBar, TabBar, MobileLayout"
```

---

## Task 10: LoginPage + OrgPickerPage + placeholder tab pages + SettingsPage

**Files:**
- Create: `mobile-pwa/src/pages/LoginPage.tsx`
- Create: `mobile-pwa/src/pages/OrgPickerPage.tsx`
- Create: `mobile-pwa/src/pages/StatusPage.tsx`
- Create: `mobile-pwa/src/pages/HistoryPage.tsx`
- Create: `mobile-pwa/src/pages/FlagProjectPickerPage.tsx`
- Create: `mobile-pwa/src/pages/SettingsPage.tsx`
- Test: `mobile-pwa/src/pages/LoginPage.test.tsx`
- Test: `mobile-pwa/src/pages/OrgPickerPage.test.tsx`

- [ ] **Step 1: Write LoginPage failing test**

`mobile-pwa/src/pages/LoginPage.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { LoginPage } from './LoginPage';
import { AuthContext } from '../authContext';
import type { AuthContextValue } from '../authTypes';

function renderWith(ctx: Partial<AuthContextValue>) {
  const value: AuthContextValue = {
    user: null,
    loading: false,
    login: async () => {},
    logout: () => {},
    expiresAt: null,
    expiryWarningOpen: false,
    extendSession: async () => {},
    ...ctx,
  };
  return render(
    <MemoryRouter>
      <AuthContext.Provider value={value}>
        <LoginPage />
      </AuthContext.Provider>
    </MemoryRouter>,
  );
}

describe('LoginPage', () => {
  it('submits email+password to login()', async () => {
    const login = vi.fn().mockResolvedValue(undefined);
    renderWith({ login });
    await userEvent.type(screen.getByLabelText(/email/i), 'a@b.c');
    await userEvent.type(screen.getByLabelText(/password/i), 'hunter2');
    await userEvent.click(screen.getByRole('button', { name: /sign in$/i }));
    expect(login).toHaveBeenCalledWith('a@b.c', 'hunter2');
  });

  it('shows the server error on failed login', async () => {
    const login = vi.fn().mockRejectedValue(new Error('invalid creds'));
    renderWith({ login });
    await userEvent.type(screen.getByLabelText(/email/i), 'a@b.c');
    await userEvent.type(screen.getByLabelText(/password/i), 'wrong');
    await userEvent.click(screen.getByRole('button', { name: /sign in$/i }));
    expect(await screen.findByText(/invalid creds/i)).toBeInTheDocument();
  });

  it('renders OAuth links pointing at /api/v1/auth/oauth/*', () => {
    renderWith({});
    const gh = screen.getByRole('link', { name: /github/i });
    const goog = screen.getByRole('link', { name: /google/i });
    expect(gh).toHaveAttribute('href', '/api/v1/auth/oauth/github');
    expect(goog).toHaveAttribute('href', '/api/v1/auth/oauth/google');
  });
});
```

- [ ] **Step 2: Implement LoginPage**

`mobile-pwa/src/pages/LoginPage.tsx`:

```tsx
import { useState } from 'react';
import { useAuth } from '../authHooks';

export function LoginPage() {
  const { login } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await login(email, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sign-in failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="m-screen" style={{ padding: '24px 20px' }}>
      <h1 style={{ fontSize: 22, margin: '24px 0 4px' }}>Deploy Sentry</h1>
      <p style={{ color: 'var(--muted, #8b949e)', marginTop: 0 }}>Sign in to continue.</p>

      <form onSubmit={onSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 24 }}>
        <label>
          <span style={{ fontSize: 12, color: 'var(--muted)' }}>Email</span>
          <input
            className="m-input"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </label>
        <label>
          <span style={{ fontSize: 12, color: 'var(--muted)' }}>Password</span>
          <input
            className="m-input"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </label>
        {error && <div style={{ color: 'var(--danger, #f87171)', fontSize: 13 }}>{error}</div>}
        <button type="submit" className="m-button m-button-primary" disabled={busy}>
          {busy ? 'Signing in…' : 'Sign in'}
        </button>
      </form>

      <div style={{ textAlign: 'center', margin: '24px 0 12px', color: 'var(--muted)', fontSize: 12 }}>
        or
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <a className="m-button" href="/api/v1/auth/oauth/github" style={{ textAlign: 'center' }}>
          Sign in with GitHub
        </a>
        <a className="m-button" href="/api/v1/auth/oauth/google" style={{ textAlign: 'center' }}>
          Sign in with Google
        </a>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Run LoginPage tests — pass**

```bash
cd mobile-pwa && npx vitest run src/pages/LoginPage.test.tsx && cd ..
```

Expected: 3 passed.

- [ ] **Step 4: Write OrgPickerPage failing test**

`mobile-pwa/src/pages/OrgPickerPage.test.tsx`:

```tsx
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { OrgPickerPage } from './OrgPickerPage';
import { setFetch } from '../api';

function LocationProbe() {
  const loc = useLocation();
  return <div data-testid="loc">{loc.pathname}</div>;
}

describe('OrgPickerPage', () => {
  let fetchMock: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    fetchMock = vi.fn();
    setFetch(fetchMock);
    localStorage.clear();
    localStorage.setItem('ds_token', 'header.payload.sig');
  });

  it('auto-redirects to /orgs/:slug/status when user has exactly one org', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          organizations: [{ id: '1', name: 'Acme', slug: 'acme', created_at: '', updated_at: '' }],
        }),
        { status: 200 },
      ),
    );
    render(
      <MemoryRouter initialEntries={['/orgs']}>
        <Routes>
          <Route path="/orgs" element={<OrgPickerPage />} />
          <Route path="/orgs/:orgSlug/status" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByTestId('loc').textContent).toBe('/orgs/acme/status'));
  });

  it('renders picker when user has multiple orgs', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          organizations: [
            { id: '1', name: 'Acme', slug: 'acme', created_at: '', updated_at: '' },
            { id: '2', name: 'Beta', slug: 'beta', created_at: '', updated_at: '' },
          ],
        }),
        { status: 200 },
      ),
    );
    render(
      <MemoryRouter initialEntries={['/orgs']}>
        <Routes>
          <Route path="/orgs" element={<OrgPickerPage />} />
          <Route path="/orgs/:orgSlug/status" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(await screen.findByText('Acme')).toBeInTheDocument();
    expect(screen.getByText('Beta')).toBeInTheDocument();
    await userEvent.click(screen.getByText('Beta'));
    await waitFor(() => expect(screen.getByTestId('loc').textContent).toBe('/orgs/beta/status'));
    expect(localStorage.getItem('ds_active_org')).toBe('beta');
  });
});
```

- [ ] **Step 5: Implement OrgPickerPage**

`mobile-pwa/src/pages/OrgPickerPage.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { orgsApi } from '../api';
import type { Organization } from '../types';

export function OrgPickerPage() {
  const nav = useNavigate();
  const [orgs, setOrgs] = useState<Organization[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    orgsApi
      .list()
      .then((r) => {
        if (r.organizations.length === 1) {
          const only = r.organizations[0];
          localStorage.setItem('ds_active_org', only.slug);
          nav(`/orgs/${only.slug}/status`, { replace: true });
        } else {
          setOrgs(r.organizations);
        }
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load organizations'));
  }, [nav]);

  if (error) {
    return (
      <div className="m-screen" style={{ padding: 20 }}>
        <p style={{ color: 'var(--danger)' }}>{error}</p>
      </div>
    );
  }
  if (orgs === null) {
    return <div className="m-page-loading">Loading…</div>;
  }
  if (orgs.length === 0) {
    return (
      <div className="m-screen" style={{ padding: 20 }}>
        <h2>No organizations</h2>
        <p style={{ color: 'var(--muted)' }}>
          You&apos;re not a member of any organization. Ask an admin to invite you, or create one in the desktop dashboard.
        </p>
      </div>
    );
  }

  return (
    <div className="m-screen" style={{ padding: 20 }}>
      <h2 style={{ margin: '8px 0 16px' }}>Choose an organization</h2>
      <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
        {orgs.map((o) => (
          <li key={o.id} className="m-list-row">
            <button
              type="button"
              className="m-button"
              style={{ width: '100%', textAlign: 'left' }}
              onClick={() => {
                localStorage.setItem('ds_active_org', o.slug);
                nav(`/orgs/${o.slug}/status`);
              }}
            >
              {o.name}
              <span style={{ color: 'var(--muted)', marginLeft: 8, fontSize: 12 }}>{o.slug}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
```

- [ ] **Step 6: Run OrgPickerPage tests — pass**

```bash
cd mobile-pwa && npx vitest run src/pages/OrgPickerPage.test.tsx && cd ..
```

Expected: 2 passed.

- [ ] **Step 7: Write placeholder tab pages**

`mobile-pwa/src/pages/StatusPage.tsx`:

```tsx
export function StatusPage() {
  return (
    <section>
      <h2>Status</h2>
      <p style={{ color: 'var(--muted)' }}>Coming in phase 2.</p>
    </section>
  );
}
```

`mobile-pwa/src/pages/HistoryPage.tsx`:

```tsx
export function HistoryPage() {
  return (
    <section>
      <h2>Deploy History</h2>
      <p style={{ color: 'var(--muted)' }}>Coming in phase 3.</p>
    </section>
  );
}
```

`mobile-pwa/src/pages/FlagProjectPickerPage.tsx`:

```tsx
export function FlagProjectPickerPage() {
  return (
    <section>
      <h2>Flags</h2>
      <p style={{ color: 'var(--muted)' }}>Coming in phase 4.</p>
    </section>
  );
}
```

- [ ] **Step 8: Write SettingsPage**

`mobile-pwa/src/pages/SettingsPage.tsx`:

```tsx
import { useAuth } from '../authHooks';

function formatExpiry(ms: number | null): string {
  if (ms == null) return 'unknown';
  const secs = Math.max(0, Math.floor((ms - Date.now()) / 1000));
  if (secs <= 0) return 'expired';
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return `${m}m ${s}s`;
}

export function SettingsPage() {
  const { user, expiresAt, logout } = useAuth();
  return (
    <section>
      <h2>Account</h2>
      <div className="m-card" style={{ marginBottom: 16 }}>
        <div className="m-list-row">
          <span style={{ color: 'var(--muted)' }}>Signed in as</span>
          <span>{user?.email ?? '—'}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--muted)' }}>Session expires in</span>
          <span>{formatExpiry(expiresAt)}</span>
        </div>
      </div>
      <button type="button" className="m-button" style={{ width: '100%' }} onClick={logout}>
        Sign out
      </button>
      <p style={{ color: 'var(--muted)', fontSize: 12, marginTop: 24 }}>
        For org / project / member management, open the{' '}
        <a href="/" style={{ color: 'var(--accent)' }}>desktop dashboard</a>.
      </p>
    </section>
  );
}
```

- [ ] **Step 9: Commit**

```bash
git add mobile-pwa/src/pages/
git commit -m "feat(mobile-pwa): add login, org picker, settings, tab placeholders"
```

---

## Task 11: App routes + main entry + SW registration

**Files:**
- Create: `mobile-pwa/src/registerSW.ts`
- Create: `mobile-pwa/src/App.tsx`
- Create: `mobile-pwa/src/main.tsx`
- Test: `mobile-pwa/src/App.test.tsx`

- [ ] **Step 1: Write SW registration helper**

`mobile-pwa/src/registerSW.ts`:

```ts
import { registerSW } from 'virtual:pwa-register';

export function initServiceWorker() {
  // autoUpdate: Workbox installs the new SW automatically on next nav.
  // We still prompt for reload so the user sees a fresh bundle immediately.
  const update = registerSW({
    immediate: true,
    onNeedRefresh() {
      // Phase 6 replaces this with a proper banner UI.
      // eslint-disable-next-line no-console
      console.info('[pwa] update available — reloading');
      update(true);
    },
    onOfflineReady() {
      // eslint-disable-next-line no-console
      console.info('[pwa] offline-ready');
    },
  });
}
```

- [ ] **Step 2: Write failing App test**

`mobile-pwa/src/App.test.tsx`:

```tsx
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AppRoutes } from './App';
import { setFetch } from './api';

describe('AppRoutes', () => {
  let fetchMock: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    fetchMock = vi.fn();
    setFetch(fetchMock);
    localStorage.clear();
  });

  it('unauthenticated visit to / bounces to /login', async () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <AppRoutes />
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText(/Sign in$/i)).toBeInTheDocument());
  });

  it('authenticated visit renders inside the layout (TabBar visible on status)', async () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    const toB64u = (s: string) => btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const token = `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify({ exp }))}.sig`;
    localStorage.setItem('ds_token', token);
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith('/users/me'))
        return Promise.resolve(
          new Response(JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }), { status: 200 }),
        );
      if (url.endsWith('/orgs'))
        return Promise.resolve(
          new Response(
            JSON.stringify({
              organizations: [{ id: '1', name: 'Acme', slug: 'acme', created_at: '', updated_at: '' }],
            }),
            { status: 200 },
          ),
        );
      return Promise.reject(new Error('unexpected: ' + url));
    });
    render(
      <MemoryRouter initialEntries={['/orgs']}>
        <AppRoutes />
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByRole('button', { name: /status/i })).toBeInTheDocument());
  });
});
```

- [ ] **Step 3: Implement `mobile-pwa/src/App.tsx`**

```tsx
import { Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, RequireAuth, RedirectIfAuth } from './auth';
import { SessionExpiryWarning } from './components/SessionExpiryWarning';
import { MobileLayout } from './layout/MobileLayout';
import { LoginPage } from './pages/LoginPage';
import { OrgPickerPage } from './pages/OrgPickerPage';
import { StatusPage } from './pages/StatusPage';
import { HistoryPage } from './pages/HistoryPage';
import { FlagProjectPickerPage } from './pages/FlagProjectPickerPage';
import { SettingsPage } from './pages/SettingsPage';

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<RedirectIfAuth />}>
        <Route path="/login" element={<LoginPage />} />
      </Route>
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Navigate to="/orgs" replace />} />
        <Route path="/orgs" element={<OrgPickerPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/orgs/:orgSlug" element={<MobileLayout />}>
          <Route index element={<Navigate to="status" replace />} />
          <Route path="status" element={<StatusPage />} />
          <Route path="history" element={<HistoryPage />} />
          <Route path="flags" element={<FlagProjectPickerPage />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <SessionExpiryWarning />
      <AppRoutes />
    </AuthProvider>
  );
}
```

- [ ] **Step 4: Implement `mobile-pwa/src/main.tsx`**

```tsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import App from './App';
import { initServiceWorker } from './registerSW';
import './styles/mobile.css';

initServiceWorker();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter basename="/m">
      <App />
    </BrowserRouter>
  </React.StrictMode>,
);
```

- [ ] **Step 5: Run App tests — pass**

```bash
cd mobile-pwa && npx vitest run src/App.test.tsx && cd ..
```

Expected: 2 passed.

- [ ] **Step 6: Run full test suite — green**

```bash
cd mobile-pwa && npx vitest run && cd ..
```

Expected: all tests pass.

- [ ] **Step 7: Typecheck**

```bash
cd mobile-pwa && npx tsc --noEmit && cd ..
```

Expected: exits 0.

- [ ] **Step 8: Dev build smoke**

```bash
cd mobile-pwa && npx vite build && cd ..
```

Expected: build succeeds, `dist/` contains `index.html`, `assets/*.js`, `assets/*.css`, `manifest.webmanifest`, `sw.js`, `workbox-*.js`, and the icons.

- [ ] **Step 9: Commit**

```bash
git add mobile-pwa/src/App.tsx mobile-pwa/src/App.test.tsx mobile-pwa/src/main.tsx mobile-pwa/src/registerSW.ts
git commit -m "feat(mobile-pwa): add App routes + SW registration"
```

---

## Task 12: Serve `/m` from the API binary

**Files:**
- Modify: `cmd/api/main.go` (or the existing static-file handler — identify by grep first)
- Modify: `Makefile`

- [ ] **Step 1: Find where `web/dist` is served today**

```bash
grep -rn "web/dist\|StaticFS\|StaticFile\|NoRoute\|Use.*Static" cmd/api/ internal/ | head -20
```

Expected output (record this before editing): lines that show how `web/dist` is served. You will mirror that pattern for `mobile-pwa/dist`.

- [ ] **Step 2: Add a `/m` static route next to the existing `web` route**

If the existing handler uses Gin's `router.StaticFS("/", http.Dir("web/dist"))`, add a sibling:

```go
// Serve the mobile PWA under /m (Phase 1 of mobile PWA).
router.StaticFS("/m", http.Dir("mobile-pwa/dist"))
// Fallback: SPA routes inside /m should serve index.html.
router.NoRoute(func(c *gin.Context) {
    p := c.Request.URL.Path
    if strings.HasPrefix(p, "/m/") {
        c.File("mobile-pwa/dist/index.html")
        return
    }
    // existing NoRoute behavior for the web app continues here
})
```

Adapt to whatever pattern already exists. **Do not** regress the existing web SPA fallback — preserve its behavior for non-`/m` paths. If the repo uses `echo`, `chi`, or plain `net/http`, use the equivalent idiom there.

- [ ] **Step 3: Add Makefile target**

Modify `Makefile` — add near the other `run-*` targets:

```make
.PHONY: run-mobile
run-mobile:
	cd mobile-pwa && npm install && npm run dev

.PHONY: build-mobile
build-mobile:
	cd mobile-pwa && npm install && npm run build
```

- [ ] **Step 4: Verify end-to-end locally**

Terminal 1: `make run-api`
Terminal 2: `cd mobile-pwa && npm run build && cd ..`
Terminal 3: `curl -sI http://localhost:8080/m/ | head -5`

Expected: `200 OK` on `/m/` returning `text/html`. Then open `http://localhost:8080/m/` in a mobile viewport (Chrome DevTools device emulation), confirm:
- Login screen renders.
- Installable prompt appears (DevTools → Application → Manifest shows no errors).
- After sign-in with a test account, org picker or status page renders with the tab bar.

- [ ] **Step 5: Commit**

```bash
git add cmd/api Makefile
git commit -m "feat(api): serve mobile-pwa at /m with SPA fallback"
```

---

## Task 13: E2E smoke test (Playwright, mobile viewport)

**Files:**
- Modify: `web/package.json` or create `mobile-pwa/playwright.config.ts` (new — keep mobile e2e separate from web)
- Create: `mobile-pwa/e2e/smoke.spec.ts`

- [ ] **Step 1: Install Playwright (dev)**

```bash
cd mobile-pwa && npm install -D @playwright/test && npx playwright install chromium && cd ..
```

- [ ] **Step 2: Write `mobile-pwa/playwright.config.ts`**

```ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: process.env.MOBILE_PWA_BASE_URL ?? 'http://localhost:3002/m',
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
```

- [ ] **Step 3: Write the smoke spec**

`mobile-pwa/e2e/smoke.spec.ts`:

```ts
import { test, expect } from '@playwright/test';

test('login screen renders on first visit', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: /deploy sentry/i })).toBeVisible();
  await expect(page.getByLabel('Email')).toBeVisible();
  await expect(page.getByLabel('Password')).toBeVisible();
  await expect(page.getByRole('button', { name: /sign in$/i })).toBeVisible();
});

test('unauthenticated deep-link bounces to /login with next param', async ({ page }) => {
  await page.goto('/orgs/acme/status');
  await expect(page).toHaveURL(/\/login\?next=%2Forgs%2Facme%2Fstatus$/);
});

test('manifest is reachable and valid JSON', async ({ request }) => {
  const res = await request.get('/manifest.webmanifest');
  expect(res.status()).toBe(200);
  const body = await res.json();
  expect(body.name).toBe('Deploy Sentry');
  expect(body.start_url).toBe('/m/');
  expect(body.icons?.length ?? 0).toBeGreaterThanOrEqual(3);
});
```

- [ ] **Step 4: Add script to package.json**

Modify `mobile-pwa/package.json`'s `scripts` block — add:

```json
"test:e2e": "playwright test",
"test:e2e:ui": "playwright test --ui"
```

- [ ] **Step 5: Run e2e tests**

```bash
cd mobile-pwa && npm run test:e2e && cd ..
```

Expected: 3 tests pass (`smoke.spec.ts`).

- [ ] **Step 6: Commit**

```bash
git add mobile-pwa/playwright.config.ts mobile-pwa/e2e/ mobile-pwa/package.json mobile-pwa/package-lock.json
git commit -m "test(mobile-pwa): add playwright smoke suite (iPhone 13 viewport)"
```

---

## Task 14: Update initiative tracker + spec phase 1 note

**Files:**
- Modify: `docs/Current_Initiatives.md`
- Modify: `docs/superpowers/specs/2026-04-24-mobile-pwa-design.md`

- [ ] **Step 1: Change the PWA initiative row's Phase to `Implementation`**

Edit `docs/Current_Initiatives.md` — on the `Mobile PWA` row, change `Design` to `Implementation` and reference this plan file.

Before:
```md
| Mobile PWA | Design | [Spec](./superpowers/specs/2026-04-24-mobile-pwa-design.md) | New top-level ...
```

After:
```md
| Mobile PWA | Implementation | [Spec](./superpowers/specs/2026-04-24-mobile-pwa-design.md) / [Phase 1 Plan](./superpowers/plans/2026-04-24-mobile-pwa-phase1-scaffolding.md) | Phase 1 (scaffolding) in flight; phases 2–6 each get their own plan. ...
```

- [ ] **Step 2: Update `Last updated` date at top of Current_Initiatives.md to today**

- [ ] **Step 3: No spec phase change yet** — spec stays in `Design` phase until all six phase plans land (per the CLAUDE.md rule that Phase flips to Complete only after all work is merged and CI passes).

- [ ] **Step 4: Commit**

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: mark mobile PWA initiative as Implementation (phase 1 in flight)"
```

---

## Task 15: Final verification

- [ ] **Step 1: Full test sweep**

```bash
cd mobile-pwa && npm run lint && npx tsc --noEmit && npm run test && npm run build && cd ..
```

Expected:
- lint: no warnings.
- tsc: no errors.
- vitest: all tests pass.
- vite build: succeeds; `dist/` exists with manifest, sw.js, icons, and bundled assets.

- [ ] **Step 2: Visual sanity on real device (if available)**

- Open DevTools mobile emulation (iPhone 13) → navigate to `http://localhost:3002/m/`.
- Verify: login renders, OAuth links visible, inputs are readable (font-size ≥ 16 px).
- Verify: bottom safe-area padding does not clip the tab bar on an iPhone X-class viewport.
- Verify: `Application → Manifest` shows no warnings; `Application → Service Workers` lists a registered SW (only in production build).

- [ ] **Step 3: Production build smoke behind the API**

Terminal 1: `make run-api`
Terminal 2: `make build-mobile`
Open `http://localhost:8080/m/` — verify the login screen renders.

- [ ] **Step 4: Final commit (if any fixes were needed)**

```bash
git status
# If anything was modified during verification, stage and commit:
# git add <files> && git commit -m "fix(mobile-pwa): <specific fix>"
```

---

## Success criteria

Phase 1 is complete when:

1. `cd mobile-pwa && npm run build` produces a valid PWA bundle with a precached app shell.
2. Visiting `/m/` serves the login screen.
3. Login with a valid user stores `ds_token`, redirects to `/orgs`, and:
   - With exactly 1 org → redirects straight to `/orgs/:slug/status`.
   - With multiple orgs → renders the picker.
4. The bottom tab bar is visible on status / history / flags and switches routes correctly.
5. `/settings` shows the current user and lets them sign out.
6. Expired token → auto-logout modal appears, "Stay signed in" successfully refreshes.
7. PWA is installable (Chrome shows Install prompt; iOS Safari's "Add to Home Screen" produces a standalone app).
8. Service worker is registered in the production build and app shell works offline (load once online, then turn network off → reload → shell still renders; API calls of course fail).
9. All unit tests pass; Playwright smoke passes.

## Out of scope for Phase 1 (tracked in Phases 2–6 plans)

- Any real data on Status / History / Flags tabs (placeholder content only in phase 1).
- Runtime API caching / stale-while-revalidate (precache only in phase 1).
- Offline-write-blocked modal.
- Build-pill, monitoring link rendering, deploy detail drill-down.
- Flag edits of any kind.
- SW update banner UI (logged to console in phase 1; proper banner in phase 6).

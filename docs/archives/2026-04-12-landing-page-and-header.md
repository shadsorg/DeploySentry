# Landing Page, Shared Header, and In-App Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Add a public dev-tool-minimal landing page at `/`, replace the sidebar's user footer with a shared `SiteHeader` carrying a top-right user menu, and ship an in-app `/docs` route rendered from bundled markdown.

**Architecture:** A single shared `SiteHeader` component (variant `landing` or `app`) sits at the top of every view. Routing makes `/` the landing page for authed and unauthed users alike; post-login goes to a new `/portal` route that uses the existing `DefaultRedirect` to land on the user's last org. The landing page is a thin orchestrator over presentational section components, with section reveals via Framer Motion and the deploy→release flow as a hand-crafted SVG with CSS keyframes. Docs render from markdown files imported with Vite's `?raw` suffix and a thin manifest.

**Tech Stack:** React 18 · React Router 6 · Vite · Framer Motion · react-markdown + remark-gfm + rehype-highlight + highlight.js · Vitest + @testing-library/react (new) · Playwright (existing)

**Spec:** `docs/superpowers/specs/2026-04-12-landing-page-and-header-design.md`

---

## Pre-flight notes

- All work happens under `web/`. The CWD for npm/vitest commands is `web/`.
- Existing user info location to remove: `web/src/components/Sidebar.tsx:111-122` (`.sidebar-footer` block).
- Existing brand to remove from sidebar: `web/src/components/Sidebar.tsx:19-22` (`.sidebar-header` block) — moves into `SiteHeader`.
- Existing `/` route to replace: `web/src/App.tsx:39` (currently `<DefaultRedirect />`).
- `AuthProvider` wraps the entire `Routes` block in `App.tsx`, so `useAuth()` is safe inside `SiteHeader` even on the public landing page.
- The Sidebar already imports `useAuth` from `@/authHooks` (recent refactor) — follow that pattern for new code.

---

### Task 1: Install dependencies

**Files:**
- Modify: `web/package.json`
- Modify: `web/package-lock.json`

- [x] **Step 1: Install runtime deps**

```bash
cd web
npm install framer-motion react-markdown remark-gfm rehype-highlight highlight.js
```

- [x] **Step 2: Install Vitest + RTL dev deps**

The spec calls for unit tests on `UserMenu`. The existing project uses Vite, so Vitest (Vite-native, near drop-in for Jest) is the path of least friction.

```bash
cd web
npm install -D vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom @vitejs/plugin-react
```

(`@vitejs/plugin-react` is already a dep but reinstalling is harmless.)

- [x] **Step 3: Verify install**

```bash
cd web
npm ls framer-motion react-markdown vitest
```

Expected: each package shown with a version, no `UNMET DEPENDENCY` errors.

- [x] **Step 4: Commit**

```bash
git add web/package.json web/package-lock.json
git commit -m "build(web): add deps for landing page, docs, and unit testing"
```

---

### Task 2: Vitest config + smoke test

**Files:**
- Create: `web/vitest.config.ts`
- Create: `web/src/test/setup.ts`
- Create: `web/src/test/smoke.test.ts`
- Modify: `web/package.json` (scripts)
- Modify: `web/tsconfig.json` (types)

- [x] **Step 1: Write the failing smoke test**

`web/src/test/smoke.test.ts`:

```ts
import { describe, it, expect } from 'vitest';

describe('vitest smoke', () => {
  it('runs', () => {
    expect(1 + 1).toBe(2);
  });
});
```

- [x] **Step 2: Create the test setup file**

`web/src/test/setup.ts`:

```ts
import '@testing-library/jest-dom/vitest';
```

- [x] **Step 3: Create vitest config**

`web/vitest.config.ts`:

```ts
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
});
```

- [x] **Step 4: Add `test` and `test:watch` scripts**

In `web/package.json`, inside `"scripts"`, add:

```json
"test": "vitest run",
"test:watch": "vitest"
```

- [x] **Step 5: Add Vitest globals to tsconfig**

In `web/tsconfig.json` under `compilerOptions.types`, add `"vitest/globals"`. If the `types` array doesn't exist, add it:

```json
"types": ["vitest/globals", "@testing-library/jest-dom"]
```

- [x] **Step 6: Run the smoke test**

```bash
cd web
npm test
```

Expected: `1 passed`, exit 0.

- [x] **Step 7: Commit**

```bash
git add web/vitest.config.ts web/src/test/setup.ts web/src/test/smoke.test.ts web/package.json web/tsconfig.json
git commit -m "test(web): add Vitest + Testing Library setup with smoke test"
```

---

### Task 3: `UserMenu` component (TDD)

**Files:**
- Create: `web/src/components/UserMenu.tsx`
- Create: `web/src/components/UserMenu.test.tsx`

The `UserMenu` is a button-trigger dropdown showing the authed user's initials, with menu items for Settings and Logout. Closes on outside-click and Escape.

- [x] **Step 1: Write the failing tests**

`web/src/components/UserMenu.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import UserMenu from './UserMenu';

const mockLogout = vi.fn();
const mockNavigate = vi.fn();

vi.mock('@/authHooks', () => ({
  useAuth: () => ({
    user: { id: '1', email: 'jane.doe@example.com', name: 'Jane Doe' },
    loading: false,
    logout: mockLogout,
    login: vi.fn(),
    register: vi.fn(),
  }),
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({ orgSlug: 'acme' }),
  };
});

function renderMenu() {
  return render(
    <MemoryRouter>
      <UserMenu />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  mockLogout.mockReset();
  mockNavigate.mockReset();
  localStorage.clear();
});

describe('UserMenu', () => {
  it('renders initials from the user name', () => {
    renderMenu();
    expect(screen.getByRole('button', { name: /user menu/i })).toHaveTextContent('JD');
  });

  it('opens the dropdown on click and shows email', async () => {
    const user = userEvent.setup();
    renderMenu();
    await user.click(screen.getByRole('button', { name: /user menu/i }));
    expect(screen.getByText('jane.doe@example.com')).toBeInTheDocument();
    expect(screen.getByText('Settings')).toBeInTheDocument();
    expect(screen.getByText('Logout')).toBeInTheDocument();
  });

  it('closes on Escape', async () => {
    const user = userEvent.setup();
    renderMenu();
    await user.click(screen.getByRole('button', { name: /user menu/i }));
    expect(screen.getByText('Logout')).toBeInTheDocument();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByText('Logout')).not.toBeInTheDocument();
  });

  it('closes on outside click', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <div>
          <UserMenu />
          <div data-testid="outside">outside</div>
        </div>
      </MemoryRouter>,
    );
    await user.click(screen.getByRole('button', { name: /user menu/i }));
    expect(screen.getByText('Logout')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByTestId('outside'));
    expect(screen.queryByText('Logout')).not.toBeInTheDocument();
  });

  it('Settings link points to current org settings', async () => {
    const user = userEvent.setup();
    renderMenu();
    await user.click(screen.getByRole('button', { name: /user menu/i }));
    const settings = screen.getByText('Settings').closest('a');
    expect(settings).toHaveAttribute('href', '/orgs/acme/settings');
  });

  it('Logout calls logout and navigates to /', async () => {
    const user = userEvent.setup();
    renderMenu();
    await user.click(screen.getByRole('button', { name: /user menu/i }));
    await user.click(screen.getByText('Logout'));
    expect(mockLogout).toHaveBeenCalledOnce();
    expect(mockNavigate).toHaveBeenCalledWith('/');
  });
});
```

- [x] **Step 2: Run the tests to verify they fail**

```bash
cd web
npm test -- UserMenu
```

Expected: FAIL — `Cannot find module './UserMenu'`.

- [x] **Step 3: Implement `UserMenu`**

`web/src/components/UserMenu.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '@/authHooks';

function getInitials(user: { name?: string; email: string }): string {
  if (user.name && user.name.trim().length > 0) {
    const parts = user.name.trim().split(/\s+/);
    if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  }
  return user.email.slice(0, 2).toUpperCase();
}

export default function UserMenu() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const { orgSlug } = useParams();
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    function onMouseDown(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('keydown', onKey);
    document.addEventListener('mousedown', onMouseDown);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('mousedown', onMouseDown);
    };
  }, [open]);

  if (!user) return null;

  const settingsOrg = orgSlug || localStorage.getItem('ds_last_org') || '';
  const settingsHref = settingsOrg ? `/orgs/${settingsOrg}/settings` : '/portal';

  function handleLogout() {
    logout();
    setOpen(false);
    navigate('/');
  }

  return (
    <div className="user-menu" ref={containerRef}>
      <button
        type="button"
        className="user-menu-trigger"
        aria-label="User menu"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        {getInitials(user)}
      </button>
      {open && (
        <div className="user-menu-dropdown" role="menu">
          <div className="user-menu-header">
            <div className="user-menu-name">{user.name || user.email}</div>
            <div className="user-menu-email">{user.email}</div>
          </div>
          <Link to={settingsHref} className="user-menu-item" onClick={() => setOpen(false)}>
            Settings
          </Link>
          <button type="button" className="user-menu-item" onClick={handleLogout}>
            Logout
          </button>
        </div>
      )}
    </div>
  );
}
```

- [x] **Step 4: Add minimal CSS for the menu**

In `web/src/styles/globals.css`, append:

```css
/* User menu */
.user-menu { position: relative; }
.user-menu-trigger {
  width: 32px; height: 32px; border-radius: 50%;
  background: #1f2937; color: #e5e7eb;
  border: 1px solid #374151;
  font-size: 12px; font-weight: 600; letter-spacing: 0.5px;
  cursor: pointer; display: inline-flex; align-items: center; justify-content: center;
}
.user-menu-trigger:hover { background: #2a3441; }
.user-menu-dropdown {
  position: absolute; top: calc(100% + 8px); right: 0;
  min-width: 220px;
  background: #0f1419; border: 1px solid #1f2937; border-radius: 8px;
  box-shadow: 0 10px 30px rgba(0,0,0,0.4);
  z-index: 100; padding: 6px 0;
}
.user-menu-header { padding: 10px 14px; border-bottom: 1px solid #1f2937; }
.user-menu-name { color: #e5e7eb; font-size: 13px; font-weight: 600; }
.user-menu-email { color: #6b7280; font-size: 12px; margin-top: 2px; }
.user-menu-item {
  display: block; width: 100%; text-align: left;
  padding: 9px 14px; background: none; border: none;
  color: #e5e7eb; font-size: 13px; cursor: pointer;
  text-decoration: none; font-family: inherit;
}
.user-menu-item:hover { background: #1a1f2a; }
```

- [x] **Step 5: Run the tests to verify they pass**

```bash
cd web
npm test -- UserMenu
```

Expected: 6 tests passing, exit 0.

- [x] **Step 6: Commit**

```bash
git add web/src/components/UserMenu.tsx web/src/components/UserMenu.test.tsx web/src/styles/globals.css
git commit -m "feat(web): add UserMenu component with initials avatar and dropdown"
```

---

### Task 4: `SiteHeader` component

**Files:**
- Create: `web/src/components/SiteHeader.tsx`
- Modify: `web/src/styles/globals.css` (append header styles)

`SiteHeader` is presentational (no behavior beyond auth-aware right slot), so no unit tests — Playwright covers it in Task 18.

- [x] **Step 1: Implement `SiteHeader`**

`web/src/components/SiteHeader.tsx`:

```tsx
import { Link } from 'react-router-dom';
import { useAuth } from '@/authHooks';
import UserMenu from './UserMenu';

type SiteHeaderProps = {
  variant: 'landing' | 'app';
};

export default function SiteHeader({ variant }: SiteHeaderProps) {
  const { user } = useAuth();
  return (
    <header className="site-header">
      <Link to="/" className="site-header-brand" aria-label="DeploySentry home">
        <span className="site-header-logo">DS</span>
        <span className="site-header-wordmark">DeploySentry</span>
      </Link>

      {variant === 'landing' && (
        <nav className="site-header-nav">
          <a href="#pillars" className="site-header-link">Product</a>
          <Link to="/docs" className="site-header-link">Docs</Link>
          <Link to="/docs/sdks" className="site-header-link">SDKs</Link>
        </nav>
      )}

      <div className="site-header-right">
        {!user && variant === 'landing' && (
          <>
            <Link to="/login" className="site-header-link">Log in</Link>
            <Link to="/register" className="btn-primary site-header-cta">Sign up</Link>
          </>
        )}
        {user && variant === 'landing' && (
          <Link to="/portal" className="btn-primary site-header-cta">Portal</Link>
        )}
        {user && <UserMenu />}
      </div>
    </header>
  );
}
```

- [x] **Step 2: Append header styles**

In `web/src/styles/globals.css`, append:

```css
/* Site header */
.site-header {
  position: sticky; top: 0; z-index: 50;
  height: 60px;
  display: flex; align-items: center;
  padding: 0 24px;
  background: #0a0e14;
  border-bottom: 1px solid #1f2937;
  gap: 32px;
}
.site-header-brand {
  display: inline-flex; align-items: center; gap: 10px;
  color: #e5e7eb; text-decoration: none;
  font-weight: 600;
}
.site-header-logo {
  width: 28px; height: 28px; border-radius: 6px;
  background: linear-gradient(135deg, #3b82f6, #8b5cf6);
  color: #fff;
  display: inline-flex; align-items: center; justify-content: center;
  font-size: 11px; font-weight: 700; letter-spacing: 0.5px;
}
.site-header-wordmark { font-size: 15px; }
.site-header-nav { display: flex; gap: 24px; flex: 1; }
.site-header-link {
  color: #9ca3af; text-decoration: none; font-size: 13px;
  font-family: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
}
.site-header-link:hover { color: #e5e7eb; }
.site-header-right { margin-left: auto; display: flex; align-items: center; gap: 14px; }
.site-header-cta { font-size: 13px !important; padding: 7px 14px !important; }
```

- [x] **Step 3: Run linter to verify TS compiles**

```bash
cd web
npm run lint
```

Expected: zero errors related to `SiteHeader`/`UserMenu` files.

- [x] **Step 4: Commit**

```bash
git add web/src/components/SiteHeader.tsx web/src/styles/globals.css
git commit -m "feat(web): add shared SiteHeader with auth-aware right slot"
```

---

### Task 5: Wire `SiteHeader` into `HierarchyLayout` and remove sidebar user/brand

**Files:**
- Modify: `web/src/components/HierarchyLayout.tsx`
- Modify: `web/src/components/Sidebar.tsx`
- Modify: `web/src/styles/globals.css` (adjust `.app-layout` and `.sidebar`)

- [x] **Step 1: Render `SiteHeader` above the sidebar+content split**

Replace the entire return block in `web/src/components/HierarchyLayout.tsx` with:

```tsx
  return (
    <div className="app-shell">
      <SiteHeader variant="app" />
      <div className="app-layout">
        <Sidebar />
        <main className="main-content">
          <Breadcrumb />
          <Outlet />
        </main>
      </div>
    </div>
  );
```

Add the import at the top:

```tsx
import SiteHeader from './SiteHeader';
```

- [x] **Step 2: Remove the sidebar brand and user footer**

In `web/src/components/Sidebar.tsx`:

1. Delete the `.sidebar-header` block (the DS logo + wordmark) at the top of the `<aside>` return.
2. Delete the entire `.sidebar-footer` block at the bottom.
3. Delete the `handleLogout` function and the `logout`/`navigate` references that become unused.
4. The resulting top of `<aside>` becomes `<div className="sidebar-switchers">...</div>`.

After the edit, `Sidebar.tsx` should no longer call `useAuth()` or `useNavigate()`. Remove those imports.

- [x] **Step 3: Add `.app-shell` styles, adjust `.app-layout` to flex below the header**

In `web/src/styles/globals.css`, add and adjust:

```css
.app-shell { display: flex; flex-direction: column; min-height: 100vh; }
.app-shell .app-layout { flex: 1; min-height: 0; }
```

(Leave the existing `.app-layout` rule alone; this just constrains it inside the shell.)

- [x] **Step 4: Verify the dev server renders the new layout**

```bash
cd web
npm run dev
```

Open `http://localhost:3001` and log in. Verify:
- A 60px dark header is visible at the top of every authed page.
- The sidebar no longer shows the DS brand or user info at the bottom.
- The user menu in the top-right shows your initials and opens the dropdown.

Stop the dev server (Ctrl-C) before committing.

- [x] **Step 5: Run lint**

```bash
cd web
npm run lint
```

Expected: zero errors.

- [x] **Step 6: Commit**

```bash
git add web/src/components/HierarchyLayout.tsx web/src/components/Sidebar.tsx web/src/styles/globals.css
git commit -m "refactor(web): move user menu to shared SiteHeader, slim down sidebar"
```

---

### Task 6: `LandingPage` shell + `/` route + `/portal` route

**Files:**
- Create: `web/src/pages/LandingPage.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/pages/LoginPage.tsx`
- Modify: `web/src/styles/globals.css`

- [x] **Step 1: Create the landing page shell**

`web/src/pages/LandingPage.tsx`:

```tsx
import SiteHeader from '@/components/SiteHeader';

export default function LandingPage() {
  return (
    <div className="landing">
      <SiteHeader variant="landing" />
      <main className="landing-main">
        <section className="landing-placeholder">
          <h1>DeploySentry</h1>
          <p>Landing content coming in subsequent tasks.</p>
        </section>
      </main>
    </div>
  );
}
```

- [x] **Step 2: Wire routes in `App.tsx`**

In `web/src/App.tsx`:

1. Add the import: `import LandingPage from './pages/LandingPage';`
2. Move `/` out from under `RequireAuth`: replace the existing `<Route path="/" element={<DefaultRedirect />} />` with `<Route path="/" element={<LandingPage />} />` placed alongside the public routes (so it has no guard).
3. Add a new authed route under `RequireAuth`: `<Route path="/portal" element={<DefaultRedirect />} />`.

After the edit, the public-routes block looks like:

```tsx
<Route path="/" element={<LandingPage />} />
<Route element={<RedirectIfAuth />}>
  <Route path="/login" element={<LoginPage />} />
  <Route path="/register" element={<RegisterPage />} />
</Route>
```

And the authed block starts with:

```tsx
<Route element={<RequireAuth />}>
  <Route path="/portal" element={<DefaultRedirect />} />
  <Route path="/orgs/new" element={<CreateOrgPage />} />
  ...
```

(Delete the old `<Route path="/" element={<DefaultRedirect />} />` line.)

- [x] **Step 3: Update `LoginPage` to default to `/portal` instead of `/`**

In `web/src/pages/LoginPage.tsx:9`, change:

```ts
const from = (location.state as { from?: { pathname: string } })?.from?.pathname || '/';
```

to:

```ts
const from = (location.state as { from?: { pathname: string } })?.from?.pathname || '/portal';
```

- [x] **Step 4: Add minimal landing styles**

In `web/src/styles/globals.css`, append:

```css
/* Landing page */
.landing { background: #0a0e14; color: #e5e7eb; min-height: 100vh; }
.landing-main { max-width: 100%; }
.landing-placeholder {
  min-height: calc(100vh - 60px);
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 16px;
}
.landing-placeholder h1 { font-size: 48px; margin: 0; }
.landing-placeholder p { color: #6b7280; margin: 0; }
```

- [x] **Step 5: Verify routing**

```bash
cd web
npm run dev
```

Verify:
- `http://localhost:3001/` (logged out) shows the landing placeholder with `Log in` and `Sign up` in the header.
- Log in → URL becomes `/portal` and redirects through `DefaultRedirect` to your last org.
- Visit `/` again while logged in → still shows the landing page, with `Portal` button + user avatar in the header.
- Click the DS brand from inside the app → goes to `/`.

Stop dev server.

- [x] **Step 6: Commit**

```bash
git add web/src/pages/LandingPage.tsx web/src/App.tsx web/src/pages/LoginPage.tsx web/src/styles/globals.css
git commit -m "feat(web): add landing page shell at / and /portal redirect target"
```

---

### Task 7: Hero section

**Files:**
- Create: `web/src/components/landing/Hero.tsx`
- Modify: `web/src/pages/LandingPage.tsx`
- Modify: `web/src/styles/globals.css`

- [x] **Step 1: Implement the hero**

`web/src/components/landing/Hero.tsx`:

```tsx
import { motion } from 'framer-motion';
import { Link } from 'react-router-dom';

const fade = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0, transition: { duration: 0.5, ease: 'easeOut' } },
};

export default function Hero() {
  return (
    <section className="hero">
      <div className="hero-inner">
        <motion.div
          initial="hidden"
          animate="show"
          variants={{ show: { transition: { staggerChildren: 0.06 } } }}
        >
          <motion.div variants={fade} className="hero-eyebrow">FEATURE FLAG INFRASTRUCTURE</motion.div>
          <motion.h1 variants={fade} className="hero-headline">
            Ship code.<br />Release features.<br /><span className="hero-accent">Separately.</span>
          </motion.h1>
          <motion.p variants={fade} className="hero-sub">
            DeploySentry decouples deployment from release. Centralize every flag through the SDK so
            you always know where each one lives — and so an LLM can clean up the dead code when it's done.
          </motion.p>
          <motion.div variants={fade} className="hero-actions">
            <Link to="/register" className="btn-primary">Get started</Link>
            <Link to="/docs" className="btn-secondary">Read the docs</Link>
          </motion.div>
        </motion.div>
      </div>
    </section>
  );
}
```

- [x] **Step 2: Mount it in `LandingPage`**

Replace the placeholder section in `web/src/pages/LandingPage.tsx`:

```tsx
import SiteHeader from '@/components/SiteHeader';
import Hero from '@/components/landing/Hero';

export default function LandingPage() {
  return (
    <div className="landing">
      <SiteHeader variant="landing" />
      <main className="landing-main">
        <Hero />
      </main>
    </div>
  );
}
```

Delete the `.landing-placeholder` CSS block from `globals.css` since it's no longer used.

- [x] **Step 3: Add hero styles**

In `web/src/styles/globals.css`, append:

```css
/* Hero */
.hero {
  min-height: calc(100vh - 60px);
  display: flex; align-items: center;
  background: radial-gradient(ellipse 80% 50% at 50% -10%, rgba(59,130,246,0.12), transparent 60%);
  padding: 60px 24px;
}
.hero-inner { max-width: 900px; margin: 0 auto; text-align: center; }
.hero-eyebrow {
  font-family: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
  font-size: 12px; letter-spacing: 0.15em;
  color: #6b7280; margin-bottom: 24px;
}
.hero-headline {
  font-size: 64px; line-height: 1.05;
  margin: 0 0 24px 0; font-weight: 700;
  color: #f3f4f6; letter-spacing: -0.02em;
}
.hero-accent { background: linear-gradient(135deg, #3b82f6, #8b5cf6); -webkit-background-clip: text; background-clip: text; color: transparent; }
.hero-sub {
  font-size: 20px; line-height: 1.5;
  color: #9ca3af;
  max-width: 640px; margin: 0 auto 36px auto;
}
.hero-actions { display: flex; gap: 14px; justify-content: center; }
@media (max-width: 700px) {
  .hero-headline { font-size: 40px; }
  .hero-sub { font-size: 16px; }
}
```

- [x] **Step 4: Visual smoke**

```bash
cd web
npm run dev
```

Visit `http://localhost:3001/`. Verify the hero animates in (eyebrow → headline → sub → buttons), and that `Get started` and `Read the docs` buttons render. Stop dev server.

- [x] **Step 5: Commit**

```bash
git add web/src/components/landing/Hero.tsx web/src/pages/LandingPage.tsx web/src/styles/globals.css
git commit -m "feat(web): add landing page hero section with animated reveal"
```

---

### Task 8: `DeployReleaseFlow` animated SVG diagram

**Files:**
- Create: `web/src/components/landing/DeployReleaseFlow.tsx`
- Modify: `web/src/pages/LandingPage.tsx`
- Modify: `web/src/styles/globals.css`

This is the centerpiece visual. Pure SVG + CSS keyframes — no Framer Motion in this component.

- [x] **Step 1: Implement the diagram**

`web/src/components/landing/DeployReleaseFlow.tsx`:

```tsx
type Node = { x: number; label: string };

const DEPLOY: Node[] = [
  { x: 80,  label: 'commit' },
  { x: 240, label: 'build' },
  { x: 400, label: 'deploy' },
  { x: 560, label: 'live (dark)' },
];
const RELEASE: Node[] = [
  { x: 240, label: 'internal' },
  { x: 400, label: '5%' },
  { x: 560, label: '50%' },
  { x: 720, label: '100%' },
];

const TRACK_DEPLOY_Y = 80;
const TRACK_RELEASE_Y = 200;

function Track({
  nodes,
  y,
  startDelay,
  trackId,
}: {
  nodes: Node[];
  y: number;
  startDelay: number;
  trackId: string;
}) {
  return (
    <g>
      {nodes.slice(1).map((node, i) => {
        const prev = nodes[i];
        return (
          <line
            key={`${trackId}-line-${i}`}
            x1={prev.x}
            y1={y}
            x2={node.x}
            y2={y}
            className="flow-rail"
            style={{ animationDelay: `${startDelay + 0.2 + i * 0.4}s` }}
          />
        );
      })}
      {nodes.map((node, i) => (
        <g
          key={`${trackId}-node-${i}`}
          className="flow-node"
          style={{ animationDelay: `${startDelay + i * 0.4}s` }}
        >
          <circle cx={node.x} cy={y} r={7} />
          <text x={node.x} y={y - 18} textAnchor="middle">{node.label}</text>
        </g>
      ))}
    </g>
  );
}

export default function DeployReleaseFlow() {
  return (
    <section className="flow-section">
      <div className="flow-inner">
        <h2 className="section-heading">Deploy. Then release.</h2>
        <p className="section-sub">
          Engineering ships dark. Product opens the gate when they're ready. The two halves are
          decoupled, owned separately, and observable end-to-end.
        </p>
        <div className="flow-diagram-wrap">
          <svg
            className="flow-diagram"
            viewBox="0 0 820 280"
            xmlns="http://www.w3.org/2000/svg"
            role="img"
            aria-label="Deploy track followed by release track, animated"
          >
            <text x={20} y={40} className="flow-track-label">DEPLOY</text>
            <text x={760} y={40} className="flow-owner" textAnchor="end">engineering owns →</text>
            <Track nodes={DEPLOY} y={TRACK_DEPLOY_Y} startDelay={0} trackId="deploy" />

            <line
              x1={20} y1={140} x2={800} y2={140}
              className="flow-separator"
              style={{ animationDelay: '2.4s' }}
            />

            <text x={20} y={170} className="flow-track-label">RELEASE</text>
            <text x={760} y={170} className="flow-owner" textAnchor="end">product / on-call owns →</text>
            <Track nodes={RELEASE} y={TRACK_RELEASE_Y} startDelay={2.8} trackId="release" />
          </svg>
        </div>
      </div>
    </section>
  );
}
```

- [x] **Step 2: Add diagram styles + keyframes**

In `web/src/styles/globals.css`, append:

```css
/* Deploy/release flow diagram */
.flow-section { padding: 100px 24px; background: #0a0e14; }
.flow-inner { max-width: 1000px; margin: 0 auto; }
.section-heading {
  font-size: 40px; color: #f3f4f6; margin: 0 0 12px 0;
  text-align: center; letter-spacing: -0.02em;
}
.section-sub {
  font-size: 17px; color: #9ca3af; text-align: center;
  max-width: 600px; margin: 0 auto 60px auto; line-height: 1.5;
}
.flow-diagram-wrap { width: 100%; overflow-x: auto; }
.flow-diagram { width: 100%; max-width: 900px; height: auto; display: block; margin: 0 auto; }

.flow-track-label {
  font-family: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
  font-size: 11px; fill: #6b7280; letter-spacing: 0.15em;
}
.flow-owner {
  font-family: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
  font-size: 11px; fill: #4b5563;
}

.flow-rail {
  stroke: #3b82f6; stroke-width: 1.5; stroke-linecap: round;
  stroke-dasharray: 200; stroke-dashoffset: 200;
  animation: flowDraw 0.4s ease-out forwards, flowDim 1.5s ease-out 6.5s forwards;
}
@keyframes flowDraw {
  to { stroke-dashoffset: 0; }
}
@keyframes flowDim {
  to { opacity: 0.3; }
}

.flow-node circle { fill: #3b82f6; opacity: 0; transform-origin: center; }
.flow-node text { fill: #e5e7eb; font-size: 12px; opacity: 0; }
.flow-node {
  animation: flowNodePop 0.3s ease-out forwards, flowDim 1.5s ease-out 6.5s forwards;
}
@keyframes flowNodePop {
  from { opacity: 0; transform: translateY(4px); }
  to   { opacity: 1; transform: translateY(0); }
}
.flow-node circle {
  animation: flowNodeCircle 0.3s ease-out forwards;
  animation-delay: inherit;
}
.flow-node text {
  animation: flowNodeText 0.3s ease-out forwards;
  animation-delay: inherit;
}
@keyframes flowNodeCircle { to { opacity: 1; } }
@keyframes flowNodeText   { to { opacity: 1; } }

.flow-separator {
  stroke: #1f2937; stroke-width: 1; stroke-dasharray: 6 6;
  stroke-dashoffset: 800;
  animation: flowDraw 0.6s ease-out forwards;
  animation-delay: 2.4s;
}

@media (prefers-reduced-motion: reduce) {
  .flow-rail, .flow-node, .flow-separator,
  .flow-node circle, .flow-node text {
    animation: none !important;
    opacity: 1 !important;
    stroke-dashoffset: 0 !important;
  }
}
```

- [x] **Step 3: Mount it in `LandingPage` after the hero**

```tsx
import SiteHeader from '@/components/SiteHeader';
import Hero from '@/components/landing/Hero';
import DeployReleaseFlow from '@/components/landing/DeployReleaseFlow';

export default function LandingPage() {
  return (
    <div className="landing">
      <SiteHeader variant="landing" />
      <main className="landing-main">
        <Hero />
        <DeployReleaseFlow />
      </main>
    </div>
  );
}
```

- [x] **Step 4: Visual smoke**

```bash
cd web
npm run dev
```

Visit `/`, scroll past the hero, verify the deploy track animates first, then the release track. The diagram should fit at 1280px and 768px viewports.

- [x] **Step 5: Commit**

```bash
git add web/src/components/landing/DeployReleaseFlow.tsx web/src/pages/LandingPage.tsx web/src/styles/globals.css
git commit -m "feat(web): add animated deploy/release flow diagram to landing"
```

---

### Task 9: `PillarsSection` (three pillar cards)

**Files:**
- Create: `web/src/components/landing/PillarsSection.tsx`
- Modify: `web/src/pages/LandingPage.tsx`
- Modify: `web/src/styles/globals.css`

- [x] **Step 1: Implement pillars**

`web/src/components/landing/PillarsSection.tsx`:

```tsx
import { motion } from 'framer-motion';

type Pillar = {
  icon: string;
  label: string;
  title: string;
  body: string;
  tag?: string;
};

const PILLARS: Pillar[] = [
  {
    icon: '⇄',
    label: 'CONTROL',
    title: 'Decoupled deploys & releases',
    body: 'Ship code dark. Release on a flag flip. No rebuild, no rollback drama, no coupling between engineering and product schedules.',
  },
  {
    icon: '◎',
    label: 'METHODOLOGY',
    title: 'Centralized SDK dispatch',
    body: 'Register the functions a flag controls, then call ds.dispatch(flag, ctx) — the SDK picks the right one. Every call site is discoverable from one place.',
    tag: 'Recommended pattern',
  },
  {
    icon: '✻',
    label: 'CLEANUP',
    title: 'LLM-ready flag retirement',
    body: 'Because every flag has a registry entry pointing at its code, an LLM agent can confidently retire a flag and the dead code with it. No archaeology.',
  },
];

const fadeUp = {
  hidden: { opacity: 0, y: 24 },
  show: { opacity: 1, y: 0, transition: { duration: 0.5, ease: 'easeOut' } },
};

export default function PillarsSection() {
  return (
    <section className="pillars-section" id="pillars">
      <div className="pillars-inner">
        <h2 className="section-heading">Three controls, one model</h2>
        <p className="section-sub">
          DeploySentry is built around a small, opinionated set of primitives.
        </p>
        <motion.div
          className="pillar-grid"
          initial="hidden"
          whileInView="show"
          viewport={{ once: true, margin: '-100px' }}
          variants={{ show: { transition: { staggerChildren: 0.1 } } }}
        >
          {PILLARS.map((p) => (
            <motion.article key={p.title} className="pillar-card" variants={fadeUp}>
              <div className="pillar-icon" aria-hidden>{p.icon}</div>
              <div className="pillar-label">{p.label}</div>
              <h3 className="pillar-title">{p.title}</h3>
              <p className="pillar-body">{p.body}</p>
              {p.tag && <div className="pillar-tag">{p.tag}</div>}
            </motion.article>
          ))}
        </motion.div>
      </div>
    </section>
  );
}
```

- [x] **Step 2: Mount it after `DeployReleaseFlow`**

In `LandingPage.tsx`, import and place after the flow:

```tsx
import PillarsSection from '@/components/landing/PillarsSection';
// ...
<DeployReleaseFlow />
<PillarsSection />
```

- [x] **Step 3: Add pillar styles**

In `web/src/styles/globals.css`, append:

```css
/* Pillars */
.pillars-section { padding: 100px 24px; background: #070b10; border-top: 1px solid #1f2937; }
.pillars-inner { max-width: 1100px; margin: 0 auto; }
.pillar-grid {
  display: grid; grid-template-columns: repeat(3, 1fr); gap: 24px; margin-top: 60px;
}
.pillar-card {
  background: #0f1419; border: 1px solid #1f2937; border-radius: 10px;
  padding: 32px 28px; position: relative;
}
.pillar-icon {
  width: 40px; height: 40px; display: flex; align-items: center; justify-content: center;
  border-radius: 8px; background: rgba(59,130,246,0.1); color: #3b82f6;
  font-size: 22px; margin-bottom: 20px;
}
.pillar-label {
  font-family: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
  font-size: 11px; color: #6b7280; letter-spacing: 0.15em; margin-bottom: 8px;
}
.pillar-title { font-size: 20px; color: #f3f4f6; margin: 0 0 12px 0; }
.pillar-body { font-size: 14px; line-height: 1.6; color: #9ca3af; margin: 0; }
.pillar-tag {
  display: inline-block; margin-top: 18px;
  padding: 4px 10px; border-radius: 99px;
  background: rgba(139,92,246,0.12); color: #a78bfa;
  font-family: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
  font-size: 10px; letter-spacing: 0.1em;
}
@media (max-width: 900px) {
  .pillar-grid { grid-template-columns: 1fr; }
}
```

- [x] **Step 4: Visual smoke**

```bash
cd web
npm run dev
```

Verify three cards animate up on scroll-into-view. Test at narrow width (cards should stack).

- [x] **Step 5: Commit**

```bash
git add web/src/components/landing/PillarsSection.tsx web/src/pages/LandingPage.tsx web/src/styles/globals.css
git commit -m "feat(web): add three-pillar section to landing page"
```

---

### Task 10: Code-snippet contrast band

**Files:**
- Create: `web/src/components/landing/CodeContrast.tsx`
- Modify: `web/src/pages/LandingPage.tsx`
- Modify: `web/src/styles/globals.css`

- [x] **Step 1: Implement the contrast band**

`web/src/components/landing/CodeContrast.tsx`:

```tsx
const BEFORE = `// Before — direct call, flag check scattered
if (flags.isEnabled('new-checkout')) {
  newCheckout(ctx);
} else {
  oldCheckout(ctx);
}`;

const AFTER = `// After — centralized dispatch
ds.register('new-checkout', { on: newCheckout, off: oldCheckout });

ds.dispatch('new-checkout', ctx);`;

export default function CodeContrast() {
  return (
    <section className="code-contrast">
      <div className="code-contrast-inner">
        <h2 className="section-heading">One call site per flag</h2>
        <p className="section-sub">
          The SDK becomes the single source of truth for which function runs and when.
        </p>
        <div className="code-pair">
          <pre className="code-block code-before"><code>{BEFORE}</code></pre>
          <pre className="code-block code-after"><code>{AFTER}</code></pre>
        </div>
      </div>
    </section>
  );
}
```

- [x] **Step 2: Mount after `PillarsSection`**

```tsx
import CodeContrast from '@/components/landing/CodeContrast';
// ...
<PillarsSection />
<CodeContrast />
```

- [x] **Step 3: Add styles**

```css
/* Code contrast */
.code-contrast { padding: 100px 24px; background: #0a0e14; }
.code-contrast-inner { max-width: 1100px; margin: 0 auto; }
.code-pair {
  display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-top: 50px;
}
.code-block {
  background: #0f1419; border: 1px solid #1f2937; border-radius: 8px;
  padding: 22px 24px; margin: 0;
  font-family: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
  font-size: 13px; line-height: 1.6; color: #e5e7eb;
  overflow-x: auto;
}
.code-after { border-color: #3b82f6; box-shadow: 0 0 0 1px rgba(59,130,246,0.2); }
@media (max-width: 800px) {
  .code-pair { grid-template-columns: 1fr; }
}
```

- [x] **Step 4: Visual smoke**

`npm run dev`, scroll past pillars, verify the two side-by-side code blocks render. Stop server.

- [x] **Step 5: Commit**

```bash
git add web/src/components/landing/CodeContrast.tsx web/src/pages/LandingPage.tsx web/src/styles/globals.css
git commit -m "feat(web): add before/after code contrast band to landing page"
```

---

### Task 11: End-to-end lifecycle strip

**Files:**
- Create: `web/src/components/landing/LifecycleStrip.tsx`
- Modify: `web/src/pages/LandingPage.tsx`
- Modify: `web/src/styles/globals.css`

- [x] **Step 1: Implement the strip**

`web/src/components/landing/LifecycleStrip.tsx`:

```tsx
import { motion } from 'framer-motion';

const STEPS = [
  { n: '01', title: 'Develop', body: 'Write the new code path behind a flag.' },
  { n: '02', title: 'Ship dark', body: 'Deploy to prod with the flag off.' },
  { n: '03', title: 'Targeted release', body: 'Open the gate for internal, then 5%, then 100%.' },
  { n: '04', title: 'Observe', body: 'Watch metrics and errors per cohort.' },
  { n: '05', title: 'Retire flag', body: 'Mark the flag complete in the registry.' },
  { n: '06', title: 'LLM cleans code', body: 'An agent prunes the dead branch from the source.' },
];

const fadeRight = {
  hidden: { opacity: 0, x: -16 },
  show: { opacity: 1, x: 0, transition: { duration: 0.4, ease: 'easeOut' } },
};

export default function LifecycleStrip() {
  return (
    <section className="lifecycle">
      <div className="lifecycle-inner">
        <h2 className="section-heading">From idea to retirement</h2>
        <p className="section-sub">
          The full lifecycle, in six steps, on one page.
        </p>
        <motion.div
          className="lifecycle-steps"
          initial="hidden"
          whileInView="show"
          viewport={{ once: true, margin: '-80px' }}
          variants={{ show: { transition: { staggerChildren: 0.08 } } }}
        >
          {STEPS.map((s) => (
            <motion.div key={s.n} className="lifecycle-step" variants={fadeRight}>
              <div className="lifecycle-num">{s.n}</div>
              <div className="lifecycle-title">{s.title}</div>
              <div className="lifecycle-body">{s.body}</div>
            </motion.div>
          ))}
        </motion.div>
      </div>
    </section>
  );
}
```

- [x] **Step 2: Mount after `CodeContrast`**

```tsx
import LifecycleStrip from '@/components/landing/LifecycleStrip';
// ...
<CodeContrast />
<LifecycleStrip />
```

- [x] **Step 3: Add styles**

```css
/* Lifecycle */
.lifecycle { padding: 100px 24px; background: #070b10; border-top: 1px solid #1f2937; }
.lifecycle-inner { max-width: 1200px; margin: 0 auto; }
.lifecycle-steps {
  display: grid; grid-template-columns: repeat(6, 1fr); gap: 16px; margin-top: 60px;
}
.lifecycle-step {
  background: #0f1419; border: 1px solid #1f2937; border-radius: 8px;
  padding: 20px 18px;
}
.lifecycle-num {
  font-family: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
  font-size: 11px; color: #3b82f6; margin-bottom: 12px; letter-spacing: 0.1em;
}
.lifecycle-title { font-size: 15px; color: #f3f4f6; font-weight: 600; margin-bottom: 8px; }
.lifecycle-body { font-size: 12px; color: #9ca3af; line-height: 1.5; }
@media (max-width: 1100px) { .lifecycle-steps { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 700px)  { .lifecycle-steps { grid-template-columns: repeat(2, 1fr); } }
```

- [x] **Step 4: Visual smoke**

`npm run dev`, scroll, verify six step cards animate in left-to-right.

- [x] **Step 5: Commit**

```bash
git add web/src/components/landing/LifecycleStrip.tsx web/src/pages/LandingPage.tsx web/src/styles/globals.css
git commit -m "feat(web): add end-to-end lifecycle strip to landing page"
```

---

### Task 12: CTA section + Footer

**Files:**
- Create: `web/src/components/landing/CTASection.tsx`
- Create: `web/src/components/landing/Footer.tsx`
- Modify: `web/src/pages/LandingPage.tsx`
- Modify: `web/src/styles/globals.css`

- [x] **Step 1: CTA section**

`web/src/components/landing/CTASection.tsx`:

```tsx
import { Link } from 'react-router-dom';

export default function CTASection() {
  return (
    <section className="cta-section">
      <div className="cta-inner">
        <h2 className="cta-headline">Stop coupling deploys to releases.</h2>
        <Link to="/register" className="btn-primary cta-button">Get started for free</Link>
        <Link to="/docs" className="cta-secondary">or read the docs →</Link>
      </div>
    </section>
  );
}
```

- [x] **Step 2: Footer**

`web/src/components/landing/Footer.tsx`:

```tsx
import { Link } from 'react-router-dom';

export default function Footer() {
  return (
    <footer className="landing-footer">
      <div className="landing-footer-inner">
        <div className="landing-footer-col">
          <div className="landing-footer-heading">Product</div>
          <a href="#pillars">Features</a>
          <Link to="/docs/sdks">SDKs</Link>
          <Link to="/docs/cli">CLI</Link>
        </div>
        <div className="landing-footer-col">
          <div className="landing-footer-heading">Docs</div>
          <Link to="/docs/getting-started">Getting started</Link>
          <Link to="/docs/sdks">SDK reference</Link>
          <Link to="/docs/cli">CLI reference</Link>
        </div>
        <div className="landing-footer-col">
          <div className="landing-footer-heading">Project</div>
          <a href="https://github.com/shadsorg/DeploySentry" target="_blank" rel="noreferrer">GitHub</a>
        </div>
      </div>
      <div className="landing-footer-bottom">
        <span className="site-header-logo">DS</span>
        <span>© DeploySentry · v1.0.0</span>
      </div>
    </footer>
  );
}
```

- [x] **Step 3: Mount both**

`LandingPage.tsx` final shape:

```tsx
import SiteHeader from '@/components/SiteHeader';
import Hero from '@/components/landing/Hero';
import DeployReleaseFlow from '@/components/landing/DeployReleaseFlow';
import PillarsSection from '@/components/landing/PillarsSection';
import CodeContrast from '@/components/landing/CodeContrast';
import LifecycleStrip from '@/components/landing/LifecycleStrip';
import CTASection from '@/components/landing/CTASection';
import Footer from '@/components/landing/Footer';

export default function LandingPage() {
  return (
    <div className="landing">
      <SiteHeader variant="landing" />
      <main className="landing-main">
        <Hero />
        <DeployReleaseFlow />
        <PillarsSection />
        <CodeContrast />
        <LifecycleStrip />
        <CTASection />
      </main>
      <Footer />
    </div>
  );
}
```

- [x] **Step 4: Add styles**

```css
/* CTA */
.cta-section { padding: 120px 24px; background: #0a0e14; text-align: center; }
.cta-inner { max-width: 700px; margin: 0 auto; }
.cta-headline {
  font-size: 44px; color: #f3f4f6; margin: 0 0 32px 0;
  letter-spacing: -0.02em;
}
.cta-button { font-size: 15px !important; padding: 12px 24px !important; }
.cta-secondary {
  display: block; margin-top: 18px;
  color: #6b7280; text-decoration: none; font-size: 13px;
}
.cta-secondary:hover { color: #e5e7eb; }

/* Landing footer */
.landing-footer { background: #070b10; border-top: 1px solid #1f2937; padding: 60px 24px 40px 24px; }
.landing-footer-inner {
  max-width: 1100px; margin: 0 auto;
  display: grid; grid-template-columns: repeat(3, 1fr); gap: 40px;
}
.landing-footer-col { display: flex; flex-direction: column; gap: 10px; }
.landing-footer-heading {
  font-family: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
  font-size: 11px; color: #6b7280; letter-spacing: 0.15em; margin-bottom: 6px;
}
.landing-footer-col a {
  color: #9ca3af; text-decoration: none; font-size: 13px;
}
.landing-footer-col a:hover { color: #e5e7eb; }
.landing-footer-bottom {
  max-width: 1100px; margin: 40px auto 0 auto;
  padding-top: 24px; border-top: 1px solid #1f2937;
  display: flex; align-items: center; gap: 12px;
  color: #4b5563; font-size: 12px;
}
@media (max-width: 700px) {
  .landing-footer-inner { grid-template-columns: 1fr; }
  .cta-headline { font-size: 30px; }
}
```

- [x] **Step 5: Visual smoke**

`npm run dev`, scroll all the way down, verify CTA + footer render and links are clickable. Stop server.

- [x] **Step 6: Commit**

```bash
git add web/src/components/landing/CTASection.tsx web/src/components/landing/Footer.tsx web/src/pages/LandingPage.tsx web/src/styles/globals.css
git commit -m "feat(web): add CTA section and footer to complete landing page"
```

---

### Task 13: `MarkdownRenderer` component

**Files:**
- Create: `web/src/components/docs/MarkdownRenderer.tsx`
- Modify: `web/src/styles/globals.css`

- [x] **Step 1: Register highlight.js languages**

We register only the languages we need to keep the bundle smaller. Add an init module:

`web/src/components/docs/highlight-languages.ts`:

```ts
import hljs from 'highlight.js/lib/core';
import typescript from 'highlight.js/lib/languages/typescript';
import javascript from 'highlight.js/lib/languages/javascript';
import bash from 'highlight.js/lib/languages/bash';
import go from 'highlight.js/lib/languages/go';
import python from 'highlight.js/lib/languages/python';
import java from 'highlight.js/lib/languages/java';
import ruby from 'highlight.js/lib/languages/ruby';
import dart from 'highlight.js/lib/languages/dart';
import json from 'highlight.js/lib/languages/json';
import yaml from 'highlight.js/lib/languages/yaml';

hljs.registerLanguage('typescript', typescript);
hljs.registerLanguage('ts', typescript);
hljs.registerLanguage('javascript', javascript);
hljs.registerLanguage('js', javascript);
hljs.registerLanguage('bash', bash);
hljs.registerLanguage('sh', bash);
hljs.registerLanguage('go', go);
hljs.registerLanguage('python', python);
hljs.registerLanguage('py', python);
hljs.registerLanguage('java', java);
hljs.registerLanguage('ruby', ruby);
hljs.registerLanguage('rb', ruby);
hljs.registerLanguage('dart', dart);
hljs.registerLanguage('json', json);
hljs.registerLanguage('yaml', yaml);
hljs.registerLanguage('yml', yaml);

export { hljs };
```

- [x] **Step 2: Implement the renderer**

`web/src/components/docs/MarkdownRenderer.tsx`:

```tsx
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import { Link } from 'react-router-dom';
import './highlight-languages';

type Props = { source: string };

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/(^-|-$)/g, '');
}

function headingId(children: React.ReactNode): string | undefined {
  if (typeof children === 'string') return slugify(children);
  if (Array.isArray(children)) {
    const first = children.find((c) => typeof c === 'string');
    if (typeof first === 'string') return slugify(first);
  }
  return undefined;
}

export default function MarkdownRenderer({ source }: Props) {
  return (
    <div className="markdown-body">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[[rehypeHighlight, { detect: true, ignoreMissing: true }]]}
        components={{
          h1: ({ children }) => <h1 id={headingId(children)}>{children}</h1>,
          h2: ({ children }) => <h2 id={headingId(children)}>{children}</h2>,
          h3: ({ children }) => <h3 id={headingId(children)}>{children}</h3>,
          a: ({ href, children }) => {
            if (href && href.startsWith('/')) {
              return <Link to={href}>{children}</Link>;
            }
            return <a href={href} target="_blank" rel="noreferrer">{children}</a>;
          },
        }}
      >
        {source}
      </ReactMarkdown>
    </div>
  );
}
```

- [x] **Step 3: Add markdown styles + import highlight.js theme**

In `web/src/styles/globals.css`, append:

```css
/* Markdown body */
.markdown-body { color: #e5e7eb; font-size: 15px; line-height: 1.7; max-width: 760px; }
.markdown-body h1 { font-size: 32px; margin: 0 0 16px 0; color: #f3f4f6; }
.markdown-body h2 { font-size: 24px; margin: 40px 0 14px 0; color: #f3f4f6; border-top: 1px solid #1f2937; padding-top: 32px; }
.markdown-body h3 { font-size: 18px; margin: 28px 0 10px 0; color: #f3f4f6; }
.markdown-body p { margin: 0 0 16px 0; color: #d1d5db; }
.markdown-body a { color: #3b82f6; text-decoration: none; }
.markdown-body a:hover { text-decoration: underline; }
.markdown-body ul, .markdown-body ol { margin: 0 0 16px 0; padding-left: 24px; color: #d1d5db; }
.markdown-body li { margin: 4px 0; }
.markdown-body code {
  font-family: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
  font-size: 13px; padding: 2px 6px;
  background: #1a1f2a; color: #e5e7eb; border-radius: 4px;
}
.markdown-body pre {
  background: #0f1419; border: 1px solid #1f2937; border-radius: 8px;
  padding: 18px 20px; overflow-x: auto; margin: 0 0 20px 0;
}
.markdown-body pre code { background: none; padding: 0; font-size: 13px; line-height: 1.6; }
.markdown-body table { width: 100%; border-collapse: collapse; margin: 0 0 20px 0; }
.markdown-body th, .markdown-body td {
  border: 1px solid #1f2937; padding: 8px 12px; text-align: left; font-size: 14px;
}
.markdown-body th { background: #0f1419; color: #f3f4f6; }
```

- [x] **Step 4: Import the highlight.js theme CSS**

In `web/src/main.tsx` (or wherever `globals.css` is imported), add:

```ts
import 'highlight.js/styles/github-dark.css';
```

- [x] **Step 5: Lint**

```bash
cd web
npm run lint
```

Expected: zero errors related to the new files.

- [x] **Step 6: Commit**

```bash
git add web/src/components/docs/MarkdownRenderer.tsx web/src/components/docs/highlight-languages.ts web/src/styles/globals.css web/src/main.tsx
git commit -m "feat(web): add MarkdownRenderer with syntax-highlighted code blocks"
```

---

### Task 14: Docs content stubs + manifest

**Files:**
- Create: `web/src/docs/getting-started.md`
- Create: `web/src/docs/sdks.md`
- Create: `web/src/docs/cli.md`
- Create: `web/src/docs/ui-features.md`
- Create: `web/src/docs/index.ts`
- Modify: `web/vite-env.d.ts` or `web/src/vite-env.d.ts` (declare `?raw` imports)

- [x] **Step 1: Declare `?raw` markdown imports for TypeScript**

In `web/src/vite-env.d.ts`, append:

```ts
declare module '*.md?raw' {
  const content: string;
  export default content;
}
```

- [x] **Step 2: Create `getting-started.md`**

`web/src/docs/getting-started.md`:

```markdown
# Getting Started

DeploySentry decouples deployment from release. This guide walks you from a fresh install to your first feature flag in production.

## Install

Self-host with Docker Compose:

```bash
git clone https://github.com/shadsorg/DeploySentry.git
cd DeploySentry
make dev-up
make migrate-up
make run-api
```

The API listens on `:8080` and the dashboard on `:3001`.

## Create your organization

1. Open http://localhost:3001
2. Sign up for an account
3. Create your first organization

## Create a project and application

Inside your organization, create a project and an application. Projects group related work; applications are the deployable units inside a project.

## Create your first flag

From the project's Flags page, click **New Flag**. Pick a category (release, feature, experiment, ops, or permission) and define a key.

## Wire up an SDK

Pick the SDK that matches your stack (see [SDKs](/docs/sdks)) and follow the init pattern. The minimum is one API key and the flag key you just created.
```

- [x] **Step 3: Create `sdks.md`**

`web/src/docs/sdks.md`:

```markdown
# SDKs

DeploySentry ships seven first-party SDKs. Each follows the same shape: instantiate a client, evaluate flags, and (optionally) register dispatch handlers.

| Language | Package | Status |
|---|---|---|
| Go | `github.com/shadsorg/deploysentry-go` | Stable |
| Node | `@deploysentry/node` | Stable |
| Python | `deploysentry` | Stable |
| Java | `io.deploysentry:deploysentry` | Stable |
| Ruby | `deploysentry` | Stable |
| React | `@deploysentry/react` | Stable |
| Flutter | `deploysentry` | Stable |

## Node

```ts
import { DeploySentry } from '@deploysentry/node';

const ds = new DeploySentry({ apiKey: process.env.DS_API_KEY! });
const isOn = await ds.isEnabled('new-checkout', { userId: '42' });
```

## Go

```go
client := deploysentry.New(deploysentry.Options{APIKey: os.Getenv("DS_API_KEY")})
on, _ := client.IsEnabled(ctx, "new-checkout", map[string]any{"userId": "42"})
```

## Python

```python
from deploysentry import DeploySentry

ds = DeploySentry(api_key=os.environ["DS_API_KEY"])
on = ds.is_enabled("new-checkout", {"user_id": "42"})
```

See each SDK's README in the [GitHub repo](https://github.com/shadsorg/DeploySentry/tree/main/sdk) for the full reference.
```

- [x] **Step 4: Create `cli.md`**

`web/src/docs/cli.md`:

```markdown
# CLI

The `deploysentry` CLI manages organizations, projects, applications, deployments, releases, and flags from the terminal.

## Install

```bash
go install github.com/shadsorg/DeploySentry/cmd/cli@latest
```

## Authenticate

```bash
deploysentry auth login
```

## Common commands

| Command | What it does |
|---|---|
| `deploysentry orgs list` | List organizations you belong to |
| `deploysentry projects list` | List projects in the current org |
| `deploysentry apps list` | List applications in the current project |
| `deploysentry flags list` | List flags |
| `deploysentry flags create <key>` | Create a flag |
| `deploysentry deploy create` | Trigger a deployment |
| `deploysentry releases list` | List releases |
| `deploysentry apikeys create` | Create an API key |

## Examples

```bash
# List flags in the current project
deploysentry flags list

# Create a release flag
deploysentry flags create new-checkout --category release --expires 2026-09-01

# Toggle a flag for the production environment
deploysentry flags set new-checkout --env production --on
```
```

- [x] **Step 5: Create `ui-features.md`**

`web/src/docs/ui-features.md`:

```markdown
# UI Features

A tour of every page in the DeploySentry dashboard.

## Projects

Lists every project in the current organization. Click a project to open its applications, flags, and analytics.

## Applications

Each project contains one or more applications — the deployable units. Each app has its own deployments, releases, and flags.

## Feature Flags

The flag list shows every flag for the current scope (project or app). Each flag has a key, category (release/feature/experiment/ops/permission), targeting rules, and rollout status.

## Flag Detail

Edit targeting rules, view evaluation history, and toggle the flag per environment.

## Deployments

Lists deployments for the current application. Each deployment links to its release record and source commit.

## Releases

Releases are independent of deployments. A release is a flag rollout — opening the gate to a cohort.

## Members

Manage organization members and their roles (owner, admin, member, viewer).

## API Keys

Create and revoke API keys scoped to the current organization.

## Settings

Hierarchical settings: org > project > app > environment. Lower levels inherit from higher levels unless overridden.

## Analytics

Per-flag evaluation counts, error rates, and rollout health.

## SDKs

Quickstart snippets for every SDK, prefilled with your API key.
```

- [x] **Step 6: Create the manifest**

`web/src/docs/index.ts`:

```ts
import gettingStarted from './getting-started.md?raw';
import sdks from './sdks.md?raw';
import cli from './cli.md?raw';
import uiFeatures from './ui-features.md?raw';

export type DocEntry = {
  slug: string;
  title: string;
  source: string;
};

export const docsManifest: readonly DocEntry[] = [
  { slug: 'getting-started', title: 'Getting Started', source: gettingStarted },
  { slug: 'sdks',            title: 'SDKs',            source: sdks },
  { slug: 'cli',             title: 'CLI',             source: cli },
  { slug: 'ui-features',     title: 'UI Features',     source: uiFeatures },
] as const;

export function findDoc(slug: string): DocEntry | undefined {
  return docsManifest.find((d) => d.slug === slug);
}
```

- [x] **Step 7: Lint to verify the `?raw` declarations work**

```bash
cd web
npm run lint
```

Expected: zero errors.

- [x] **Step 8: Commit**

```bash
git add web/src/docs/ web/src/vite-env.d.ts
git commit -m "docs(web): add stub markdown content and manifest for in-app docs"
```

---

### Task 15: `DocsPage` + `DocsSidebar` + routes

**Files:**
- Create: `web/src/components/docs/DocsSidebar.tsx`
- Create: `web/src/pages/DocsPage.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/styles/globals.css`

- [x] **Step 1: Implement `DocsSidebar`**

`web/src/components/docs/DocsSidebar.tsx`:

```tsx
import { NavLink } from 'react-router-dom';
import { docsManifest } from '@/docs';

export default function DocsSidebar() {
  return (
    <aside className="docs-sidebar">
      <div className="docs-sidebar-heading">DOCUMENTATION</div>
      <nav className="docs-sidebar-nav">
        {docsManifest.map((doc) => (
          <NavLink
            key={doc.slug}
            to={`/docs/${doc.slug}`}
            className={({ isActive }) => `docs-sidebar-link${isActive ? ' active' : ''}`}
          >
            {doc.title}
          </NavLink>
        ))}
      </nav>
    </aside>
  );
}
```

- [x] **Step 2: Implement `DocsPage`**

`web/src/pages/DocsPage.tsx`:

```tsx
import { useParams, Navigate } from 'react-router-dom';
import SiteHeader from '@/components/SiteHeader';
import DocsSidebar from '@/components/docs/DocsSidebar';
import MarkdownRenderer from '@/components/docs/MarkdownRenderer';
import { findDoc, docsManifest } from '@/docs';

export default function DocsPage() {
  const { slug } = useParams();

  if (!slug) {
    return <Navigate to={`/docs/${docsManifest[0].slug}`} replace />;
  }

  const doc = findDoc(slug);
  if (!doc) {
    return <Navigate to={`/docs/${docsManifest[0].slug}`} replace />;
  }

  return (
    <div className="docs-shell">
      <SiteHeader variant="app" />
      <div className="docs-layout">
        <DocsSidebar />
        <main className="docs-content">
          <MarkdownRenderer source={doc.source} />
        </main>
      </div>
    </div>
  );
}
```

- [x] **Step 3: Wire docs routes in `App.tsx`**

Inside the `<RequireAuth />` block, add (just after `/portal`):

```tsx
<Route path="/docs" element={<DocsPage />} />
<Route path="/docs/:slug" element={<DocsPage />} />
```

And import at the top:

```tsx
import DocsPage from './pages/DocsPage';
```

- [x] **Step 4: Add docs styles**

In `web/src/styles/globals.css`, append:

```css
/* Docs */
.docs-shell { display: flex; flex-direction: column; min-height: 100vh; background: #0a0e14; }
.docs-layout { display: flex; flex: 1; min-height: 0; }
.docs-sidebar {
  width: 240px; flex-shrink: 0;
  background: #070b10; border-right: 1px solid #1f2937;
  padding: 32px 16px;
}
.docs-sidebar-heading {
  font-family: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
  font-size: 11px; color: #6b7280; letter-spacing: 0.15em;
  padding: 0 12px 12px 12px;
}
.docs-sidebar-nav { display: flex; flex-direction: column; gap: 2px; }
.docs-sidebar-link {
  display: block; padding: 8px 12px; border-radius: 6px;
  color: #9ca3af; text-decoration: none; font-size: 13px;
}
.docs-sidebar-link:hover { background: #0f1419; color: #e5e7eb; }
.docs-sidebar-link.active { background: #1a1f2a; color: #f3f4f6; }
.docs-content { flex: 1; padding: 60px 80px; overflow-y: auto; }
@media (max-width: 800px) {
  .docs-layout { flex-direction: column; }
  .docs-sidebar { width: auto; border-right: none; border-bottom: 1px solid #1f2937; padding: 16px; }
  .docs-content { padding: 32px 24px; }
}
```

- [x] **Step 5: Visual smoke**

```bash
cd web
npm run dev
```

While logged in, visit `/docs`. Verify:
- Auto-redirects to `/docs/getting-started`.
- Sidebar shows four entries; active one is highlighted.
- Markdown renders with headings, bullet lists, tables, and syntax-highlighted code blocks.
- Clicking sidebar entries swaps the content without a full page reload.

Stop server.

- [x] **Step 6: Commit**

```bash
git add web/src/components/docs/DocsSidebar.tsx web/src/pages/DocsPage.tsx web/src/App.tsx web/src/styles/globals.css
git commit -m "feat(web): add /docs route with markdown renderer and sidebar nav"
```

---

### Task 16: Sidebar Documentation link

**Files:**
- Modify: `web/src/components/Sidebar.tsx`

- [x] **Step 1: Add the Help section + Documentation link**

Inside the `<nav className="sidebar-nav">` block in `web/src/components/Sidebar.tsx`, just before the closing `</nav>`, add:

```tsx
<div className="sidebar-section">Help</div>
<NavLink
  to="/docs"
  className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
>
  <span className="nav-icon">?</span>
  Documentation
</NavLink>
```

The Help link should be visible on every authed view, so place it after the `projectSlug && orgSlug` block (outside its conditional).

- [x] **Step 2: Visual smoke**

`npm run dev`. Verify the Documentation link appears in the sidebar at every level (no project, project selected, app selected) and routes to `/docs`.

- [x] **Step 3: Commit**

```bash
git add web/src/components/Sidebar.tsx
git commit -m "feat(web): add Documentation link to sidebar Help section"
```

---

### Task 17: Lazy-load `LandingPage` and `DocsPage`

**Files:**
- Modify: `web/src/App.tsx`

- [x] **Step 1: Convert imports to `React.lazy`**

At the top of `web/src/App.tsx`, replace:

```tsx
import LandingPage from './pages/LandingPage';
import DocsPage from './pages/DocsPage';
```

with:

```tsx
import { lazy, Suspense } from 'react';
const LandingPage = lazy(() => import('./pages/LandingPage'));
const DocsPage = lazy(() => import('./pages/DocsPage'));
```

- [x] **Step 2: Wrap the `<Routes>` (or just the lazy routes) in `<Suspense>`**

The simplest correct change is to wrap the entire `<Routes>` element:

```tsx
<Suspense fallback={<div className="page-loading">Loading...</div>}>
  <Routes>
    {/* ... existing routes ... */}
  </Routes>
</Suspense>
```

- [x] **Step 3: Build to verify code-splitting works**

```bash
cd web
npm run build
```

Expected: build succeeds. The `dist/assets/` directory should now contain separate chunks for `LandingPage-*.js` and `DocsPage-*.js`.

```bash
ls dist/assets/ | grep -E '(Landing|Docs)'
```

Expected: at least one matching file per page.

- [x] **Step 4: Smoke**

`npm run dev`. Verify `/`, `/docs`, and the authed app shell still all render.

- [x] **Step 5: Commit**

```bash
git add web/src/App.tsx
git commit -m "perf(web): lazy-load LandingPage and DocsPage chunks"
```

---

### Task 18: Playwright E2E spec

**Files:**
- Create: `web/e2e/ui/landing-and-header.spec.ts`

This spec covers the unauthed landing page, the authed header behavior, and docs navigation. It uses the existing `mock-api.ts` helper for the authed flow.

- [x] **Step 1: Inspect existing helpers to match the project's mocking pattern**

```bash
cd web
sed -n '1,40p' e2e/helpers/mock-api.ts
```

Look at how other UI specs (e.g. `e2e/ui/api-keys.spec.ts`) set up an authenticated session. Match that pattern in step 2.

- [x] **Step 2: Write the spec**

`web/e2e/ui/landing-and-header.spec.ts`:

```ts
import { test, expect } from '@playwright/test';
import { setupMockApi, loginAsTestUser } from '../helpers/mock-api';

test.describe('Landing page (unauthed)', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockApi(page);
  });

  test('renders hero and log-in link', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText(/Ship code\.\s*Release features\./i)).toBeVisible();
    await expect(page.getByRole('link', { name: 'Log in' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Sign up' })).toBeVisible();
  });

  test('Log in link navigates to /login', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('link', { name: 'Log in' }).click();
    await expect(page).toHaveURL(/\/login$/);
  });
});

test.describe('Landing page and header (authed)', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockApi(page);
    await loginAsTestUser(page);
  });

  test('shows Portal button and user menu when authed', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Portal' })).toBeVisible();
    await expect(page.getByRole('button', { name: /user menu/i })).toBeVisible();
  });

  test('clicking the DS brand from inside the app returns to landing', async ({ page }) => {
    await page.goto('/portal');
    await page.waitForURL(/\/orgs\//);
    await page.getByRole('link', { name: /DeploySentry home/i }).click();
    await expect(page).toHaveURL('/');
    await expect(page.getByText(/Ship code\./i)).toBeVisible();
  });

  test('user menu opens, shows initials, and logout works', async ({ page }) => {
    await page.goto('/');
    const trigger = page.getByRole('button', { name: /user menu/i });
    await expect(trigger).toBeVisible();
    await trigger.click();
    await expect(page.getByText('Logout')).toBeVisible();
    await page.getByText('Logout').click();
    await expect(page).toHaveURL('/');
    await expect(page.getByRole('link', { name: 'Log in' })).toBeVisible();
  });
});

test.describe('Docs', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockApi(page);
    await loginAsTestUser(page);
  });

  test('/docs redirects to getting-started and renders content', async ({ page }) => {
    await page.goto('/docs');
    await expect(page).toHaveURL(/\/docs\/getting-started$/);
    await expect(page.getByRole('heading', { name: 'Getting Started' })).toBeVisible();
  });

  test('sidebar nav switches docs pages', async ({ page }) => {
    await page.goto('/docs');
    await page.getByRole('link', { name: 'SDKs' }).first().click();
    await expect(page).toHaveURL(/\/docs\/sdks$/);
    await expect(page.getByRole('heading', { name: 'SDKs' })).toBeVisible();
  });
});
```

If `loginAsTestUser` does not exist in `mock-api.ts`, use the same login flow used by sibling specs (e.g. `e2e/ui/api-keys.spec.ts`). If the helper has a different name, substitute it.

- [x] **Step 3: Run the spec**

```bash
cd web
npx playwright test e2e/ui/landing-and-header.spec.ts
```

Expected: all tests pass. If a selector fails because of a missing helper or different login flow, adjust the spec to match the project's existing pattern (mirror what `api-keys.spec.ts` or `members.spec.ts` does).

- [x] **Step 4: Run the full E2E suite to catch regressions**

```bash
cd web
npm run test:e2e
```

Expected: all existing specs still pass plus the new one.

- [x] **Step 5: Commit**

```bash
git add web/e2e/ui/landing-and-header.spec.ts
git commit -m "test(e2e): cover landing page, header user menu, and docs navigation"
```

---

### Task 19: Final smoke + completion

**Files:**
- Modify: `docs/Current_Initiatives.md` (or whichever current-initiatives file the repo uses — see CLAUDE.md)

- [x] **Step 1: Run all checks**

```bash
cd web
npm run lint
npm test
npm run build
```

Expected: lint clean, vitest green, build succeeds.

- [x] **Step 2: Manual smoke walkthrough**

```bash
cd web
npm run dev
```

Walk through:
1. Visit `/` logged out → see hero, scroll through all sections, click `Get started` → reach `/register`.
2. Register or log in → URL becomes `/portal`, redirects to last org dashboard.
3. Click DS brand in header → returns to `/`. Confirm `Portal` button + user avatar are visible.
4. Click `Portal` → returns to last org. The top-right user menu is visible on every page; the sidebar no longer has a user footer or DS brand.
5. Open user menu → click `Settings` → navigates to org settings. Reopen menu → click `Logout` → navigates to `/`, logged out.
6. Log back in. Click `Documentation` in the left sidebar → reaches `/docs/getting-started`. Click each sidebar entry, verify content swaps and code blocks are syntax-highlighted. Verify internal markdown links (`/docs/sdks`) navigate within the app.
7. Resize the browser to ~700px wide and re-walk the landing page; verify sections collapse correctly and the diagram either fits or scrolls.

Stop the dev server.

- [x] **Step 3: Update Current Initiatives**

The repo's `CLAUDE.md` requires `docs/Current_Initiatives.md` and a Completion Record on the plan file. Update:

1. Add this plan to (or remove from) `docs/Current_Initiatives.md` per the project's existing format.
2. Add a Completion Record section to this plan file:

```markdown
## Completion Record
- **Branch**: <current branch name>
- **Committed**: Yes
- **Pushed**: <Yes/No after push>
- **CI Checks**: <Pass/Fail>
```

- [x] **Step 4: Commit completion metadata**

```bash
git add docs/Current_Initiatives.md docs/superpowers/plans/2026-04-12-landing-page-and-header.md
git commit -m "docs: mark landing page implementation complete"
```

---

## Self-review notes

**Spec coverage check:**
- Landing page sections (hero, deploy/release flow, three pillars, code contrast, lifecycle strip, CTA, footer): Tasks 7–12 ✓
- Shared SiteHeader with auth-aware right slot: Task 4 ✓
- UserMenu with initials, settings link, logout, outside-click, Escape: Task 3 (TDD) ✓
- HierarchyLayout integration + sidebar cleanup: Task 5 ✓
- Routing: `/` always landing, `/portal` post-login target, login default updated: Task 6 ✓
- In-app `/docs` route with bundled markdown: Tasks 13–15 ✓
- Sidebar Documentation link: Task 16 ✓
- Lazy-loading: Task 17 ✓
- Playwright E2E coverage: Task 18 ✓
- Dependencies + bundle plan: Task 1 ✓
- Vitest setup: Task 2 (added because the spec called for unit tests but no test framework existed) ✓
- Animated diagram with `prefers-reduced-motion`: Task 8 ✓

**Deviations from spec:**
- Spec said "Jest + RTL". Plan uses **Vitest** (Vite-native, drop-in API equivalent) — pragmatic given the project already uses Vite.
- Spec said `~12` new files. Actual is 16 (counted correctly during the spec self-review fix).

**Type/name consistency:**
- `UserMenu` props: none. ✓
- `SiteHeader` prop: `variant: 'landing' | 'app'`. ✓
- `MarkdownRenderer` prop: `source: string`. ✓
- `docsManifest` shape: `{ slug, title, source }` — used identically by `DocsSidebar` (Task 15) and `DocsPage` (Task 15). ✓
- `findDoc(slug)` returns `DocEntry | undefined` — caller in `DocsPage` checks for `undefined`. ✓

No placeholders, no TODO/TBD. Complete code in every step.

## Completion Record
- **Branch**: `main`
- **Committed**: Yes (22 commits, 6a548a6..469bc10)
- **Pushed**: Yes
- **CI Checks**: Lint clean, 7 unit tests passing, 8 E2E tests passing. Pre-existing TS errors in authHooks.ts, MembersPage.tsx, realtime.ts (not introduced by this work).

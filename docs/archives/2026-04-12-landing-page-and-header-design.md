# Landing Page, Shared Header, and In-App Docs — Design Spec

**Date:** 2026-04-12
**Status:** Approved (brainstorming complete, ready for implementation plan)
**Related plans:** TBD (writing-plans next)

## Overview

Three coordinated additions to the DeploySentry web app:

1. A new public **landing page** at `/` that explains the product (split deploy from release, controls introduced, the SDK-centralized dispatch methodology, end-to-end animated flow).
2. A **shared `SiteHeader`** component used by both the landing page and the authenticated app, replacing the current sidebar user-footer with a top-right user menu.
3. An **in-app `/docs` route** rendered from bundled markdown, linked from the left-hand sidebar of the authenticated app.

These changes are purely additive to the data plane (no API or migration changes). The landing page and docs are gated by no auth requirement and auth requirement respectively, but neither requires backend work.

## Goals

- Give DeploySentry a marketing surface that articulates its three differentiators: decoupled deploys/releases, the SDK-centralized dispatch pattern, and the LLM-ready cleanup story.
- Replace the inconsistent header experience (no header on authed pages, user info buried in the sidebar footer) with a single shared header that works on both surfaces.
- Make documentation reachable from inside the app without leaving for an external site.

## Non-goals

- No external docs site, no Mintlify/Docusaurus.
- No CMS or runtime-fetched content. Docs ship as bundled markdown and update with the web build.
- No copy finalization. Initial markdown content is **stub-quality** — coherent enough to render and navigate, but final copy is a follow-up task.
- No SDK changes. The "SDK-centralized dispatch" pattern is described in copy as the recommended pattern; productizing it is out of scope here.
- No new backend endpoints.

## Decisions locked in brainstorming

| # | Decision | Choice |
|---|---|---|
| 1 | Visual style | Developer-tool minimal — dark background, mono accents, subtle gradients (Vercel/Linear/Resend lineage) |
| 2 | Routing model for `/` | `/` always renders the landing page, authed or not. Post-login redirect goes to last-org via existing `DefaultRedirect` logic (now triggered from `/login` instead of `/`) |
| 3 | Animation approach | Framer Motion for section reveals; pure SVG + CSS keyframes for the deploy→release diagram |
| 4 | Docs hosting | In-app `/docs` route, content bundled as markdown imported via Vite's `?raw` |
| 5 | Header structure | Single shared `SiteHeader` component with a `variant` prop, full-width across the top of both landing and authed views |
| 6 | LLM-cleanup framing | Methodology / SDK pattern, framed as the recommended pattern. Not pitched as a turnkey product feature |
| 7 | Storytelling arc | Product-first — animated flow leads, three pillars follow, problem framing is implicit |

## Architecture

### Routing changes (`web/src/App.tsx`)

```
/                       → LandingPage           (public, no guard)
/login                  → LoginPage             (RedirectIfAuth)
/register               → RegisterPage          (RedirectIfAuth)
/docs                   → redirect /docs/getting-started   (RequireAuth)
/docs/:slug             → DocsPage              (RequireAuth)
/orgs/new               → CreateOrgPage         (RequireAuth)
/orgs/:orgSlug/...      → HierarchyLayout       (RequireAuth, unchanged)
```

`DefaultRedirect` is removed from `/` and instead used as the post-login navigation target inside `LoginPage` (or wherever the login success handler lives).

### File layout

**New files (16):**

```
web/src/pages/LandingPage.tsx
web/src/pages/DocsPage.tsx
web/src/components/SiteHeader.tsx
web/src/components/UserMenu.tsx
web/src/components/landing/Hero.tsx
web/src/components/landing/DeployReleaseFlow.tsx
web/src/components/landing/PillarsSection.tsx
web/src/components/landing/CTASection.tsx
web/src/components/landing/Footer.tsx
web/src/components/docs/DocsSidebar.tsx
web/src/components/docs/MarkdownRenderer.tsx
web/src/docs/index.ts                        # docs manifest
web/src/docs/getting-started.md              # stub content
web/src/docs/sdks.md                         # stub content
web/src/docs/cli.md                          # stub content
web/src/docs/ui-features.md                  # stub content
```

**Touched files (5):**

```
web/src/App.tsx                              # routes + post-login target
web/src/components/Sidebar.tsx               # remove user footer + brand header, add Docs link
web/src/components/HierarchyLayout.tsx       # render SiteHeader above sidebar+content
web/src/styles/globals.css                   # header layout, landing styles, docs styles
web/package.json                             # new deps
```

### `SiteHeader` component

Single component, both surfaces.

```ts
type SiteHeaderProps = {
  variant: 'landing' | 'app';
};
```

- Fixed at the top of the viewport, 60px tall, dark background, 1px bottom border.
- **Left:** DS logomark + "DeploySentry" wordmark, links to `/`. Clicking from anywhere returns to the landing page.
- **Center (landing variant only):** marketing nav — `Product` (anchor to pillars section), `Docs` (`/docs`), `SDKs` (`/docs/sdks`).
- **Right (auth-aware):**
  - Unauthed: `Log in` link + `Sign up` button.
  - Authed, landing variant: `Portal` button (routes to last-org dashboard via the same logic `DefaultRedirect` uses) + `UserMenu`.
  - Authed, app variant: `UserMenu` only.

Reads auth state via `useAuth()`. Safe on landing page because `AuthProvider` wraps the whole router in `App.tsx`.

### `UserMenu` subcomponent

- Trigger: 32px round avatar with the user's initials (derived from `user.name` if present, else first two letters of `user.email`), with a chevron.
- Dropdown anchored to the trigger:
  - Header row: full name + email (read-only).
  - `Settings` → `/orgs/:orgSlug/settings` for the current org if URL has one, else last-org.
  - `Logout` → calls `logout()`, navigates to `/`.
- Closes on outside-click and `Escape`.
- Hand-rolled, no Radix/Headless UI dependency. Click-outside via a `useEffect` mousedown listener; ~80 lines total.
- Unit-tested with Jest + React Testing Library.

### `HierarchyLayout` impact

- Becomes a vertical flex: `<SiteHeader variant="app" />` on top, then the existing `[sidebar | content]` row beneath.
- The sidebar's `.sidebar-footer` user block (currently `Sidebar.tsx:111-115`) is **deleted** — user info now lives only in the header.
- The sidebar's `.sidebar-header` (DS logo + "DeploySentry" wordmark) is **also deleted** since the brand now lives in the top header. The sidebar's first content becomes the org switcher.

### Landing page composition (`LandingPage.tsx`)

Thin orchestrator: `<SiteHeader variant="landing" />` followed by section components in a single dark `<main>`. No router-level state. ~40 lines.

**Sections in order:**

**1. Hero (`landing/Hero.tsx`)**
- Full viewport height, dark background with a subtle radial gradient (top-center, ~8% lighter).
- Eyebrow tag (mono, muted): `FEATURE FLAG INFRASTRUCTURE`.
- Headline (~64px sans): **"Ship code. Release features. Separately."**
- Subheadline (~20px muted): one sentence on splitting deploy from release and centralizing flag logic.
- Primary button `Get started` → `/register`. Secondary button `Read the docs` → `/docs`.
- Right side / below: a static teaser of the deploy→release diagram (the lit end-state, no animation).
- Framer Motion: stagger-fade eyebrow → headline → subheadline → buttons on mount, 60ms steps.

**2. Animated deploy→release flow (`landing/DeployReleaseFlow.tsx`)** — see dedicated section below.

**3. Three pillars (`landing/PillarsSection.tsx`)**
- Section heading: **"Three controls, one model"**.
- Three equal cards (stack on mobile):
  - **Decoupled deploys & releases** — ship dark, release on a flag flip, no rebuild.
  - **Centralized SDK dispatch** — register functions against a flag, call `sdk.dispatch(flag, ctx)`, the SDK picks the right one. Tag: `Recommended pattern`.
  - **LLM-ready cleanup** — every flag's call sites are discoverable from the central registry, so an agent (or human) can confidently retire a flag and the dead code with it. Short snippet showing the registry shape.
- Each card: dark surface, 1px border, mono small-caps section label, sans body, single accent-color icon. Framer Motion `whileInView` fade-up with a 100ms stagger.

**4. Code-snippet contrast band**
- Single full-width band, dark surface, syntax-highlighted snippet contrasting the old way vs the dispatch pattern side by side.
  ```ts
  // Before — direct call, flag check scattered
  if (flags.isEnabled('new-checkout')) newCheckout(); else oldCheckout();

  // After — centralized dispatch
  ds.dispatch('new-checkout', ctx);
  ```
- Static, no animation. The "show, don't tell" moment for the methodology.

**5. End-to-end lifecycle strip**
- Horizontal storyboard: `Develop → Ship dark → Targeted release → Observe → Retire flag → LLM cleans code`. Six small step cards with connecting arrows.
- Framer Motion: arrows draw in sequence on scroll-into-view.
- Distinct from the deploy/release diagram in section 2 — that one is technical (parallel tracks), this one is narrative (lifecycle).

**6. CTA (`landing/CTASection.tsx`)**
- Centered. Headline: **"Stop coupling deploys to releases."**
- Primary button → `/register`. Secondary muted text link → `/docs`.

**7. Footer (`landing/Footer.tsx`)**
- Three columns: Product (Features, SDKs, CLI) · Docs (Getting started, SDK reference, CLI reference) · Project (GitHub, Changelog).
- Bottom row: small DS logomark + copyright + version.

### Animated deploy→release flow diagram

`landing/DeployReleaseFlow.tsx` — pure SVG + CSS keyframes, no Framer Motion.

**Conceptual layout:** two parallel horizontal tracks separated by a dashed vertical line. The **deploy** track (top) completes before the **release** track (bottom) begins, visually demonstrating the decoupling.

```
DEPLOY (engineering owns →)
 ●────────●────────●────────●
 commit   build    deploy   live (dark)

 ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─

RELEASE (product / on-call owns →)
            ●────────●────────●────────●
          internal   5%       50%      100%
```

**SVG components:**
- Two horizontal rails (`<line>`), 1px stroke, muted color.
- Eight nodes total (4 per track), `<circle>` r=6, accent color, with text labels above.
- A dashed horizontal separator between tracks.
- A small floating badge on the right of each track: `engineering owns →` and `product / on-call owns →`.

**Animation sequence (CSS keyframes, ~5s loop):**

| t (s) | Event |
|---|---|
| 0.0 | All nodes invisible, rails dim |
| 0.2 | `commit` node fades in |
| 0.4 | Rail draws from `commit` to `build` (stroke-dashoffset) |
| 0.6 | `build` node fades in |
| 0.8–1.6 | Repeat for `deploy` and `live (dark)` |
| 2.0 | `live (dark)` pulses once (scale 1→1.15→1) |
| 2.4 | Dashed separator draws horizontally |
| 2.8–4.4 | Release track plays through `internal → 5% → 50% → 100%` at the same cadence |
| 4.6 | All eight nodes lit |
| 5.0–6.5 | Hold |
| 6.5 | Fade to ~30% opacity, restart |

**Implementation:**
- Each rail segment is its own `<line>` with `stroke-dasharray` / `stroke-dashoffset` and a per-element CSS `animation-delay`.
- Nodes use a small `@keyframes pop` (opacity 0→1, scale 0.8→1) with staggered delays.
- All timings live as CSS custom properties at the component root for easy retuning.
- `prefers-reduced-motion: reduce` → render the static end state with all nodes lit, no keyframes.

**Responsive behavior:**
- ≥900px: side-by-side tracks as drawn.
- <900px: tracks stack vertically (two rows of 4), separator becomes horizontal between them. Same animation logic.

### Docs route

**Routes:** `/docs` redirects to `/docs/getting-started`. `/docs/:slug` renders `DocsPage`.

**`DocsPage.tsx` layout:**
- Wrapped in `SiteHeader` (variant `app`) for visual continuity.
- Below the header: two-column shell — `DocsSidebar` (240px), `MarkdownRenderer` (max-width 760px, centered in the remaining space).
- **Not** nested under `HierarchyLayout` — docs don't need org/project/app context. This is its own top-level authed layout, gated by `RequireAuth`.

**`DocsSidebar`:** reads from a manifest exported from `web/src/docs/index.ts`:

```ts
import gettingStarted from './getting-started.md?raw';
import sdks from './sdks.md?raw';
import cli from './cli.md?raw';
import uiFeatures from './ui-features.md?raw';

export const docsManifest = [
  { slug: 'getting-started', title: 'Getting Started', source: gettingStarted },
  { slug: 'sdks',            title: 'SDKs',            source: sdks },
  { slug: 'cli',             title: 'CLI',             source: cli },
  { slug: 'ui-features',     title: 'UI Features',     source: uiFeatures },
] as const;
```

Renders one `<NavLink>` per entry, highlights active.

**`MarkdownRenderer.tsx`:**
- Receives the raw markdown for the active slug, runs it through `react-markdown` with `remark-gfm` and `rehype-highlight`.
- Custom component overrides:
  - `h1`/`h2`/`h3` → inject anchor IDs for in-page TOC.
  - `pre`/`code` → apply the dark code theme.
  - `a` → use react-router `Link` for internal links (those starting with `/`).
- ~60 lines.

**Sidebar link addition (`Sidebar.tsx`):** new `Help` section at the bottom of `<nav>`:

```tsx
<div className="sidebar-section">Help</div>
<NavLink to="/docs" className={...}>
  <span className="nav-icon">?</span>
  Documentation
</NavLink>
```

Visible on every authed view; no project context required.

**Stub content scope:**
- `getting-started.md` — adapted from existing `docs/Getting_Started.md`. Sections: Install, Create org, Create your first flag, Wire up an SDK.
- `sdks.md` — overview table of all 7 SDKs with install snippet and a link to each SDK's README. Section per SDK with the basic init pattern.
- `cli.md` — top-level command list with one-line descriptions, then a section per subcommand (`auth`, `flags`, `deploy`, etc.) with synopsis and example.
- `ui-features.md` — a tour of each main page (Projects, Apps, Flags, Deployments, Members, API Keys, Settings, Analytics, SDKs) with one sentence on what it does.

## Dependencies

| Package | Approx. size (gz) | Purpose |
|---|---|---|
| `framer-motion` | ~50KB | Section reveal animations on landing |
| `react-markdown` | ~30KB | Docs rendering |
| `remark-gfm` | ~15KB | GFM tables/checklists in docs |
| `rehype-highlight` | ~5KB | Code block syntax highlighting |
| `highlight.js` | ~30KB (registered langs only) | Highlighter engine. Register only `ts`, `js`, `bash`, `go`, `python`, `java`, `ruby`, `dart`, `json`, `yaml` |

**Total bundle impact:** ~130KB gzipped. `LandingPage` and `DocsPage` are lazy-loaded via `React.lazy` so they don't weigh down the authed app shell.

## Testing strategy

**Unit tests:**
- `UserMenu.test.tsx` (Jest + RTL):
  - Outside-click closes the menu.
  - `Escape` closes the menu.
  - Logout button fires `logout()` and navigates to `/`.
  - Settings link points to current org's settings.
- No unit tests for purely presentational landing components.

**Playwright E2E:** new spec `web/e2e/ui/landing-and-header.spec.ts`:
- Unauthed `/` shows landing, hero headline visible, log-in button works.
- Authed `/` shows landing with `Portal` button + `UserMenu` visible.
- Clicking the DS logo from `/orgs/foo/projects/bar/flags` returns to `/`.
- `UserMenu` opens, shows initials, `Logout` works.
- `/docs` redirects to `/docs/getting-started`, sidebar nav switches docs pages, code blocks highlight.

**Manual smoke before merge:** `make run-web`, walk through landing → login → app → docs → logout.

## Rollout

- Single feature branch, all changes land together.
- Purely additive on the data plane; no migration, no API changes, no feature flag.
- Verification gate before merge: full Playwright suite green + manual smoke.

## Open questions / follow-ups

- Final landing copy is a follow-up task (this spec ships stubs).
- Final docs content is a follow-up task (stubs render but need real prose).
- A future task may swap the bundled-markdown docs for an API-fetched source. The `docsManifest` interface is small and easy to replace.

## File and line references

- Current sidebar user footer: `web/src/components/Sidebar.tsx:111-115` — to be removed.
- Current `/` route: `web/src/App.tsx:39` — to be replaced with `LandingPage`.
- `AuthProvider` wraps the whole router: `web/src/App.tsx:27` — confirms `useAuth()` is safe inside `SiteHeader` on the landing page.
- Existing markdown source: `docs/Getting_Started.md` — basis for `web/src/docs/getting-started.md`.

# Mobile PWA — Design Spec

**Date:** 2026-04-24
**Phase:** Design
**Owner:** Shad Gamel

## Overview

A mobile-first Progressive Web App for monitoring and managing DeploySentry from a phone. Complements (does not replace) the existing Flutter app at `/mobile/`. Focused on the three things an operator needs away from a laptop: see what's running, see what's been deployed, and tweak feature flags in flight.

Design scope is intentionally narrow: the PWA is **view status + view deploy history + view and limited-edit of feature flags**. Everything else (creating orgs/projects/apps, member management, rollout strategy config, analytics, etc.) is deliberately pushed to the desktop web app.

## Goals

- Provide a fast, installable mobile experience for on-call operators and managers.
- Mirror the existing web visual language (reskin in progress) for continuity.
- Share auth/API model with the web app — no new backend surface.
- Work on a flaky mobile connection: stale reads stay visible offline, writes are blocked offline (no queued mutations).
- Ship in small, independently-deployable phases.

## Non-Goals (v1)

- Native mobile app features (that's what `/mobile/` is for long-term).
- Push notifications for deploy/flag events.
- Biometric / WebAuthn re-auth.
- Queued offline mutations.
- Compound targeting rule editing.
- Creating or deleting targeting rules.
- Creating, editing, or archiving flags themselves.
- Any org/project/app/member/API-key/settings/webhook/Slack management.
- Rollout strategy or rollout-group configuration.
- Deploy creation, promotion, or rollback from the phone.
- Release management.
- Analytics dashboards.

## Architecture

### Package layout

A new top-level package `mobile-pwa/`, sibling to `web/`. Its own `package.json`, Vite config, TypeScript config, service worker, and build output. No shared workspace package with `web/` — a small amount of glue (api client, auth provider, types) is duplicated by design; extraction happens later if duplication becomes painful.

```
mobile-pwa/
├── package.json
├── vite.config.ts              # vite-plugin-pwa registered
├── tsconfig.json
├── index.html                  # mobile viewport meta, manifest link
├── public/
│   ├── manifest.webmanifest
│   ├── icon-192.png
│   ├── icon-512.png
│   └── icon-maskable-512.png
└── src/
    ├── main.tsx
    ├── App.tsx                 # routes + providers
    ├── api.ts                  # trimmed subset of web/src/api.ts
    ├── auth.tsx                # JWT auth provider (mirrors web)
    ├── authContext.ts
    ├── authJwt.ts
    ├── types.ts                # shared types (subset of web/src/types.ts)
    ├── layout/
    │   ├── MobileLayout.tsx    # bottom-tab shell
    │   ├── TabBar.tsx
    │   └── TopBar.tsx          # org chip, back button, refresh indicator
    ├── pages/
    │   ├── LoginPage.tsx
    │   ├── OrgPickerPage.tsx
    │   ├── StatusPage.tsx
    │   ├── HistoryPage.tsx
    │   ├── DeploymentDetailPage.tsx
    │   ├── FlagProjectPickerPage.tsx
    │   ├── FlagListPage.tsx
    │   ├── FlagDetailPage.tsx
    │   ├── RuleEditSheet.tsx
    │   └── SettingsPage.tsx    # account-only: logout, token expiry, about
    ├── components/
    │   ├── EnvChip.tsx
    │   ├── StatusPill.tsx
    │   ├── FilterChip.tsx
    │   ├── FilterSheet.tsx
    │   ├── StaleBadge.tsx
    │   └── OfflineWriteBlockedModal.tsx
    ├── hooks/
    │   ├── useAutoPoll.ts      # 15 s poll with visibility-pause
    │   └── useCachedFetch.ts   # thin wrapper reading SW cache timestamps
    └── styles/
        ├── tokens.css          # copied from web globals.css (theme vars)
        └── mobile.css          # layout primitives
```

### Serving

- Served at `/m` subpath off the existing API origin (avoids CORS/cookie-scope complications). Subdomain (`m.dr-sentry.com`) can be introduced later without code change.
- Dev: `make run-mobile` runs `vite` on port 3002 with `/api` proxy to `:8080`.
- Prod: `mobile-pwa/dist/` served as static assets by the API (or by a fronting reverse proxy) under `/m/*`.

### Tech stack

- Vite 5 + React 18 + TypeScript 5 (same as `web/`).
- `vite-plugin-pwa` for manifest generation, service worker registration, and Workbox integration.
- `react-router-dom` v6 (same as `web/`).
- No UI kit; custom CSS using theme tokens shared with `web/`.

## Navigation & routes

Bottom-tab shell with three persistent tabs. Drill-downs push on top of the current tab's stack; switching tabs preserves scroll position per tab.

```
/m/login                                                # public
/m/orgs                                                 # shown only if user has >1 orgs
/m/orgs/:orgSlug                                        # redirect → /status
/m/orgs/:orgSlug/status                                 # TAB 1
/m/orgs/:orgSlug/history                                # TAB 2
/m/orgs/:orgSlug/history/:deploymentId                  #   drill-down
/m/orgs/:orgSlug/flags                                  # TAB 3 — project picker
/m/orgs/:orgSlug/flags/:projectSlug                     #   project-scoped list
/m/orgs/:orgSlug/flags/:projectSlug/apps/:appSlug       #   app-scoped list
/m/orgs/:orgSlug/flags/:projectSlug/:flagId             #   flag detail
/m/orgs/:orgSlug/flags/:projectSlug/:flagId/rules/:ruleId/edit   # modal sheet
/m/settings                                             # account
```

### Tab bar behavior

- Always visible on the three top-level screens and on drill-downs.
- Hidden on `/login` and `/orgs` (pre-auth / pre-org).
- Tapping a tab while already on a drill-down pops to the tab root.
- A top-bar back chevron appears on every drill-down; browser back gesture (Android back, iOS swipe) works identically.

### Org selection

1. On successful login, the client calls `GET /api/v1/orgs`.
2. **Exactly one org** → redirect to `/orgs/:slug/status` immediately. No picker.
3. **Multiple orgs** → route to `/orgs` and render a picker (list of org names, tappable).
4. A small org-name chip in the top-left of every in-org screen returns the user to `/orgs` to switch.
5. Last-selected org is persisted in `localStorage` (`ds_active_org`) and preferred on next login.

## Screens

### Tab 1 — Status (`/orgs/:orgSlug/status`)

Mirrors the web `OrgStatusPage` fan-in; auto-polls `GET /api/v1/orgs/:slug/status` every 15 s (paused when the page is hidden via the Page Visibility API).

- **Top bar:** org chip + refresh indicator.
- **Project cards** (collapsed by default): project name, aggregate health pill (healthy / deploying / degraded / down), app count. Tap to expand.
- **Expanded card:** rows of apps; each row shows app name + an env-chip strip (`prod ✓ · staging ⏱ · dev ✓`). Each chip color-codes its env's latest deploy status; tap a chip to jump to that deployment's detail in the History tab. A `↗` icon per app opens the monitoring link if configured.
- **Build pill** next to each env chip where a recent `github-actions` status exists (⏱/✓/✗), click-through to the workflow html_url.
- **Stale-data badge** shown when rendering SW-cached data older than the freshest in-flight fetch.

### Tab 2 — Deploy History (`/orgs/:orgSlug/history`)

Mirrors the web `OrgDeploymentsPage` with mobile-first filters.

- **Filter chip row** at top (horizontally scrollable): Project · App · Env · Status. Tapping each chip opens a bottom sheet for selection. A "Filters" button shows a full-screen panel for combining filters plus date range.
- **Chronological list:** status pill · version · env · age (`2m ago`). Tap row → detail.
- **Cursor-paginated infinite scroll** (reuses existing `GET /api/v1/orgs/:slug/deployments?cursor=...`).
- **Filters serialize to URL** so deep-links work.
- **Deployment detail:** version, status timeline (started → healthy → completed | failed), phase progression, mode badge, source column, build pill. **Read-only** — no rollback/promote on mobile v1.

### Tab 3 — Flags

#### Project picker (`/orgs/:orgSlug/flags`)

List of projects for the active org. Tap to drill. If the user has only one project, auto-redirect to that project.

#### Flag list (`/flags/:projectSlug` or `/flags/:projectSlug/apps/:appSlug`)

- Search input at top (filters by flag key or name).
- Category chips: release / feature / experiment / ops / permission (multi-select).
- Rows: flag key (mono font) · name · env-chip strip showing on/off state per env.
- Scope toggle: "Project flags" vs. "App: <name>" if inside an app.

#### Flag detail (`/flags/:projectSlug/:flagId`)

- **Header:** key (mono), name, category badge, owners list, expiration pill if `release`.
- **Environment sections** — one collapsible section per environment. Each shows:
  - A prominent toggle for `enabled` in that env → `PUT /api/v1/flags/:id/environments/:envId` with `{ "enabled": boolean }`.
  - Current `default_value` with tap-to-edit (text field for string, toggle for boolean, number field for number, JSON editor disabled on mobile with "Edit on desktop" link).
  - Ordered list of targeting rules, each row showing:
    - Priority number.
    - Rule type badge.
    - Summary (e.g., "25% rollout", "plan eq enterprise", "3 user IDs").
    - A per-rule enable/disable toggle → `PUT /flags/:flagId/rules/:ruleId` with `enabled` patch.
    - "Edit" button → opens rule edit sheet.
    - Drag-handle (long-press to pick up, drag up/down) → commits new order via `PUT /flags/:flagId/rules/reorder`.
  - **No "New rule" or "Delete rule" controls.** A single link: "Create or delete rules on desktop →" deep-links to `https://dr-sentry.com/orgs/:orgSlug/projects/:projectSlug/flags/:flagId`.
- **History section** at the bottom: paginated audit-log entries (who toggled what when) from `GET /flags/:id/audit`.

#### Rule edit sheet (`/rules/:ruleId/edit`)

Full-screen iOS-style modal scoped to an existing rule. Fields by rule type:

| Rule type      | Editable fields on mobile                                                     |
|----------------|--------------------------------------------------------------------------------|
| `percentage`   | slider + numeric input 0–100                                                   |
| `user_target`  | chip-style editor for user IDs (add / remove)                                  |
| `attribute`    | attribute name, operator dropdown, value editor (string / number / list), `negate` toggle |
| `segment`      | segment picker (select from existing segments; no segment creation)            |
| `schedule`     | start/end datetime pickers, timezone, days-of-week checkboxes                  |
| `compound`     | **read-only** with "Edit on desktop" CTA                                       |

Save → `PUT /api/v1/flags/:flagId/rules/:ruleId`. Cancel → discard and pop. Write is blocked with the offline modal if no network.

### Login / Account

- **`/login`:** email + password form, "Sign in with GitHub" and "Sign in with Google" buttons using the existing `/api/v1/auth/oauth/*` endpoints. `next=` query param for post-login redirect.
- **`/settings`:** current user email, number of orgs the user belongs to, token-expiry countdown, "Sign out" button, version / build hash, link to the desktop web app.

## Data flow

### API client

`mobile-pwa/src/api.ts` mirrors `web/src/api.ts` but only exposes what the PWA needs:

- `authApi`: `login`, `me`, `extend`, `logout`, `oauth*`.
- `orgsApi`: `list`.
- `statusApi`: `getOrgStatus(orgSlug)`.
- `deploymentsApi`: `list(orgSlug, filters, cursor)`, `get(id)`.
- `projectsApi`: `list(orgSlug)`.
- `appsApi`: `list(orgSlug, projectSlug)`.
- `flagsApi`: `list({ projectId, appId?, category?, archived? })`, `get(id)`, `updateEnvironmentState(flagId, envId, patch)` (used for env-scoped enable + default-value edits), `updateRule(flagId, ruleId, patch)`, `reorderRules(flagId, orderedRuleIds)`.
- `auditApi`: `listForFlag(flagId, cursor)`.

Auth: read `ds_token` from `localStorage`, attach `Authorization: Bearer <jwt>` (or `ApiKey` if token starts with `ds_`, matching web). 401 → redirect to `/login?next=<current>`.

### Auth flow

Mirrors `web/src/auth.tsx`:

- `AuthProvider` manages token, user, expiry, and timer-scheduled warning + logout.
- `SessionExpiryWarning` modal appears 60 s before JWT expiry, offering "Stay signed in" → `POST /auth/extend`.
- Silent 401 handler from the API client clears the token and navigates to `/login?next=...`.
- `RequireAuth` outlet wrapper around protected routes.
- No biometric, no PIN. Same `localStorage` key as web (`ds_token`). Because the PWA is served from the same origin (subpath `/m`), localStorage is shared with the desktop web app — a user already signed in to `web` is signed in to the PWA automatically, and signing out of one signs out of the other. This is intentional for v1. If the PWA later moves to a subdomain, it will have its own localStorage scope and the user will re-login on first visit.

### Permissions / RBAC

Client reflects the same gates the API enforces:

- Toggle/edit actions are disabled (greyed out, labelled "Read-only") when the current user's role for the active project is `viewer`.
- No new permission scopes introduced — backend enforces `flags:write`, `project:developer`, etc., and the client respects those responses.

### Offline & service worker

`vite-plugin-pwa` + Workbox:

- **App shell** (HTML / JS / CSS / manifest / icons): `precacheAndRoute` — baked into the SW at build time, instantly available offline.
- **Read-only GET** to `/api/v1/orgs`, `/api/v1/orgs/:slug/status`, `/api/v1/orgs/:slug/deployments*`, `/api/v1/flags*`, `/api/v1/projects*`, `/api/v1/apps*`: `StaleWhileRevalidate` strategy, 24-hour max-age, per-URL cache entries.
- **Mutations** (`POST`, `PUT`, `DELETE`, including `/toggle`): never cached, never queued. When offline, the client detects `navigator.onLine === false` (and falls back to a fetch try/catch) and shows `OfflineWriteBlockedModal`: "You're offline — connect to make changes."
- **Stale badge:** every screen that renders cached data subscribes to the `ServiceWorker` cache-match timestamp. If the displayed data came from cache and a refresh fetch is in flight, the screen renders "Showing data from 2m ago". On fetch success, the timestamp updates and the badge disappears.
- **SW update flow:** `vite-plugin-pwa` auto-update mode — when a new SW is installed and waiting, show a non-blocking banner "Update available — tap to reload" that calls `skipWaiting` + reload.
- **Manifest / install:** `name: "Deploy Sentry"`, `short_name: "DS"`, `start_url: "/m/"`, `display: "standalone"`, `theme_color` matching the reskin accent, `background_color` matching the app shell, maskable icons at 192 and 512. `<meta name="apple-mobile-web-app-capable" content="yes">` etc. in `index.html` for iOS install fidelity.

## Styling

- **Theme tokens** shared with web via a copy of the CSS custom-property block from `web/src/styles/globals.css` into `mobile-pwa/src/styles/tokens.css`. Variables: `--bg`, `--surface`, `--surface-2`, `--border`, `--text`, `--muted`, `--accent`, `--success`, `--warn`, `--danger`, etc.
- **Layout primitives** in `mobile-pwa/src/styles/mobile.css`: `.m-screen`, `.m-tab-bar`, `.m-top-bar`, `.m-card`, `.m-sheet`, `.m-list-row`, `.m-chip-strip`, `.m-filter-chip`.
- **Touch targets** minimum 44×44 pt.
- **Safe areas** respected via `env(safe-area-inset-bottom)` / `-top` padding; `viewport-fit=cover`.
- **Momentum scrolling** on list views (`-webkit-overflow-scrolling: touch`).
- **Dark / light theme** follows the existing web theme toggle (same CSS variable overrides driven by the same `localStorage` key, if web uses one).

## Testing

### Unit (vitest)

- API client fetch + auth header logic + 401 redirect.
- `AuthProvider`: token scheduling, expiry warning, extend, logout, clear.
- `FlagEnvState` reducer: toggle, edit default, rule enable/disable, reorder.
- Rule summary renderer: produces correct one-line summaries for each rule type.
- `useAutoPoll`: pauses on hidden, resumes on visible.

### Component (vitest + @testing-library/react)

- `StatusPage`: renders fan-in response, expands/collapses project cards, polls on interval.
- `FlagDetailPage`: env toggle and default-value edit both commit `PUT /flags/:id/environments/:envId`; rule-enable toggle commits `PUT /flags/:id/rules/:ruleId`.
- `RuleEditSheet`: one test per rule type (`percentage`, `user_target`, `attribute`, `segment`, `schedule`) verifying field-to-patch mapping; `compound` renders read-only CTA.
- `MobileLayout`: tab-bar persistence across drill-downs, tapping active tab pops to root.

### E2E (Playwright, mobile viewport)

- Login → pick org → status loads → drill to deployment → back to status preserves scroll.
- Login → flags → open flag → toggle prod env → verify PUT fires and UI reflects new state.
- Login → go offline → reload → stale badge renders → try to toggle → offline modal appears.
- Multi-org: login as user with >1 org → picker renders → select org → status loads.
- Single-org: login as user with exactly 1 org → auto-redirects, no picker.

### Manual device smoke

- Install to iOS Safari home screen; verify standalone mode, status-bar color, safe-area padding, back-swipe, SW update prompt.
- Install to Android Chrome home screen; verify standalone mode, theme color, back gesture, SW update prompt.
- Toggle airplane mode while the app is open; verify cached reads still render with stale badge and writes block correctly.

## Phased rollout

Each phase is independently deployable and produces a usable artifact.

| Phase | Scope |
|-------|-------|
| **1. Scaffolding** | `mobile-pwa/` package, Vite + `vite-plugin-pwa`, manifest, icons, basic login, org picker, tab-bar shell with empty routes. Deployed to `/m`. |
| **2. Status tab** | `StatusPage` with fan-in polling, project/app cards, env chips, build pills, monitoring links. |
| **3. History tab** | `HistoryPage` list + filters + cursor pagination + `DeploymentDetailPage`. |
| **4. Flags read-only** | Project/app picker, flag list with search + category filters, flag detail with env sections and rule listings (no writes yet). Flag history section. |
| **5. Flags writes** | Env enable toggle, default value edit, per-rule enable/disable, rule edit sheet for non-compound rule types, rule reorder. |
| **6. Offline polish** | SW stale-while-revalidate for reads, `StaleBadge`, `OfflineWriteBlockedModal`, SW update banner. |

## Open questions

None at design time. All scope boundaries resolved in brainstorming.

## Completion Record

<!-- Fill in when phase is set to Complete -->
- **Branch:** _TBD_
- **Committed:** _TBD_
- **Pushed:** _TBD_
- **CI Checks:** _TBD_

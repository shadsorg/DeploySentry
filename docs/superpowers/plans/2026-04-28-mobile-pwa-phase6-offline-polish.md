# Mobile PWA — Phase 6: Offline Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox syntax. Each task ends with a verification step that includes `npm run lint` (eslint) — never commit a step without lint clean.

**Goal:** Make the PWA feel resilient on a flaky mobile connection. Add stale-while-revalidate to read GETs (so cached reads render instantly while a fresh fetch is in flight), surface a `<StaleBadge>` when the screen is rendering cached data, block writes with an `<OfflineWriteBlockedModal>` when the device is offline, and replace the auto-reload SW behavior with a non-blocking `<SwUpdateBanner>` the user can tap.

**Architecture:**

- **Reads:** Workbox `StaleWhileRevalidate` strategy registered for the GET endpoints we already use. Cache key = full URL. 24h `maxAgeSeconds`. Per the spec, this only applies to reads — mutations are never cached.
- **Stale signal:** rather than wiring app code into the SW cache map (fragile across browsers), use an in-app helper `useFetchFreshness` that wraps `fetch` and tells the caller whether the latest read happened on the network or came from the SW cache. We rely on the `cache: 'no-store'` flag plus `performance.getEntriesByType('resource')` to differentiate, with a fallback to a simple "data older than X seconds" heuristic. Cheaper and works on iOS Safari which doesn't expose CacheStorage iteration to the client by default.
- **Write blocking:** `navigator.onLine === false` is checked at the API client level. When false AND the request is non-GET, throw an `OfflineWriteBlockedError` synchronously without firing the request. App code catches this error type and renders the modal. We keep the network round-trip on optimistic-update paths (Tasks in Phase 5 already revert on error) — the modal just prevents the optimistic update entirely so the user isn't surprised when the revert fires after a long timeout.
- **SW update banner:** replace `registerSW.ts`'s auto-update behavior with a callback that flips a global flag → React state → renders a banner. Tap → call `update(true)` → reload.

No new dependencies. `vite-plugin-pwa` and Workbox already provide everything.

**Scope reminder (per spec):**
- IN: SW SWR for known read GETs, `<StaleBadge>` rendered on screens that show cached reads, `<OfflineWriteBlockedModal>` for offline mutation attempts, `<SwUpdateBanner>` instead of auto-reload, manual smoke device verification (deferred to manual testing post-merge).
- OUT: queued offline mutations, biometric/WebAuthn re-auth, push notifications, "read-only badge" for viewer role (still deferred — needs an API surface).

---

## File Structure

```
mobile-pwa/
└── src/
    ├── api.ts                                 # MODIFY — wrap mutations in offline-check; export OfflineWriteBlockedError
    ├── api.test.ts                            # MODIFY — add offline-detection tests
    ├── lib/
    │   ├── freshness.ts                       # CREATE — useFetchFreshness hook + helpers
    │   ├── freshness.test.ts
    │   ├── offlineError.ts                    # CREATE — OfflineWriteBlockedError class + isOfflineWriteBlockedError type-guard
    │   └── offlineError.test.ts
    ├── components/
    │   ├── StaleBadge.tsx                     # CREATE — small "Showing data from <time>" pill
    │   ├── StaleBadge.test.tsx
    │   ├── OfflineWriteBlockedModal.tsx       # CREATE — full-screen modal "You're offline"
    │   ├── OfflineWriteBlockedModal.test.tsx
    │   ├── SwUpdateBanner.tsx                 # CREATE — non-blocking banner "Update available"
    │   └── SwUpdateBanner.test.tsx
    ├── registerSW.ts                          # MODIFY — surface need-refresh + offline-ready to React via a small store
    ├── pages/
    │   ├── StatusPage.tsx                     # MODIFY — render StaleBadge when applicable
    │   ├── HistoryPage.tsx                    # MODIFY — render StaleBadge when applicable
    │   ├── FlagListPage.tsx                   # MODIFY — render StaleBadge when applicable
    │   └── FlagDetailPage.tsx                 # MODIFY — render StaleBadge + catch OfflineWriteBlockedError on every mutation
    ├── App.tsx                                # MODIFY — mount SwUpdateBanner globally
    ├── vite.config.ts                         # MODIFY — add Workbox runtimeCaching for read GETs
    ├── e2e/smoke.spec.ts                      # MODIFY — add offline-mode smoke (toggle context offline, attempt toggle, expect modal)
    └── styles/
        └── mobile.css                         # MODIFY — append stale badge + banner + offline modal styles
```

---

## Task 1: OfflineWriteBlockedError + offline detection in api.ts (TDD)

**Files:**
- Create: `mobile-pwa/src/lib/offlineError.ts`
- Create: `mobile-pwa/src/lib/offlineError.test.ts`
- Modify: `mobile-pwa/src/api.ts`
- Modify: `mobile-pwa/src/api.test.ts`

The API client is the single chokepoint where mutations leave the app, so we detect offline there.

- [ ] **Step 1 (test offlineError.ts first):**
  - `OfflineWriteBlockedError` extends `Error` with `name === 'OfflineWriteBlockedError'`.
  - `isOfflineWriteBlockedError(err)` returns true for instances of `OfflineWriteBlockedError`, false for plain `Error`, false for non-Error values.

- [ ] **Step 2:** Implement:

```ts
export class OfflineWriteBlockedError extends Error {
  constructor(message = "You're offline — connect to make changes.") {
    super(message);
    this.name = 'OfflineWriteBlockedError';
  }
}

export function isOfflineWriteBlockedError(err: unknown): err is OfflineWriteBlockedError {
  return err instanceof OfflineWriteBlockedError;
}
```

- [ ] **Step 3 (test api.ts offline detection):** Add to `api.test.ts`:
  - `flagEnvStateApi.set` — when `navigator.onLine === false`, throws `OfflineWriteBlockedError` SYNCHRONOUSLY without calling fetch. Use `vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false)` (with `configurable: true`) or `Object.defineProperty(navigator, 'onLine', ...)`.
  - `flagEnvStateApi.set` — when `navigator.onLine === true`, fetch IS called normally.
  - `flagsApi.setRuleEnvState` — same offline check.
  - `flagsApi.updateRule` — same offline check.
  - `flagsApi.list` (a GET) — does NOT throw when offline; the SW handles read fallback.
  - `flagsApi.get` (a GET) — does NOT throw when offline.

- [ ] **Step 4:** In `api.ts`, modify the `request<T>` helper to throw `OfflineWriteBlockedError` early when:
  - `init?.method` is one of `'POST' | 'PUT' | 'PATCH' | 'DELETE'`, AND
  - `navigator.onLine === false`.

  Method matching is case-insensitive. Reads (GET, HEAD, undefined method) skip the check.

- [ ] **Step 5:** Verify: `npm run test --silent` green, `npx tsc --noEmit` clean, `npm run lint` clean.

Commit message:
```
feat(mobile-pwa): add OfflineWriteBlockedError + offline guard in api client
```

---

## Task 2: SwUpdateBanner + registerSW refactor (TDD)

**Files:**
- Modify: `mobile-pwa/src/registerSW.ts` — expose a small subscribable store
- Create: `mobile-pwa/src/components/SwUpdateBanner.tsx`
- Create: `mobile-pwa/src/components/SwUpdateBanner.test.tsx`
- Modify: `mobile-pwa/src/App.tsx` — mount the banner globally

### Pattern

Add a tiny pub/sub in `registerSW.ts` so the React tree can react to SW state without coupling to `virtual:pwa-register`.

```ts
// registerSW.ts
type Listener = (state: { needRefresh: boolean; offlineReady: boolean }) => void;
const listeners = new Set<Listener>();
let state = { needRefresh: false, offlineReady: false };
let updateFn: ((reload?: boolean) => Promise<void>) | null = null;

function emit() {
  for (const l of listeners) l(state);
}

export function subscribeServiceWorker(l: Listener): () => void {
  listeners.add(l);
  l(state); // emit current state immediately
  return () => listeners.delete(l);
}

export function applyServiceWorkerUpdate(): Promise<void> {
  return updateFn ? updateFn(true) : Promise.resolve();
}

export function initServiceWorker() {
  updateFn = registerSW({
    immediate: true,
    onNeedRefresh() {
      state = { ...state, needRefresh: true };
      emit();
    },
    onOfflineReady() {
      state = { ...state, offlineReady: true };
      emit();
    },
  });
}
```

### Banner

`<SwUpdateBanner>` subscribes to the store, renders nothing while `!needRefresh`, and renders a small fixed-position banner when `needRefresh`. Tap → call `applyServiceWorkerUpdate()` (which calls `update(true)` under the hood and triggers a reload).

- [ ] **Step 1 (test):**
  - With `needRefresh === false`, the banner renders nothing.
  - With `needRefresh === true`, the banner renders text matching "Update available" and a button labelled "Reload".
  - Tapping the button calls `applyServiceWorkerUpdate`.
  - For tests, mock `subscribeServiceWorker` and `applyServiceWorkerUpdate` via `vi.mock('../registerSW', ...)`.

- [ ] **Step 2:** Implement the banner. Use `useSyncExternalStore` (or a small useEffect + useState) bound to `subscribeServiceWorker`. Append CSS for `.m-sw-banner` (fixed top, slides in from above, theme-aware).

- [ ] **Step 3:** Mount `<SwUpdateBanner />` once in `App.tsx`, INSIDE the auth provider but OUTSIDE the route shell so it's visible on every screen including login.

- [ ] **Step 4:** Verify all three checks clean.

Commit message:
```
feat(mobile-pwa): add SwUpdateBanner + subscribable SW state
```

---

## Task 3: OfflineWriteBlockedModal + integration (TDD)

**Files:**
- Create: `mobile-pwa/src/components/OfflineWriteBlockedModal.tsx`
- Create: `mobile-pwa/src/components/OfflineWriteBlockedModal.test.tsx`
- Modify: `mobile-pwa/src/pages/FlagDetailPage.tsx` — catch `OfflineWriteBlockedError` on every mutation path; show modal

### Modal

```ts
interface OfflineWriteBlockedModalProps {
  open: boolean;
  onClose: () => void;
}
```

Renders nothing when `!open`. When `open`, renders a fixed-position overlay with body "You're offline — connect to make changes." and a "Got it" button calling `onClose`. Tapping the backdrop also closes. Add `role="alertdialog"` and `aria-labelledby` for a11y.

### FlagDetailPage integration

The page already has 4 mutation paths: env enable toggle, env value edit, per-rule per-env toggle, rule reorder, and one indirect path (`RuleEditSheet` save). Wrap each `commitEnvState` / `commitRuleEnvState` / `swapRulePriorities` / sheet-save handler so that a thrown `OfflineWriteBlockedError` doesn't:
- Apply the optimistic update (revert is unneeded since nothing changed locally).
- Show the regular inline error.
- Instead → open the modal.

The simplest pattern: a single page-level `[modalOpen, setModalOpen]` plus a helper:

```ts
async function withOfflineGuard<T>(fn: () => Promise<T>): Promise<T | undefined> {
  try {
    return await fn();
  } catch (err) {
    if (isOfflineWriteBlockedError(err)) {
      setOfflineModalOpen(true);
      return undefined;
    }
    throw err;
  }
}
```

Wrap each mutation entry-point with `withOfflineGuard`. Since `OfflineWriteBlockedError` is now thrown SYNCHRONOUSLY in `request<T>`, the optimistic-update logic must check FIRST whether a write would be blocked. The cleanest way: do the offline check BEFORE applying the local-state change, by calling a small `assertOnlineForWrite()` helper exported from `lib/offlineError.ts` at the top of each commit function, OR by attempting the network call FIRST (await) before applying optimistic state. The plan picks option B: the existing optimistic pattern wraps `apply(patch)` BEFORE `await flagEnvStateApi.set(...)`. Flip the order to: try the API call first; if it throws `OfflineWriteBlockedError`, open the modal and return; otherwise apply optimistically AND fire the call (re-try would race — instead make the helper return a promise that's already resolved, then `apply(patch)` happens). For simplicity, just probe `navigator.onLine` synchronously at the top of each commit function via `assertOnlineForWrite()`.

```ts
// lib/offlineError.ts
export function assertOnlineForWrite(): void {
  if (typeof navigator !== 'undefined' && navigator.onLine === false) {
    throw new OfflineWriteBlockedError();
  }
}
```

This keeps optimistic updates on-line; a synchronous throw means no UI flash before the modal appears.

- [ ] **Step 1 (test modal):** open=false → renders nothing; open=true → renders alertdialog with copy + "Got it" button; clicking "Got it" calls onClose; backdrop click also calls onClose. Add an `aria-label`/`role` assertion.

- [ ] **Step 2:** Append `assertOnlineForWrite()` to `lib/offlineError.ts` + corresponding test.

- [ ] **Step 3 (test FlagDetailPage integration):** With `Object.defineProperty(navigator, 'onLine', { value: false, configurable: true })`:
  - Tap an env toggle → no PUT is fired; modal opens; toggle DOES NOT flip locally.
  - Save in `RuleEditSheet` → no PUT is fired; modal opens.
  - Tap a reorder arrow → no PUTs fire; modal opens.
- After test, restore `navigator.onLine`.

- [ ] **Step 4:** Wire the modal in `FlagDetailPage`. Use `withOfflineGuard` around each commit fn. Reset `navigator.onLine` checks in the existing optimistic helpers.

- [ ] **Step 5:** Verify all three checks clean.

Commit message:
```
feat(mobile-pwa): add OfflineWriteBlockedModal + guard FlagDetailPage mutations
```

---

## Task 4: StaleBadge + freshness hook (TDD)

**Files:**
- Create: `mobile-pwa/src/lib/freshness.ts`
- Create: `mobile-pwa/src/lib/freshness.test.ts`
- Create: `mobile-pwa/src/components/StaleBadge.tsx`
- Create: `mobile-pwa/src/components/StaleBadge.test.tsx`

### Freshness signal

The simplest reliable signal: track when each list/get last successfully fetched, and label the rendered data "stale" if the most recent successful fetch was more than `STALE_THRESHOLD_MS` ago AND a refresh fetch is currently in flight (so the user knows fresh data is loading).

```ts
// lib/freshness.ts
export interface FetchFreshness {
  lastSuccess: number | null;   // Date.now() of last successful fetch
  inflight: boolean;            // a fetch is currently running
}

export function useTrackedFetch<T>(
  fetcher: () => Promise<T>,
  deps: unknown[],
): { data: T | null; freshness: FetchFreshness; error: Error | null; refetch: () => void } {
  // useState/useEffect implementation
}
```

The hook fires `fetcher` on mount and whenever `deps` change. While running, `inflight = true`. On success, `lastSuccess = Date.now()`. Returns the freshness object so pages can render `<StaleBadge lastSuccess={...} inflight={...} />`.

### StaleBadge

```ts
interface StaleBadgeProps {
  lastSuccess: number | null;
  inflight: boolean;
  thresholdMs?: number;          // default 30_000 (30s)
}
```

Renders nothing if `lastSuccess === null` (first ever load) OR if `Date.now() - lastSuccess < thresholdMs`. Otherwise renders a small pill: `Showing data from <relative time>`. Uses the existing `relativeTime` helper or inlines a similar one.

- [ ] **Step 1 (test freshness):**
  - Hook starts with `data: null, freshness: { lastSuccess: null, inflight: true }`.
  - After fetch resolves: `data` populated, `inflight: false`, `lastSuccess` is a number.
  - On `refetch()`: `inflight: true` again.
  - On error: `data` retains previous value (or null on first load); `error` set; `inflight: false`.

- [ ] **Step 2:** Implement the hook. Don't reinvent React Query — keep it purpose-built and small.

- [ ] **Step 3 (test StaleBadge):**
  - `lastSuccess === null` → renders nothing.
  - `lastSuccess` from 5 seconds ago → renders nothing (under threshold).
  - `lastSuccess` from 60 seconds ago → renders text matching "Showing data from".
  - With `inflight === true`, the pill shows a loading dot or has `data-refreshing="true"` (your call — bonus a11y).

- [ ] **Step 4:** Implement StaleBadge. Append CSS to `mobile.css`.

- [ ] **Step 5:** Verify all three checks clean.

Commit message:
```
feat(mobile-pwa): add useTrackedFetch hook + StaleBadge component
```

---

## Task 5: Wire StaleBadge into the four read pages (TDD)

**Files:**
- Modify: `mobile-pwa/src/pages/StatusPage.tsx`
- Modify: `mobile-pwa/src/pages/HistoryPage.tsx`
- Modify: `mobile-pwa/src/pages/FlagListPage.tsx`
- Modify: `mobile-pwa/src/pages/FlagDetailPage.tsx`
- Modify each page's test file: append minimal "renders StaleBadge when data is stale" tests.

### Strategy

Each page already manages its own `useEffect` + state for fetching. Phase 6 doesn't replace those — just augments them with a `lastSuccess` state and an `inflight` boolean. Add the badge in the page header.

A more invasive refactor (pages adopt `useTrackedFetch`) is OUT of scope. Keep the diffs small.

For each page:
1. Add `const [lastSuccess, setLastSuccess] = useState<number | null>(null);` and `const [refreshing, setRefreshing] = useState(false);`
2. In every fetch start, `setRefreshing(true)`.
3. In every fetch success, `setLastSuccess(Date.now()); setRefreshing(false);`.
4. On failure, `setRefreshing(false);` (lastSuccess stays — keeps prior data labeled with its old timestamp).
5. Render `<StaleBadge lastSuccess={lastSuccess} inflight={refreshing} />` near the top of the page body.

For pages with auto-poll (StatusPage), the refresh fetch sets `refreshing` true and on success keeps `lastSuccess` rolling forward.

- [ ] **Step 1:** Update each page in turn. For each, append at least one test that:
  - Mocks the API to delay (use `setTimeout` in the mock or return a manually-resolved promise).
  - Asserts that BEFORE resolution the page does NOT show the badge (since lastSuccess is null on first ever load).
  - For pages with auto-poll: after the second poll cycle when the mock is slow, the badge appears.

  Skip elaborate poll-cycle tests if they're flaky — a single render-with-stale-prop unit test on each page is sufficient. The key verification is that the badge IS in the DOM under the right conditions; the timing is unit-tested in Task 4 already.

  Pragmatic shortcut: render each page with `lastSuccess` injected as a stale value via a small test helper, OR just verify the badge component is mounted at the right place. Mocking poll cycles in component tests is brittle.

- [ ] **Step 2:** Verify all three checks clean.

Commit message:
```
feat(mobile-pwa): render StaleBadge on Status/History/FlagList/FlagDetail
```

---

## Task 6: Workbox runtimeCaching for known read GETs

**Files:**
- Modify: `mobile-pwa/vite.config.ts`

Append a `runtimeCaching` block to the existing `workbox` config. Use `StaleWhileRevalidate` for these URL patterns (all under `/api/v1/`):

```ts
workbox: {
  globPatterns: ['**/*.{js,css,html,png,svg,ico,webmanifest}'],
  navigateFallback: '/m/index.html',
  navigateFallbackDenylist: [/^\/api\//],
  runtimeCaching: [
    {
      urlPattern: ({ request, url }) =>
        request.method === 'GET' &&
        /^\/api\/v1\/(orgs|flags|projects|audit-log|users)/.test(url.pathname),
      handler: 'StaleWhileRevalidate',
      options: {
        cacheName: 'ds-api-reads',
        expiration: {
          maxEntries: 200,
          maxAgeSeconds: 60 * 60 * 24, // 24h
        },
        cacheableResponse: { statuses: [0, 200] },
      },
    },
  ],
},
```

This is build-time config, so there is no test for the registration itself. Verify by:
- [ ] **Step 1:** `npm run build` succeeds; the generated `dist/sw.js` contains a reference to `ds-api-reads` (greppable).
- [ ] **Step 2:** Ensure the SW precache count and bundle size haven't regressed surprisingly.
- [ ] **Step 3:** Verify lint + tsc clean (no test changes here).

Commit message:
```
build(mobile-pwa): add Workbox runtime caching for read GETs
```

---

## Task 7: Playwright smoke + initiatives + final verify + push

**Files:**
- Modify: `mobile-pwa/e2e/smoke.spec.ts`
- Modify: `docs/Current_Initiatives.md`

### A. Smoke

Add a NEW Playwright test: log in, drill to a flag, **set the page context offline** via `await context.setOffline(true)`, tap an env toggle, assert the offline modal renders ("You're offline" text visible). Then `setOffline(false)` and tap "Got it" to verify it closes.

For SW update banner test, mock `subscribeServiceWorker` is harder in real Playwright — skip that test and rely on unit tests + manual smoke.

### B. Initiatives

Update `docs/Current_Initiatives.md`:
- All 6 phases of Mobile PWA shipped.
- Move the entire Mobile PWA initiative to the bottom-of-table "complete" status, OR (preferred) flip its phase to `Complete` and on merge, archive it to `docs/archives/`.

For this Phase 6 PR, just bump the row to show "Phase 5 merged (PR #62), Phase 6 in flight, all 6 phases complete on merge." Bump "Last updated" to today.

### C. Final verify

From `mobile-pwa/`:
1. `npm run lint --silent`
2. `npx tsc --noEmit`
3. `npm run test --silent` — target: 195+ tests, baseline 173.
4. `npm run build --silent`

Skip e2e locally (port 3002 conflict).

### D. Commit + push + open PR

ONE commit:
```
docs,test(mobile-pwa): wire phase 6 smoke + initiatives
```

Push and open the PR. Body should mention:
- All 4 polish components (StaleBadge, OfflineWriteBlockedModal, SwUpdateBanner, SWR for reads).
- Test coverage delta.
- Phase 6 completes the Mobile PWA initiative; per the spec, initiative moves to archives on merge.

---

## Success criteria

- All 7 tasks committed in order on `feature/mobile-pwa-phase6`.
- `npm run lint` clean **before every commit**.
- `npx tsc --noEmit` clean.
- Vitest green; new tests added for every new module (target ≥ 22 added: offlineError ~3, freshness ~5, StaleBadge ~3, OfflineWriteBlockedModal ~3, SwUpdateBanner ~3, page integration ~4).
- Build succeeds; `dist/sw.js` references `ds-api-reads` cache.
- Playwright smoke green when run in CI.
- `docs/Current_Initiatives.md` updated.
- PR opened with a body that signals this completes the Mobile PWA initiative.

## Out of scope

- Queued offline mutations.
- Push notifications.
- Biometric/WebAuthn.
- Read-only badge for viewer-role users (pending an API surface).
- Real device manual smoke is owned by the user post-merge.

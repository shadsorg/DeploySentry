# UI Audit — Batch B+ (Unblocked Smaller Items)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Status**: Implementation
**Date**: 2026-04-30
**Spec**: [`../specs/2026-04-29-ui-audit-disparities-design.md`](../specs/2026-04-29-ui-audit-disparities-design.md)
**Audit**: [`../../ui-audit/MOCKUP_DISPARITIES.md`](../../ui-audit/MOCKUP_DISPARITIES.md)
**Estimated total**: ~2 hours

**Goal**: Land the 5 audit items that don't need product input and weren't in Batch A or B. Each is small, isolated, and uses data the API already returns.

**Branch**: `feature/ui-audit-batch-b-plus`

**Out of scope** (deferred to Batch C — needs product input):
- §8 FlagDetailPage 2-column rewrite
- §9 FlagDetailPage targeting "Active Rules Hierarchy" stat strip — depends on stats API
- §10 AnalyticsPage quota banner — depends on quota API
- §13 MembersPage security-events feed — depends on audit-log endpoint
- §20 DocsPage landing page

---

### Task 1: §11 SettingsPage (project level) — section spacing tweak

**File**: `web/src/pages/SettingsPage.tsx` (project level) and possibly `web/src/styles/globals.css`
**Estimate**: ~10 min

- [ ] Audit the project-level settings sections (`General Information`, `Routing`, `Cleanup`, `Project Visibility`, `Auto-Sync Repository`, `Delete this project`) and add card borders / spacing to match the mockup's distinct sectioning.
- [ ] Reuse existing `.card` class — no new tokens.
- [ ] Verify visual at the project Settings page after `make run-web`.

---

### Task 2: §16 APIKeysPage scope cards

**File**: `web/src/pages/APIKeysPage.tsx`
**Estimate**: ~20 min

- [ ] Replace the scopes `<input type="checkbox">` list in the create form with a `checkbox-group` styled as selection cards. Each card: scope name + small description + checkbox.
- [ ] Reuse the existing scope-badge color logic (`scopeBadgeClass`) for the card accent.
- [ ] If `web/src/styles/globals.css` doesn't already have a `.scope-card` style, add one. Should look like Stripe-style card-checkboxes.

---

### Task 3: §3 OrgDeploymentsPage — Critical Events sidebar

**File**: `web/src/pages/OrgDeploymentsPage.tsx`
**Estimate**: ~30 min

- [ ] Add a right-rail aside that shows the **last 3 failed deployments** from the current filter set. Filter client-side from the existing `OrgDeploymentsResponse.deployments` rows where `status === 'failed'` (or `'rolled_back'`).
- [ ] Each entry: project · app · env · time-ago · "View →" link.
- [ ] Reuse existing `.status-pill.status-failed` styling.
- [ ] Hide the panel when there are 0 failed rows in the result.
- [ ] At narrow widths the aside should collapse below the table, not overlap.

---

### Task 4: §6 AppPage — 3-card env summary below header

**File**: `web/src/pages/AppPage.tsx`
**Estimate**: ~25 min

- [ ] Below the AppPage header (icon tile + name + tabs), add a 3-card env summary grid showing each env's current state (slug + version + health pulse). Reuse the `.env-card` class from Batch B.
- [ ] If the app has more than 3 envs, show first 3 with "+ N more" link to the env settings tab.
- [ ] Data: pull from the same source AppPage already uses for header data (likely `OrgStatus` or per-app status fetch).
- [ ] Verify it doesn't double-up if the page is nested inside an outlet that already renders env info.

---

### Task 5: §14 MembersPage — role filter pills

**File**: `web/src/pages/MembersPage.tsx`
**Estimate**: ~25 min

- [ ] Above the members table, add filter pills: `All`, `Owner`, `Admin`, `Member`, `Viewer` (org-level roles). Active pill highlighted with `.filter-pill.active` class from Batch B.
- [ ] Filter the existing in-state members list client-side by `role`. No new endpoint needed.
- [ ] Default to `All`.

---

## Verification (whole batch)

- [ ] `npm run build` clean
- [ ] `npm run lint --max-warnings 0` clean
- [ ] `npm run prettier:check` clean (or `npx prettier --check "src/**/*.{ts,tsx,css,json}"`)
- [ ] Visually inspect each page in dev: project Settings, APIKeysPage create flow, OrgDeploymentsPage with at least 1 failed row in fixtures, AppPage, MembersPage with multiple roles
- [ ] Update `docs/ui-audit/MOCKUP_DISPARITIES.md` to mark §3, §6, §11, §14, §16 as ✅ addressed

## PR

Single PR titled `feat(web): UI audit Batch B+ — small unblocked polish items`. Body links the spec + audit + lists the 5 items by audit section.

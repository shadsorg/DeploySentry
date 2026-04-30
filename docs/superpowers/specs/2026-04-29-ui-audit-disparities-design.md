# UI Audit Disparities — Address Mockup ↔ Implementation Gaps

**Status**: Design
**Date**: 2026-04-29
**Audit source**: [`docs/ui-audit/MOCKUP_DISPARITIES.md`](../../ui-audit/MOCKUP_DISPARITIES.md) (generated 2026-04-24)

## Problem

A side-by-side audit of all 20 `newscreens/*.html` mockups vs. their `web/src/pages/*.tsx` counterparts found 4 high / 10 medium / 4 low / 1 n/a disparities. The audit was committed and never acted on. Some are 5-minute polish; others are layout work; a few are product decisions in disguise.

This spec decides what's in scope, what's deferred, and how to sequence delivery so the easy wins ship before the hard ones.

## Scope

Three batches, each shippable independently. Batch A is ~3h of polish; Batch B is ~6h of layout; Batch C requires product input before any code is written.

### Batch A — Polish (in scope)

Five quick wins from the audit. Each is small, isolated, and visually obvious. All are unlocked by existing CSS tokens / API surfaces — no backend work required.

1. **OrgStatusPage** — replace the 4-cell `stat-grid` with 4 `info-cards` with `.ms` icon tiles in the top-right corner of each card. Audit §1.
2. **RolloutsPage** — add a thin progress bar derived from `current_phase_index / steps.length` inside `.status-pill.status-running` rows. Audit §17.
3. **FlagListPage** — rename the "Stale" stat label to "Stale 30d+" so the threshold is explicit. Audit §7.
4. **APIKeysPage** — add a 2-line `code-block` showing `dsctl apikey create --name <name>` (static copy; doesn't need to be live). Audit §15.
5. **RolloutGroupsPage** — add 3 glass-panel "bento" info cards (`Smart Targeting`, `Canary Releases`, `SDK Integration`) under the empty-state card. Static copy. Audit §18.

### Batch B — Layout (in scope)

Three layout-level items. Each is bigger than Batch A but still self-contained — no new endpoints, no schema changes.

6. **OrgStatusPage** — promote the per-app env chips into a 3-card env summary grid (version + pod count + health pulse). Audit §1.
7. **SettingsPage (org level)** — add an inheritance breadcrumb (`Acme Corp > E-Commerce > Staging`) above the settings editor. Data model already supports hierarchical scope resolution; this is purely visual. Audit §12.
8. **StrategyEditor** — replace the step-table with a column of `.strategy-step` cards connected by a vertical CSS `stepper-line` gradient. Indigo glow on active step. Audit §19.

### Batch C — Product decisions (deferred)

Four items that require a product call before any work happens. Each is parked here so they don't get lost; this spec does not implement them.

- **FlagDetailPage** 2-column layout (mockup §8) — does the team want to drop the tabbed single-column layout for a 2-column live-value + per-env evaluation traffic view? That's a significant rewrite of a 943-line page.
- **MembersPage** security-events feed (mockup §13) — needs an audit-log endpoint capable of filtering security-relevant events. Requires backend plumbing decision.
- **DocsPage** landing (mockup §20) — does the `/docs` (no slug) route get a marketing-style landing, or stay as a markdown renderer?
- **AnalyticsPage** quota-awareness banner (mockup §10) — depends on whether a quota API exists.

### Out of scope

- Audit §2 (org-environments) — the mockup is actually a `SettingsPage` env editor; it's a docs-mapping fix, not code work. Will be corrected in the audit doc itself, not as part of this initiative.
- Audit §5 (`project-applications.html` is 0 bytes) — already in the no-mockup list; nothing to address.
- Pixel tolerances and trivial token swaps — out of scope by the audit's own definition.
- `StrategiesPage` side-by-side list+builder rewrite (mockup §19 second half) — explicitly deferred by the audit.

## Approach

Each batch lives on its own branch and ships as its own PR. Within a batch, tasks can land independently — there are no shared dependencies between, e.g., the FlagListPage label rename and the RolloutsPage progress bar.

Implementation order: A first, then B. Batch C requires a separate spec once product input lands.

## Done when

- All 5 Batch A items merged on main, each verified visually against the corresponding mockup.
- All 3 Batch B items merged on main, each verified visually against the corresponding mockup.
- Batch C items remain noted in `Current_Initiatives.md` as "Awaiting product input" until decisions are made.
- The audit doc itself is updated to flag §2 mapping correction and to mark addressed items.

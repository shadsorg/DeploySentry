# UI Audit — Batch A Polish

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status**: Implementation
**Date**: 2026-04-29
**Spec**: [`../specs/2026-04-29-ui-audit-disparities-design.md`](../specs/2026-04-29-ui-audit-disparities-design.md)
**Audit reference**: [`../../ui-audit/MOCKUP_DISPARITIES.md`](../../ui-audit/MOCKUP_DISPARITIES.md)
**Estimated total**: ~3h

**Goal:** Land 5 small, visually obvious polish items from the UI audit. Each ships independently. No backend work, no schema changes, no new API surface.

**Branch:** `feature/ui-audit-batch-a` (single branch, single PR; tasks committed in order)

**Tech Stack:** React + TypeScript, existing CSS tokens (`.ms`, `.info-card`, `.status-pill`, `.code-block`, glass-panel utilities)

---

## Sequencing rationale

Tasks are listed cheapest-first so each commit is a clean checkpoint. The Stale label rename (Task 1) is a 2-minute change and proves the dev loop; the bento empty state (Task 5) is the largest at ~30 min.

---

### Task 1: FlagListPage — rename "Stale" stat to "Stale 30d+"

**File:** `web/src/pages/FlagListPage.tsx`
**Audit:** §7
**Estimate:** 2 min

- [ ] Find the stat-card label rendering "Stale" and change it to "Stale 30d+".
- [ ] Verify visually on the page — count value should be unchanged.

---

### Task 2: APIKeysPage — add CLI hint code block

**File:** `web/src/pages/APIKeysPage.tsx`
**Audit:** §15
**Estimate:** 5 min

- [ ] Below the existing stat strip (or above the keys table — match mockup placement), add a `.code-block` with two lines:
  ```
  $ dsctl apikey create --name <name>
  ```
  Followed by a tiny "Or use the dashboard form below" caption.
- [ ] Use existing `.code-block` styling — no new CSS.
- [ ] Static content; no live binding.

---

### Task 3: RolloutsPage — progress bar on running rollouts

**File:** `web/src/pages/RolloutsPage.tsx`
**Audit:** §17
**Estimate:** 15 min

- [ ] Inside each row whose status is `running`, render a thin progress bar using existing CSS variables. Width derived from `current_phase_index / steps.length`. If `steps.length` is 0 or undefined, hide the bar.
- [ ] Place the bar inside (or directly below) the `.status-pill.status-running` element so it visually belongs to that row.
- [ ] Use a single CSS class — no inline styles.
- [ ] Verify with at least one running rollout in dev (or fixtures).

---

### Task 4: OrgStatusPage — KPI icon tiles

**File:** `web/src/pages/OrgStatusPage.tsx`
**Audit:** §1 (icon tile portion only — the project-card matrix is Batch B)
**Estimate:** 20 min

- [ ] Swap the existing 4-cell `stat-grid` for 4 `info-cards` (or whatever the existing card class name is). Each card retains its current value + label.
- [ ] Add a top-right `.ms` icon tile to each: `sensors` (uptime), `memory` (latency), `dns` (deployments), and one more matching the 4th metric.
- [ ] Reuse existing icon-tile pattern from elsewhere in the codebase (`web/src/components/` or `globals.css`).
- [ ] Verify on a live dev page that the 4-up grid still wraps correctly at narrow widths.

---

### Task 5: RolloutGroupsPage — bento empty state

**File:** `web/src/pages/RolloutGroupsPage.tsx`
**Audit:** §18
**Estimate:** 30 min

- [ ] When the rollout-groups list is empty, below the existing empty-state CTA card, render 3 glass-panel info cards in a `grid-3`:
  1. **Smart Targeting** — short blurb on rule-based group membership.
  2. **Canary Releases** — short blurb on phased rollout.
  3. **SDK Integration** — short blurb on flag delivery to clients.
- [ ] Each card uses an existing glass-panel class + a `.ms` icon at the top.
- [ ] All copy is static — no API calls, no localization.
- [ ] Verify the bento cards do not appear when the list is non-empty.

---

## Verification (whole batch)

- [ ] `make run-web` shows all five changes in dev.
- [ ] `npm run build` (in `web/`) succeeds with no new TS errors.
- [ ] `npm run lint` (in `web/`) succeeds with no new warnings.
- [ ] Update `docs/ui-audit/MOCKUP_DISPARITIES.md` to mark §1 (icon tiles only), §7, §15, §17, §18 as ✅ addressed.

## PR

Single PR titled: `feat(web): UI audit Batch A — polish quick wins`. Body links to the audit doc and lists the 5 items with their audit section numbers.

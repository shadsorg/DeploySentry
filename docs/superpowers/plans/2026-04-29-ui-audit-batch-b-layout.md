# UI Audit — Batch B Layout

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status**: Design
**Date**: 2026-04-29
**Spec**: [`../specs/2026-04-29-ui-audit-disparities-design.md`](../specs/2026-04-29-ui-audit-disparities-design.md)
**Audit reference**: [`../../ui-audit/MOCKUP_DISPARITIES.md`](../../ui-audit/MOCKUP_DISPARITIES.md)
**Prerequisite**: Batch A merged (so the OrgStatusPage KPI cards are already in place when Task 1 below extends the project-card matrix).
**Estimated total**: ~6h

**Goal:** Three layout-level changes from the UI audit. Each is bigger than a Batch A polish item but still self-contained — no new endpoints, no schema changes.

**Branch:** `feature/ui-audit-batch-b` (single branch; one PR per task is fine if PRs are kept short, otherwise bundle).

**Tech Stack:** React + TypeScript, existing CSS tokens, SVG/CSS for the stepper line.

---

### Task 1: OrgStatusPage — project-card matrix with env card row

**File:** `web/src/pages/OrgStatusPage.tsx`
**Audit:** §1 (project matrix portion)
**Estimate:** ~45 min

**Context:** Today the page renders projects as a single-column list of `<button class="org-status-project-bar">` rows, with each app's environments shown as inline `env-chip` pills. The mockup wants a 2-column grid of project cards, each with a 3-column env card row showing version + pod count + health pulse.

- [ ] Replace the project-bar list with a 2-column grid of project cards. Use existing `.glass-panel` / `.info-card` styling.
- [ ] Each project card header: icon tile + project name + version chip + avatar stack (reuse what's there now).
- [ ] Inside each card, render a `grid-3` of small env cards. Each env card: env name, version, pod count, `.env-chip.health-*` pulse. Pod count and version come from the same `OrgStatus` payload that already feeds the page.
- [ ] Project card footer: keep existing latency / error / last-deployed metrics.
- [ ] Add filter pills at the top of the matrix (`All Projects` / `Degraded Only`) — pure client-side filter on the existing payload.
- [ ] Verify at narrow widths: matrix collapses to 1 column, env cards collapse to a vertical stack.

---

### Task 2: SettingsPage — inheritance breadcrumb

**File:** `web/src/pages/SettingsPage.tsx` (level=org / project / app)
**Audit:** §12
**Estimate:** ~30 min

**Context:** The hierarchical settings model (org > project > app > environment) already exists in the data layer. The UI doesn't visualize it, so users can't tell at a glance which scope they're editing or what it inherits from.

- [ ] Add an inheritance breadcrumb component at the top of the settings editor. Format: `Acme Corp > E-Commerce > Staging` (org > project > env, with the current scope bolded).
- [ ] At org level, show only the org segment.
- [ ] At project level, show org > project, with project bolded.
- [ ] At app level, show org > project > app.
- [ ] If a setting is inherited (not overridden at the current scope), badge it as `Inherited from <scope>`. The data model already supports scope resolution; this is purely visual.
- [ ] No layout reflow elsewhere — the breadcrumb sits in a row above the existing form, not in a left rail.

---

### Task 3: StrategyEditor — visual stepper

**Files:**
- `web/src/pages/StrategyEditor.tsx` (or `web/src/components/strategies/StrategyEditor.tsx` — verify before editing)
- `web/src/styles/globals.css` (add stepper-line styles if not already present)

**Audit:** §19 (Editor portion only — the side-by-side `StrategiesPage` rewrite is explicitly out of scope per spec)
**Estimate:** ~45 min

**Context:** Today the step editor renders steps as form rows in a table. The mockup shows numbered step nodes connected by a vertical SVG/CSS gradient line, each step a card with `wait time`, `bake time`, and `add condition` controls.

- [ ] Replace the step-table with a vertical column of `.strategy-step` cards.
- [ ] Add a CSS pseudo-element `::before` on the column container to render the connector line (linear gradient, indigo → transparent at the bottom).
- [ ] Active / current step: indigo glow ring (`box-shadow` token).
- [ ] Each card retains the existing form controls (`waitTime`, `bakeTime`, conditions). No data model changes.
- [ ] Drag-to-reorder is out of scope — keep existing up/down arrow controls if they exist.
- [ ] Verify the stepper still works correctly with 0, 1, 5, and 20 steps.

---

## Verification (whole batch)

- [ ] `make run-web` shows all three changes in dev. Test against fixtures with multiple projects, multiple settings scopes, and at least 5-step strategy.
- [ ] `npm run build` succeeds with no new TS errors.
- [ ] `npm run lint` succeeds with no new warnings.
- [ ] Update `docs/ui-audit/MOCKUP_DISPARITIES.md` to mark §1 (matrix portion), §12, §19 (editor portion) as ✅ addressed.

## PR

Title: `feat(web): UI audit Batch B — layout work (project matrix, inheritance breadcrumb, strategy stepper)`. Body links to spec + audit doc and lists each task's audit section.

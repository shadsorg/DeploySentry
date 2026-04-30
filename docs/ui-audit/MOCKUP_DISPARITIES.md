# UI Audit — Mockup ↔ Implementation Disparities

**Generated**: 2026-04-24
**Last addressed**: 2026-04-30 (Batch B+ — see [spec](../superpowers/specs/2026-04-29-ui-audit-disparities-design.md))
**Scope**: Phase-2 review of the Sentry Dark UI reskin.

> **Batch A (2026-04-29):** §1 (icon tile portion), §7, §15, §17, §18 ✅ merged via PR #67.
> **Batch B (2026-04-29):** §1 (project-card matrix), §12 (inheritance breadcrumb), §19 (editor stepper) ✅ merged via PR #68. §2 reclassified as docs-mapping (not code).
> **Batch B+ (2026-04-30):** §3, §6, §11, §14, §16 ✅ on PR #72.
>
> **Batch C (2026-04-30 — product input received):**
> - §4 latency/error chips → **future spec** at [`../superpowers/specs/2026-04-30-deploy-metrics-chips-design.md`](../superpowers/specs/2026-04-30-deploy-metrics-chips-design.md). UI placeholder uses `?` chips until ready.
> - §8 FlagDetailPage → **mockup direction rejected**, keeping tabs. 2-person approval extracted as future org-level option ([spec](../superpowers/specs/2026-04-30-two-person-approval-design.md)). Bigger product call: every dashboard mutation should go through a per-user staging layer with a "Deploy" review step before commit ([spec](../superpowers/specs/2026-04-30-staged-changes-and-deploy-workflow-design.md)).
> - §9 Active Rules Hierarchy stat strip → blocked on telemetry; ship `evaluations_24h` only when the read endpoint exists, dim `propagation` and `logic_failures` with "Coming soon".
> - §10 quota banner → **dropped**, no quota model planned.
> - §13 Recent Security Events feed → confirmed scope: login/logout + access changes (member.added/removed/role_changed, apikey.created/revoked, auth.login_failed/session_revoked). Implementation depends on the org-audit deliverable in [`../superpowers/specs/2026-04-30-flag-lifecycle-and-org-audit-design.md`](../superpowers/specs/2026-04-30-flag-lifecycle-and-org-audit-design.md).
> - §19 StrategiesPage list+builder → leave as-is for now.
> - §20 DocsPage → **TOC, not marketing landing**. Plan filed at [`../superpowers/plans/2026-04-30-docs-index-toc.md`](../superpowers/plans/2026-04-30-docs-index-toc.md), ready to implement.
**How this was built**: Each HTML mockup in `newscreens/` was read alongside
its React counterpart in `web/src/pages/`. Disparities are structural and
visual gaps a designer would notice on side-by-side review — NOT pixel
tolerances or trivial token swaps. Severity is:

- **high** — layout or feature gap (mockup has a chart/grid the page doesn't)
- **medium** — visible polish gap (empty states, icon tiles, status treatments)
- **low** — cosmetic / already mostly aligned

Companion artifacts: screenshots of the no-mockup pages live in
`docs/ui-audit/screenshots/`, one PNG per page.

> ⚠️ `newscreens/project-applications.html` is a **0-byte file** — it is not
> actually a mockup. `ProjectAppsTab.tsx` is in the "no-mockup" group below.

---

## 1. `org-overview.html` ↔ `OrgStatusPage.tsx`
**Severity**: high

**Disparities**
- Hero KPI strip: mockup has 4 glass-panel cards with ghosted icons in the top-right (`sensors`, `memory`, `dns`) plus a sparkline bar chart on uptime; page has a plain `stat-grid` (4 equal cells, no icons, no sparklines).
- Project matrix: mockup is a two-column grid of rich project cards — header (icon tile + name + version + avatar stack), three-env card row showing version + pod count + health pulse, footer with latency/error/last-deployed metrics. Page is a single-column list of `<button class="org-status-project-bar">` + inline `env-chip` pills.
- Mockup has `Project Status Matrix` heading with filter pills (`All Projects` / `Degraded Only`); page has no per-view filter.
- No "Active Rollouts" / "Config Flow" panels on the page — mockup implies a third content block.

**Quick wins**
- Swap the 4-cell `stat-grid` for 4 `info-cards` with `.ms` icon tiles in the top-right corner (≈20 min).
- Promote the app-row env chips to small cards (version + pod count) within a `grid-3` inside each project (≈45 min).

---

## 2. `org-environments.html` ↔ `OrgStatusPage.tsx` (environments tab)
**Severity**: medium

**Disparities**
- Mockup is actually an **org settings screen** — a sticky left "Add Environment" form and a right-side "Active Fleet" table. It does not belong on OrgStatusPage; it's a better match for `SettingsPage level="org"` envs tab.
- Page has no two-column form+table layout at the org level.
- Mockup warning banner ("slugs are immutable once used") is absent from the env form in `SettingsPage`.

**Quick wins**
- Move this audit pair to point at `SettingsPage` envs section (docs change only).
- Add the immutability warning banner near the env-slug input (≈10 min).

---

## 3. `deploy-history.html` ↔ `OrgDeploymentsPage.tsx`
**Severity**: medium

**Disparities**
- Mockup is an "Audit Log" timeline — vertical connector line, step icons (`rocket_launch`, `verified`, `edit`), glass-panel entries with commit refs and inline user attribution.
- Page is a conventional filter-sidebar + `org-deployments-row` table (`grid-template-columns: 120px minmax(260px,2fr) 120px 110px 110px 160px`).
- Mockup's "Critical Events" sidebar callouts and animated-ping status dots are absent on the page.

**Quick wins**
- Keep the table, but add a `Critical Events` aside column that pulls the last 3 failed deployments — reuses existing `status-pill.status-failed` styling (≈30 min).

---

## 4. `deployment-history.html` ↔ `DeploymentsPage.tsx`
**Severity**: medium

**Disparities**
- Same audit-log layout as #3 at the app level. Page is a similar row-table pattern.
- Mockup per-row footer chips (`Latency: 12ms`, `Errors: 0.01%`) — page doesn't surface these on the list row.
- Mockup shows progress bars on in-flight rollouts; page only shows a status pill.

**Quick wins**
- Add a thin progress bar inside `.status-pill.status-running` for in-flight rows (≈20 min).

---

## 5. `project-applications.html` (EMPTY) ↔ `ProjectAppsTab.tsx`
**Severity**: n/a — no mockup exists
- The mockup file is 0 bytes. See `docs/ui-audit/screenshots/project-shell.png` + note below. Add to the no-mockup list.

---

## 6. `project-application.html` ↔ `AppPage.tsx`
**Severity**: medium

**Disparities**
- Mockup has a rich app-shell header: icon tile + app name + version chip + a 3-col env status grid below. Page is a bare `content-header` + tabs row (no version chip, no env grid).
- Mockup's "Total Applications / Avg Uptime / Active Deployments" tri-KPI strip on the project page is missing.

**Quick wins**
- Add a 3-card env summary below the AppPage header, reusing the `.env-chip.health-*` styles (≈25 min).

---

## 7. `flag-application.html` ↔ `FlagListPage.tsx`
**Severity**: medium

**Disparities**
- Page already has a stat strip (Flags / Active / Stale / Archived) — aligned with mockup's "Total Flags / Active / Stale Flags / Owners" but mockup's "Stale 30+ days" definition is clearer; page label is just "Stale".
- Mockup has a "Flag Performance" bar chart panel; page has no chart.
- Mockup flag-row composition is card-like with big category badge + owner avatar + env dots; page is a table row.

**Quick wins**
- Rename the Stale stat to `Stale 30d+` (2 min) and hyperlink the owner count to MembersPage (5 min).

---

## 8. `flag-details.html` ↔ `FlagDetailPage.tsx`
**Severity**: medium

**Disparities**
- Mockup is a 2-column layout: left = live value panel, right = environment cards with per-env evaluation traffic and approval banner (`Changes to this environment require a 2-person approval`).
- Page is a tabbed single-column layout (54 inline styles, 943 lines).
- Mockup's "Targeting Summary" preview panel (rule list with natural-language `IF email ends with @infracore.io THEN serve TRUE`) is absent — page jumps straight into rule-builder UI.

**Quick wins**
- Add a read-only "Targeting Summary" block above the editable rules (natural-language stringification of each rule — ≈45 min).

---

## 9. `targeting-rules.html` ↔ `FlagDetailPage.tsx` (targeting section)
**Severity**: high

**Disparities**
- Mockup includes an "Active Rules Hierarchy" mini-dashboard with big numbers (`1.2M` evaluations, `100%` propagation, `0` logic failures) at the top.
- Page drops users straight into the rule editor with no evaluation stats.
- Mockup has clearly-separated "Create New Rule" CTA with inline segment chips; page uses a button + modal.

**Quick wins**
- Surface the existing targeting stats from the API in a 3-card header strip (≈30 min).

---

## 10. `project-analytics.html` ↔ `AnalyticsPage.tsx`
**Severity**: medium

**Disparities**
- Mockup has a 4-card KPI strip (Flag Evaluations / Unique Users / Uptime SLO / Avg Latency) with colored sparkline indicators; page already has an `analytics.css` with chart classes but the layout density differs.
- Mockup includes a "Resource Monitoring" upgrade-nag banner ("85% of current request limit"); page has no quota-awareness banner.

**Quick wins**
- If the quota API exists, add the banner. Otherwise skip — that's a product decision.

---

## 11. `project-settings.html` ↔ `SettingsPage.tsx` (level=project)
**Severity**: low

**Disparities**
- Mockup has sectioned cards: `General Information`, `Routing`, `Cleanup`, `Project Visibility`, `Auto-Sync Repository`, `Delete this project`. Page structure matches, but section borders/cards may be less distinct.
- Mockup toggles on `Auto-Sync Repository` are pill-style; page uses the existing `.toggle-switch`.

**Quick wins**
- Audit section spacing to match mockup (≈10 min CSS tweak).

---

## 12. `environment-config.html` ↔ `SettingsPage.tsx` (level=org)
**Severity**: high

**Disparities**
- Mockup is a complex 3-pane layout: left inheritance breadcrumb (`Acme Corp > E-Commerce > Staging`), middle env/config table with masked secret values (`postgres://admin:********@…`), right "Unpublished Draft — 3 changes pending" panel.
- Page has flat settings lists per level; no inheritance visualization, no draft-state indicator, no secret masking UI.

**Quick wins**
- Add the inheritance breadcrumb atop the settings editor (≈30 min). The data model already supports hierarchical scope resolution — this is just a visual surface.

---

## 13. `members-permissions.html` ↔ `MembersPage.tsx`
**Severity**: medium

**Disparities**
- Mockup has a 3-panel layout: `Team Directory` table on left, `Role: Editor` permission editor in the middle showing `Deployment Privileges` + `Infrastructure Access` checkbox matrix, `Recent Security Events` feed on the right.
- Page is a single table + invite modal — no permission-matrix visualization, no security feed.

**Quick wins**
- Add a right-side activity feed using `activity-log` classes (≈20 min — the data model likely has an audit-log endpoint).

---

## 14. `member-group.html` ↔ `MembersPage.tsx` (group/role view)
**Severity**: medium

**Disparities**
- Same structure as #13; the only difference is the middle column is filtered to a specific role. Page has no concept of "role group" as a standalone view.

**Quick wins**
- Add role filter pills above the members table so clicking one filters + shows the role's policy (≈25 min).

---

## 15. `api-key-management.html` ↔ `APIKeysPage.tsx` (list)
**Severity**: medium

**Disparities**
- Mockup has a 3-card KPI strip at top (`Total Requests (24h) 1.28M`, `Active Keys 14`, `Avg. Latency 42ms`) + a security-best-practices nag banner.
- Page has a plain stat strip (total / active / expiring / revoked) — counts only, no latency or request-volume metrics.
- Mockup includes a terminal-style "Use our official CLI" code block; page has no CLI prompt surface.

**Quick wins**
- Add a 2-line `code-block` that shows `dsctl apikey create --name <name>` (≈5 min) — it doesn't need to be live.

---

## 16. `api-key-detailed.html` ↔ `APIKeysPage.tsx` (detail/create flow)
**Severity**: low

**Disparities**
- Mockup shows a "Create New API Key" modal with scope selection cards (`Deploy`, `Read`, `Admin`) and a reveal box for the generated token.
- Page already has a reveal box (`.key-reveal`) but scope selection is a multi-select checkbox list, not scope cards.

**Quick wins**
- Swap the scope checklist for a `checkbox-group` styled as selection cards (≈20 min).

---

## 17. `rollouts-active.html` ↔ `RolloutsPage.tsx`
**Severity**: medium

**Disparities**
- Mockup has a `Critical Events` panel (failed nodes, verification passed, policy applied) — a side-stream of rollout-adjacent infra events. Page doesn't surface these.
- Mockup rollout rows include a progress bar showing `% rolled out`; page shows current phase number only.
- Mockup includes a heatmap-style replica grid; page does not.

**Quick wins**
- Add a progress bar derived from `current_phase_index / steps.length` (≈15 min).

---

## 18. `rollout-groups.html` ↔ `RolloutGroupsPage.tsx`
**Severity**: low

**Disparities**
- Mockup's empty state is rich: radial-gradient background + large icon + dual CTAs (`Create First Group`, `Import Segments`) + 3 "bento" info cards below.
- Page empty state is minimal (icon + heading + paragraph + single button).

**Quick wins**
- Borrow the bento pattern: under the empty-state card, add 3 glass-panel info cards (`Smart Targeting`, `Canary Releases`, `SDK Integration`) — all static copy (≈30 min).

---

## 19. `rollout-strategy.html` ↔ `StrategiesPage.tsx` + `StrategyEditor.tsx`
**Severity**: high

**Disparities**
- Mockup is a 4/8 column layout: left = strategy picker cards with "ACTIVE" badge + indigo glow, right = **visual strategy builder** with numbered step nodes connected by a vertical SVG `stepper-line` gradient.
- Page `StrategiesPage` is a flat table; `StrategyEditor` is a modal with a form (no visual canvas).
- Mockup's builder shows each step as a "wait time / bake time / add condition" card — page shows as form rows in a table.

**Quick wins**
- In `StrategyEditor`, replace the step-table with a column of `.strategy-step` cards connected by a vertical CSS line (partial work already done — add the connector line and indigo-glow on active step). ≈45 min.
- `StrategiesPage` side-by-side list+builder is a bigger refactor — park for a later phase.

---

## 20. `documentation.html` ↔ `DocsPage.tsx` (+ `components/docs/*`)
**Severity**: low

**Disparities**
- Mockup has a rich hero intro with radial-gradient bg, dual-card layout ("Managed Infrastructure" vs "Self-Host"), and a "Ready for the next step?" CTA at the bottom.
- Page (DocsPage) is a straight markdown renderer with a left sidebar — no intro hero, no landing CTAs.
- This is likely intentional: DocsPage renders per-slug markdown, not a landing; the mockup may be for a `/docs` index page.

**Quick wins**
- If the `/docs` (no slug) route should show a landing, add a `DocsLanding` component using the mockup (≈60 min). Otherwise close as "not applicable".

---

## Summary by severity

| Severity | Count | Pages |
|---|---|---|
| high | 4 | OrgStatusPage, FlagDetailPage (targeting), SettingsPage (org env-config), StrategyEditor/StrategiesPage |
| medium | 10 | OrgDeploymentsPage, DeploymentsPage, AppPage, FlagListPage, FlagDetailPage (detail), AnalyticsPage, MembersPage, APIKeysPage (list), RolloutsPage, org-environments |
| low | 4 | ProjectAppsTab, SettingsPage (project), APIKeysPage (detail), RolloutGroupsPage, DocsPage |
| n/a | 1 | project-applications.html is empty |

## Recommended sequence

1. **Batch A — low-hanging polish** (~3 h total): bento empty state on RolloutGroupsPage, progress bar on RolloutsPage, KPI icons on OrgStatusPage, quota banner on AnalyticsPage, CLI code block on APIKeysPage.
2. **Batch B — layout work** (~6 h total): OrgStatusPage project-card matrix, SettingsPage env-config inheritance breadcrumb, StrategyEditor visual stepper.
3. **Batch C — product decisions needed**: FlagDetailPage 2-column layout (does the team want to keep tabs?), MembersPage security-events feed (audit-log API plumbing), DocsPage landing (do we even want one?).

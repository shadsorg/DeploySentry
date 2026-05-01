# Current Initiatives

> Last updated: 2026-05-01 (Staged Changes Phase A + B + C-5 plumbing on main via PRs #86 / #87 / #94; backend handlers C-1/C-2/C-3/C-4 + UI conversions C-6 through C-11 + operator toggle C-12 in PR queue; UI audit §13 MembersPage RecentMemberActivity shipped via PR #82; Deliverable 2 shipped via PR #80; UI audit §20 docs index shipped)

## Active (not yet complete)

| Initiative | Phase | Plan/Spec File | Notes |
|---|---|---|---|
| E2E SDK Flag Delivery — remaining | Implementation | [Spec](./superpowers/specs/2026-04-12-e2e-sdk-flag-delivery-design.md) | Main suite merged in PR #34 (2026-04-12). React probe re-enabled in Scenario A (root cause: react-harness vite config aliased `@dr-sentry/react` to a path that no longer existed once the SDK switched to dual ESM/CJS via `exports`). Still pending: fault injection (manual one-time verification per spec §"Testing the Tests"), required-gate flip in branch protection (after a soak). |
| Identity & Provenance | Design | [Plan](./superpowers/plans/2026-04-16-identity-and-provenance.md) | External work: acquire deploysentry.com, create CrowdSoftApps GitHub org, npm publisher identity, close provenance loop. |
| Sidecar Traffic Management | Design | [Spec](./superpowers/specs/2026-04-17-sidecar-traffic-management-design.md) / [Plan](./superpowers/plans/2026-04-17-sidecar-traffic-management.md) | Envoy-based sidecar for canary traffic splitting, header routing, and observability. |
| Cloudflare Canary & Observability | Design (deferred) | [Spec](./superpowers/specs/2026-04-22-cloudflare-canary-and-observability-design.md) | PaaS-friendly binary-level canary via edge weighted routing (Cloudflare LB first, pluggable `TrafficRouter`) + provider-agnostic abort signals. Implementation deferred until customer need arises. |
| First-Class Deploy Gates per Environment | Design | [Spec](./superpowers/specs/2026-04-22-deploy-gate-environments-design.md) | External feature request from jobmgr (CrowdSoftApps): let operators control "is this env open for deploys?" from the DS UI and make DS — not GitHub Actions — the authoritative trigger for gated environment deploys. Not yet scheduled. |
| Org Status — Phase 4 polish | Design | [Spec](./superpowers/specs/2026-04-23-org-status-and-deploy-history-design.md) / [Phase 4 Plan](./superpowers/plans/2026-04-29-org-status-phase4-polish.md) | Phases 1–3 shipped on main. Phase 4 deferred items split out: ETag client caching on `/orgs/:slug/status`, CSV export on `OrgDeploymentsPage`, org-default monitoring-link templates. |
| Build Status — deferred follow-ups | Design | [Plan](./superpowers/plans/2026-04-29-build-status-deferred-followups.md) | Parent initiative complete; non-blocking follow-ups: handler unit tests, Combobox Vitest, Playwright smoke, MCP `deploy_create` tool update, stale-build sweep. |
| UI Audit Disparities | Implementation (Batch C) | [Spec](./superpowers/specs/2026-04-29-ui-audit-disparities-design.md) / [Audit doc](./ui-audit/MOCKUP_DISPARITIES.md) | Addresses 2026-04-24 audit (4 high / 10 medium / 4 low). A merged (#67), B merged (#68), B+ merged (#72). Batch C: §20 docs index merged (#75); §13 MembersPage RecentMemberActivity merged (#82). §10 dropped. §8 → staged-changes spec instead. Batch C complete pending any leftover §13 follow-ups (e.g. members handler tests). |
| Staged Changes + Deploy Review | Implementation (Phase C tail / pre-rollout) | [Spec](./superpowers/specs/2026-04-30-staged-changes-and-deploy-workflow-design.md) / [Plan](./superpowers/plans/2026-05-01-staged-changes-and-deploy-workflow.md) | UI mutations go to a per-user `staged_changes` table; header shows "N pending → Deploy"; review page lets the user select/discard before commit. Replaces today's immediate-write semantics for dashboard edits. SDK/CLI/webhook writes unchanged. **On main:** Phase A backend foundation (#86), Phase B header banner + review page (#87), Phase C-5 plumbing (`useStagingEnabled` + `stageOrCall` hooks) + first UI conversion (FlagDetailPage archive/restore) (#94). **In PR queue (independent):** backend commit handlers — C-1 flag.update/archive/restore (#89), C-2 rule + per-env state (#91), C-3 settings + member.role_changed (#92), C-4 rollout family (#93); UI conversions — C-6 env-state (#95), C-7 rule update/delete (#96), C-8 per-rule per-env (#97), C-9 flag settings (#98), C-10 members role change (#99), C-11 strategy update/delete (#100); C-12 operator-facing `StagingModeToggle` on DeployChangesPage (#101). **Remaining (not started):** provisional-id resolution + `*.create` commit handlers (needs design pass), real feature-flag-backed gate (replaces localStorage backing in `useStagingEnabled`), one-time org-wide enablement migration. |
| Two-Person Approval (org option) | Design (future) | [Spec](./superpowers/specs/2026-04-30-two-person-approval-design.md) | Configurable per-org gate that converts certain Deploy actions into approval requests. Builds on staged-changes; same-spec interaction at the commit boundary. |
| Deploy Metrics Chips | Design (future) | [Spec](./superpowers/specs/2026-04-30-deploy-metrics-chips-design.md) | Per-deploy latency/error-rate chips on list rows. Pulls from rollout health gates when present, else snapshots from the configured health source at T+1h/4h/24h vs a 24h pre-deploy baseline. |
| MCP Flag Routing Fix | Implementation | [Plan](./superpowers/plans/2026-05-01-mcp-flag-routing-fix.md) | `ds_list_flags` / `ds_create_flag` 404 — MCP tools targeted a non-existent nested URL. Realigning to the flat `/api/v1/flags` route and relaxing `createFlagRequest.ProjectID` to slug-or-UUID for parity with `evaluateRequest`. |
| Migration Drift Gate | Design | [Plan](./superpowers/plans/2026-05-01-migration-drift-gate.md) | Boot-time check + CI gate so the running binary refuses to serve (or loudly warns) when its bundled migrations are ahead of `schema_migrations.version`. Surfaced during PR #88 smoke-test: post-#80 binary against pre-060 DB silently broke every flag read with a generic "failed to list flags" 500. |

## Archived (recently)

All initiatives previously listed as "Implementation — pending merge" have shipped. Plans and specs moved to `docs/archives/` on 2026-04-29:

- **MembersPage RecentMemberActivity (UI audit §13)** — PR #82 merged 2026-05-01. Audit writes for `member.added` / `member.removed` / `member.role_changed` wired in `internal/members/handler.go`; compact panel mounted at the top of `MembersPage`. Login/logout writes deferred to a future personal-events stream design.
- **Org Audit Page + Revert (Deliverable 3)** — PR #77 merged 2026-04-30. Backend revert registry + `OrgAuditPage` shipped. Sidebar role-gate follow-up merged via PR #79.
- **Flag Hard Delete + Retention (Deliverable 2)** — PR #80 merged 2026-05-01. Migration 060, `POST /flags/:id/queue-deletion` (+ `DELETE` cancel) + `POST /flags/:id/restore` + `DELETE /flags/:id?force=true` with `X-Confirm-Slug`, retention sweep with system audit writes, Settings-tab Lifecycle panel.
- **Mobile PWA Production Serve** — PR #65 merged 2026-04-30. PWA served from API binary at `/m/*`.

- **Canary Rollout E2E** — PR #36 merged 2026-04-13.
- **API Security Hardening** — PR #37 merged 2026-04-13.
- **Project & App Edit/Delete** — PR #39 merged 2026-04-14, migration 039.
- **SDK Application Parameter** — merged on main, both SDKs.
- **Groups & Resource Authorization** — merged on main, migration 041.
- **Environment-Scoped API Keys** — merged on main, migration 042.
- **MCP Server & Deploy Onboarding** — merged on main, 12 MCP tools + `deploysentry mcp serve`.
- **SDK npm Publishing** — `@dr-sentry/sdk@1.1.0` and `@dr-sentry/react@1.1.0` published to npm.
- **Configurable Rollout Strategies** — all 5 plans merged: foundation, engine+deploy, config rollouts, releases+coordination, web UI.
- **Feature Lifecycle Layer** — PR #45 merged 2026-04-19, migration 052.
- **Org Status & Deploy History (P1–P3)** — backend, OrgStatusPage, OrgDeploymentsPage all on main. P4 split out as its own plan above.
- **Build Status Ingestion & Deploy-Create Autocomplete** — all 4 runtime phases on main, smoke runner shipped. Deferred items split out as their own plan above.
- **CLI Flag Flow Fix + Tests** — PRs #56 + #57 merged 2026-04-28.
- **CLI Self-Update** — PR #58 merged 2026-04-28.
- **Navigation Redesign** — `ProjectPage` + `AppPage` wrappers landed; sidebar simplified.
- **Targeting Rules Per Environment** — `5289a9f` merged.
- **YAML Flag Config** — `33d563b` (export endpoint) merged.
- **Deploy Onboarding & API Key Scoping** — migration 045 + `Deploy_Monitoring_Setup.md` merged.
- **Docs & MCP Traffic Tools** — `tools_traffic.go` + `Traffic_Management_Guide.md` merged.
- **Flag Detail Enhancements** — Settings + History tabs, audit wiring on main.
- **Reintegrate Security Features** — `AllowedCIDRs`, `FlagActivityChecker`, `SessionManager.BlacklistToken` all on main.
- **CLI Auth + Onboarding Fix** — `a9d348c` merged (API-key login replacing broken OAuth).
- **Mobile PWA (P1–P6)** — already archived 2026-04-28; full archive at `./archives/2026-04-24-mobile-pwa-design.md`.

See `docs/archives/` for historical plans, specs, and their completion records.

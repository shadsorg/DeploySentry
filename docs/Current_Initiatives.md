# Current Initiatives

> Last updated: 2026-04-30 (Deliverable 3 shipped; Deliverable 2 plan filed)

## Active (not yet complete)

| Initiative | Phase | Plan/Spec File | Notes |
|---|---|---|---|
| Flag Hard Delete + Retention (Deliverable 2) | Design | [Spec](./superpowers/specs/2026-04-30-flag-lifecycle-and-org-audit-design.md) / [Plan](./superpowers/plans/2026-04-30-flag-hard-delete.md) | Deliverable 2 of the flag-lifecycle/org-audit spec. Migration 060 adds `delete_after`/`deleted_at`; new endpoints `POST /flags/:id/queue-deletion`, `DELETE /flags/:id?force=true`, `POST /flags/:id/restore`; retention sweep job tombstones expired flags; Settings tab Lifecycle panel for active/within-retention/elapsed states. **Phase 0 fixes the latent bug where `service.ArchiveFlag` never persists `archived_at`** — precondition without which hard-delete cannot work. Branch: `feature/flag-hard-delete`. |
| Org Audit Page + Revert (Deliverable 3) | Complete | [Spec](./superpowers/specs/2026-04-30-flag-lifecycle-and-org-audit-design.md) / [Plan](./superpowers/plans/2026-04-30-org-audit-and-revert.md) | Shipped via PR #77 on 2026-04-30 (merge `04b5915`). Org audit page at `/orgs/:orgSlug/audit` with old/new diff + one-click revert; backend revert registry maps `(entity_type, action) → existing service-layer method`. Reverts are themselves audit-logged. Follow-ups: sidebar role-gating, `flags.ErrNotFound` sentinel to replace `strings.Contains` not-found checks, MembersPage security events feed. To be archived after a soak. |
| E2E SDK Flag Delivery — remaining | Implementation | [Spec](./superpowers/specs/2026-04-12-e2e-sdk-flag-delivery-design.md) | Main suite merged in PR #34 (2026-04-12). Still pending: React probe (sdk/react ESM fix), fault injection, required-gate test. |
| Identity & Provenance | Design | [Plan](./superpowers/plans/2026-04-16-identity-and-provenance.md) | External work: acquire deploysentry.com, create CrowdSoftApps GitHub org, npm publisher identity, close provenance loop. |
| Sidecar Traffic Management | Design | [Spec](./superpowers/specs/2026-04-17-sidecar-traffic-management-design.md) / [Plan](./superpowers/plans/2026-04-17-sidecar-traffic-management.md) | Envoy-based sidecar for canary traffic splitting, header routing, and observability. |
| Cloudflare Canary & Observability | Design (deferred) | [Spec](./superpowers/specs/2026-04-22-cloudflare-canary-and-observability-design.md) | PaaS-friendly binary-level canary via edge weighted routing (Cloudflare LB first, pluggable `TrafficRouter`) + provider-agnostic abort signals. Implementation deferred until customer need arises. |
| First-Class Deploy Gates per Environment | Design | [Spec](./superpowers/specs/2026-04-22-deploy-gate-environments-design.md) | External feature request from jobmgr (CrowdSoftApps): let operators control "is this env open for deploys?" from the DS UI and make DS — not GitHub Actions — the authoritative trigger for gated environment deploys. Not yet scheduled. |
| Org Status — Phase 4 polish | Design | [Spec](./superpowers/specs/2026-04-23-org-status-and-deploy-history-design.md) / [Phase 4 Plan](./superpowers/plans/2026-04-29-org-status-phase4-polish.md) | Phases 1–3 shipped on main. Phase 4 deferred items split out: ETag client caching on `/orgs/:slug/status`, CSV export on `OrgDeploymentsPage`, org-default monitoring-link templates. |
| Build Status — deferred follow-ups | Design | [Plan](./superpowers/plans/2026-04-29-build-status-deferred-followups.md) | Parent initiative complete; non-blocking follow-ups: handler unit tests, Combobox Vitest, Playwright smoke, MCP `deploy_create` tool update, stale-build sweep. |
| Mobile PWA Production Serve | Implementation | [Plan](./superpowers/plans/2026-04-29-mobile-pwa-prod-serve.md) | PR #65 open on `feature/mobile-pwa-prod-serve` — embeds built PWA into API binary at `/m/*`. CI failing on Lint, Lint UI, E2E SDK Tests; needs diagnosis before merge. |
| UI Audit Disparities | Implementation (Batch A) | [Spec](./superpowers/specs/2026-04-29-ui-audit-disparities-design.md) / [Batch A Plan](./superpowers/plans/2026-04-29-ui-audit-batch-a-polish.md) / [Batch B Plan](./superpowers/plans/2026-04-29-ui-audit-batch-b-layout.md) / [Audit doc](./ui-audit/MOCKUP_DISPARITIES.md) | Addresses 2026-04-24 audit (4 high / 10 medium / 4 low). A merged (#67), B merged (#68), B+ open (#72). Batch C answers received 2026-04-30 — see audit doc for per-item disposition. Remaining build: §20 docs index ([plan](./superpowers/plans/2026-04-30-docs-index-toc.md)), §13 once org-audit ships. §10 dropped. §8 → staged-changes spec instead. |
| Staged Changes + Deploy Review | Design | [Spec](./superpowers/specs/2026-04-30-staged-changes-and-deploy-workflow-design.md) | UI mutations go to a per-user `staged_changes` table; header shows "N pending → Deploy"; review page lets the user select/discard before commit. Replaces today's immediate-write semantics for dashboard edits. SDK/CLI/webhook writes unchanged. |
| Two-Person Approval (org option) | Design (future) | [Spec](./superpowers/specs/2026-04-30-two-person-approval-design.md) | Configurable per-org gate that converts certain Deploy actions into approval requests. Builds on staged-changes; same-spec interaction at the commit boundary. |
| Deploy Metrics Chips | Design (future) | [Spec](./superpowers/specs/2026-04-30-deploy-metrics-chips-design.md) | Per-deploy latency/error-rate chips on list rows. Pulls from rollout health gates when present, else snapshots from the configured health source at T+1h/4h/24h vs a 24h pre-deploy baseline. |

## Archived (recently)

All initiatives previously listed as "Implementation — pending merge" have shipped. Plans and specs moved to `docs/archives/` on 2026-04-29:

- **Org Audit Page + Revert (Deliverable 3)** — PR #77 merged 2026-04-30. Backend revert registry + `OrgAuditPage` shipped. Sidebar role-gate follow-up on `chore/org-audit-cleanup`.

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

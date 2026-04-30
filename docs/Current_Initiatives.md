# Current Initiatives

> Last updated: 2026-04-29 (post-audit cleanup)

## Active (not yet complete)

| Initiative | Phase | Plan/Spec File | Notes |
|---|---|---|---|
| E2E SDK Flag Delivery — remaining | Implementation | [Spec](./superpowers/specs/2026-04-12-e2e-sdk-flag-delivery-design.md) | Main suite merged in PR #34 (2026-04-12). Still pending: React probe (sdk/react ESM fix), fault injection, required-gate test. |
| Identity & Provenance | Design | [Plan](./superpowers/plans/2026-04-16-identity-and-provenance.md) | External work: acquire deploysentry.com, create CrowdSoftApps GitHub org, npm publisher identity, close provenance loop. |
| Sidecar Traffic Management | Design | [Spec](./superpowers/specs/2026-04-17-sidecar-traffic-management-design.md) / [Plan](./superpowers/plans/2026-04-17-sidecar-traffic-management.md) | Envoy-based sidecar for canary traffic splitting, header routing, and observability. |
| Cloudflare Canary & Observability | Design (deferred) | [Spec](./superpowers/specs/2026-04-22-cloudflare-canary-and-observability-design.md) | PaaS-friendly binary-level canary via edge weighted routing (Cloudflare LB first, pluggable `TrafficRouter`) + provider-agnostic abort signals. Implementation deferred until customer need arises. |
| First-Class Deploy Gates per Environment | Design | [Spec](./superpowers/specs/2026-04-22-deploy-gate-environments-design.md) | External feature request from jobmgr (CrowdSoftApps): let operators control "is this env open for deploys?" from the DS UI and make DS — not GitHub Actions — the authoritative trigger for gated environment deploys. Not yet scheduled. |
| Org Status — Phase 4 polish | Design | [Spec](./superpowers/specs/2026-04-23-org-status-and-deploy-history-design.md) / [Phase 4 Plan](./superpowers/plans/2026-04-29-org-status-phase4-polish.md) | Phases 1–3 shipped on main. Phase 4 deferred items split out: ETag client caching on `/orgs/:slug/status`, CSV export on `OrgDeploymentsPage`, org-default monitoring-link templates. |
| Build Status — deferred follow-ups | Design | [Plan](./superpowers/plans/2026-04-29-build-status-deferred-followups.md) | Parent initiative complete; non-blocking follow-ups: handler unit tests, Combobox Vitest, Playwright smoke, MCP `deploy_create` tool update, stale-build sweep. |
| Mobile PWA Production Serve | Implementation | [Plan](./superpowers/plans/2026-04-29-mobile-pwa-prod-serve.md) | PR #65 open on `feature/mobile-pwa-prod-serve` — embeds built PWA into API binary at `/m/*`. CI failing on Lint, Lint UI, E2E SDK Tests; needs diagnosis before merge. |
| UI Audit Disparities | Implementation (Batch A) | [Spec](./superpowers/specs/2026-04-29-ui-audit-disparities-design.md) / [Batch A Plan](./superpowers/plans/2026-04-29-ui-audit-batch-a-polish.md) / [Batch B Plan](./superpowers/plans/2026-04-29-ui-audit-batch-b-layout.md) / [Audit doc](./ui-audit/MOCKUP_DISPARITIES.md) | Addresses 2026-04-24 audit (4 high / 10 medium / 4 low). Batch A = 5 polish quick wins (~3h); Batch B = 3 layout items (~6h); Batch C = product decisions, deferred. |

## Archived (recently)

All initiatives previously listed as "Implementation — pending merge" have shipped. Plans and specs moved to `docs/archives/` on 2026-04-29:

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

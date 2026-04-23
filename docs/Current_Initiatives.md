# Current Initiatives

> Last updated: 2026-04-23

## Active (not yet complete)

| Initiative | Phase | Plan/Spec File | Notes |
|---|---|---|---|
| Canary Rollout E2E | Implementation | On `feature/canary-rollout-e2e` branch (PR #36) | All 3 specs implemented (canary E2E, reliability hardening, all-strategies + live polling). Pending merge. |
| E2E SDK Flag Delivery | Implementation | [Plan](./superpowers/plans/2026-04-12-e2e-sdk-flag-delivery.md) / [Spec](./superpowers/specs/2026-04-12-e2e-sdk-flag-delivery-design.md) | 5 tests passing (Node-only); React probe deferred pending sdk/react ESM fix; fault injection + required-gate pending |
| Project & App Edit/Delete | Implementation | [Plan](./superpowers/plans/2026-04-15-project-app-edit-delete.md) / [Spec](./superpowers/specs/2026-04-13-project-edit-delete-design.md) | Implemented on main. Migration 039, backend + frontend complete. Needs manual testing. |
| SDK Application Parameter | Implementation | [Plan](./superpowers/plans/2026-04-15-sdk-application-parameter.md) / [Spec](./superpowers/specs/2026-04-15-sdk-application-parameter-design.md) | Implemented on main. Both SDKs + backend complete. All tests pass. |
| Groups & Resource Authorization | Implementation | [Plan](./superpowers/plans/2026-04-16-groups-and-resource-authorization.md) / [Spec](./superpowers/specs/2026-04-16-groups-and-resource-authorization-design.md) | On `feature/groups-and-resource-authorization` branch. Migration 041, groups/grants packages, visibility filtering, frontend complete. Pending merge. |
| Environment-Scoped API Keys | Implementation | [Plan](./superpowers/plans/2026-04-16-environment-scoped-api-keys.md) / [Spec](./superpowers/specs/2026-04-16-environment-scoped-api-keys-design.md) | On `feature/groups-and-resource-authorization` branch. Migration 042, model/repo/service/handler/middleware/CLI/UI complete. Pending merge. |
| MCP Server & Deploy Onboarding | Implementation | [Plan](./superpowers/plans/2026-04-16-mcp-server-deploy-onboarding.md) / [Spec](./superpowers/specs/2026-04-16-mcp-server-deploy-onboarding-design.md) | On `feature/groups-and-resource-authorization` branch. 12 MCP tools, stdio transport, `deploysentry mcp serve`. Pending merge. |
| Identity & Provenance | Design | [Plan](./superpowers/plans/2026-04-16-identity-and-provenance.md) | Acquire deploysentry.com, create CrowdSoftApps GitHub org, npm publisher identity, close provenance loop. |
| Sidecar Traffic Management | Design | [Spec](./superpowers/specs/2026-04-17-sidecar-traffic-management-design.md) / [Plan](./superpowers/plans/2026-04-17-sidecar-traffic-management.md) | Envoy-based sidecar for canary traffic splitting, header routing, and observability. |
| SDK npm Publishing | Implementation | [Plan](./superpowers/plans/2026-04-16-sdk-npm-publishing.md) | Dual ESM/CJS build + publish `@dr-sentry/sdk` and `@dr-sentry/react` to npm; unblocks Railway/Render deploys and `vite build`. |
| Configurable Rollout Strategies | Complete | [Spec](./superpowers/specs/2026-04-18-configurable-rollout-strategies-design.md) / [Plan 1](./superpowers/plans/2026-04-18-rollout-strategies-foundation.md) / [Plan 2](./superpowers/plans/2026-04-18-rollout-engine-deploy.md) / [Plan 3](./superpowers/plans/2026-04-19-rollout-config-integration.md) / [Plan 4](./superpowers/plans/2026-04-19-releases-and-coordination.md) / [Plan 5](./superpowers/plans/2026-04-20-rollout-web-ui.md) | All 5 plans merged: templates, engine+deploy, config rollouts, rollout groups+coordination, web UI. Initiative complete. |
| Feature Lifecycle Layer | Implementation | [Spec](./superpowers/specs/2026-04-19-feature-lifecycle-layer-design.md) / [Plan](./superpowers/plans/2026-04-19-feature-lifecycle-layer.md) / [Guide](./Feature_Lifecycle.md) | Additive layer on top of flags: smoke/user test status, scheduled removal, iteration tracking, 8 new webhook events. Migration 052. Consumed by the CrowdSoft feature-agent + portal. |
| Cloudflare Canary & Observability | Design (deferred) | [Spec](./superpowers/specs/2026-04-22-cloudflare-canary-and-observability-design.md) | PaaS-friendly binary-level canary via edge weighted routing (Cloudflare LB first, pluggable `TrafficRouter`) + provider-agnostic abort signals (push/pull; New Relic, Datadog, Prometheus/Grafana, CloudWatch, generic HTTP). Implementation deferred until customer need arises. |

## Archived

All completed initiatives have been moved to `docs/archives/`. See that directory for historical plans, specs, and their completion records. Most recently archived: **Agentless Deploy Reporting** — all 7 phases shipped 2026-04-23 ([design + completion record](./archives/2026-04-23-agentless-deploy-reporting-design.md)).

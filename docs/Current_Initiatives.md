# Current Initiatives

> Last updated: 2026-04-15

## Active (not yet complete)

| Initiative | Phase | Plan/Spec File | Notes |
|---|---|---|---|
| Canary Rollout E2E | Implementation | On `feature/canary-rollout-e2e` branch (PR #36) | All 3 specs implemented (canary E2E, reliability hardening, all-strategies + live polling). Pending merge. |
| E2E SDK Flag Delivery | Implementation | [Plan](./superpowers/plans/2026-04-12-e2e-sdk-flag-delivery.md) / [Spec](./superpowers/specs/2026-04-12-e2e-sdk-flag-delivery-design.md) | 5 tests passing (Node-only); React probe deferred pending sdk/react ESM fix; fault injection + required-gate pending |
| Project & App Edit/Delete | Design | [Plan](./superpowers/plans/2026-04-15-project-app-edit-delete.md) / [Spec](./superpowers/specs/2026-04-13-project-edit-delete-design.md) | Edit fix + soft/hard delete with activity guard + instant delete when no flags; both projects and applications; API + web UI |
| SDK Application Parameter | Design | [Plan](./superpowers/plans/2026-04-15-sdk-application-parameter.md) / [Spec](./superpowers/specs/2026-04-15-sdk-application-parameter-design.md) | Required `application` slug in both SDKs; backend filters flags by app (union of project-level + app-specific) |

## Archived

All completed initiatives have been moved to `docs/archives/`. See that directory for historical plans, specs, and their completion records.

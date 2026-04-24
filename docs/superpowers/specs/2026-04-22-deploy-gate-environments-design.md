# Feature Request: First-Class Deploy Gates per Environment

**Phase**: Design (feature request — not yet scheduled)
**Filed by**: jobmgr (CrowdSoftApps) — external dogfooding consumer
**Date**: 2026-04-22

## Problem

We want to let operators control "is this environment currently open for deploys?" from the DeploySentry UI, and make DS — not GitHub Actions — the authoritative trigger for gated environment deploys.

Today, the closest primitives are:

1. **`environments.requires_approval` column** (schema only) — exists in `migrations/006_create_environments.up.sql` but no code path enforces it. No API exposure. No UI. No runtime effect.
2. **Rollout strategy `approval` steps** — only gate *phases within a rollout* (e.g., canary at 5% → wait for approve → canary at 25%). They don't gate whether a deployment starts at all.
3. **Feature flags** — can be repurposed as `autodeploy-<env>` kill switches, which is what jobmgr is doing today. CI evaluates the flag at the start of its deploy job and aborts if `false`. Works, but:
   - It conflates "feature flag" (application-facing) with "infrastructure toggle" (ops-facing)
   - There's no CI-triggered "pending approval" state
   - The operator has to know to also manually re-dispatch the GHA workflow after flipping the flag; the flag flip alone doesn't cause a deploy
   - No native audit trail tying a specific deployment to who approved it

## Proposed behavior

### Environment-level gate (existing `requires_approval` column, wired up)

When `environments.requires_approval = true` and CI calls `POST /api/v1/deployments`:

- Deployment is created in new status `pending_approval` (instead of `starting`)
- DS emits `deployment.approval_requested` webhook
- Dashboard shows an **Approve** / **Reject** affordance on the deployment card
- On approve:
  - Deployment moves to `starting` and proceeds through the normal lifecycle
  - Emits `deployment.approval_granted` with `approved_by` + `approved_at`
  - Optionally emits a GitHub `repository_dispatch` event if the env has a GitHub integration configured (see below)
- On reject: deployment moves to terminal `rejected`; emits `deployment.approval_rejected`

### Environment → GitHub trigger integration

New setting per environment: `github_dispatch_on_approval`.

```yaml
github:
  owner: shadsorg
  repo: jobmgr
  dispatch_type: deploysentry-prod-approved   # arrives at GHA as `on.repository_dispatch.types`
  token_secret: DS_GITHUB_PAT                 # DS-stored GitHub token with repo:dispatch scope
```

On `deployment.approval_granted`, DS calls `POST /repos/:owner/:repo/dispatches` with the type above. Payload includes deployment ID, git SHA, environment, approver. The consumer's workflow listens:

```yaml
on:
  repository_dispatch:
    types: [deploysentry-prod-approved]
```

This is the critical piece that makes DS the authoritative trigger rather than a passive observer.

### Schema additions

```sql
ALTER TYPE deployment_status ADD VALUE 'pending_approval';
ALTER TYPE deployment_status ADD VALUE 'rejected';

ALTER TABLE deployments
  ADD COLUMN approval_status TEXT,        -- 'requested' | 'granted' | 'rejected' | NULL
  ADD COLUMN approved_by UUID REFERENCES users(id),
  ADD COLUMN approved_at TIMESTAMPTZ,
  ADD COLUMN rejected_reason TEXT;

ALTER TABLE environments
  ADD COLUMN github_integration JSONB;    -- { owner, repo, dispatch_type, token_secret_ref }
```

`requires_approval` column already exists; just needs behavior attached.

### New API endpoints

```
POST /api/v1/deployments/:id/approve   { reason?: string }
POST /api/v1/deployments/:id/reject    { reason: string }
GET  /api/v1/environments/:id/pending-deployments
```

Existing `POST /api/v1/deployments/:id/advance` is for canary-phase gates; keep it distinct. Approval gates are at the *deployment* level, not the *phase* level.

### New webhook events

- `deployment.approval_requested`
- `deployment.approval_granted` (includes `approved_by`, `approved_at`)
- `deployment.approval_rejected` (includes `rejected_by`, `rejected_reason`)

### Dashboard UI

- On the environment detail page: `Requires approval` toggle (wires `environments.requires_approval`)
- On the deployment list: deployments in `pending_approval` state surface a yellow banner with **Approve** / **Reject** buttons (role-gated to `operator` or `admin`)
- On the environment list: `(2 pending)` badge next to any env with pending deploys
- Audit log entry per approval/rejection

## Why this matters (vs. the flag pattern)

| Need | Flag pattern (today) | Deploy gate (proposed) |
|---|---|---|
| Freeze environment during an incident | Flip flag off | Toggle `requires_approval`, or reject all pending deploys |
| Audit who approved a specific deploy | No — flag flip is just a flag change event | Yes — `approved_by` + `approved_at` on the deployment |
| CI learns of approval event-driven | No — CI has to poll or re-dispatch manually | Yes — `repository_dispatch` or webhook |
| One-click approval in DS UI | No — flip flag + separately re-trigger GHA | Yes — one button does both |
| Reject a specific bad commit | No — flag is env-wide | Yes — reject that deployment, leave env open |
| Integrates cleanly with existing event lifecycle | No — flag events are separate from deploy events | Yes — sits alongside `deployment.completed`, etc. |

## Open questions

1. **Scope.** Does an env with `requires_approval=true` require approval for *every* deploy (including rollbacks)? Recommend: yes for forward deploys, no for DS-initiated auto-rollbacks (rollback is inherently time-sensitive).
2. **Timeout.** Should pending_approval auto-reject after N hours? Recommend: configurable per env, default 24h, default action = reject with reason "approval timeout".
3. **Role required to approve.** Recommend: configurable per env (`required_role`), default `admin`.
4. **Multiple approvers.** v1 = single approver. Later: `approvers_required: N` for prod.
5. **GitHub PAT storage.** Proposed `github_integration.token_secret_ref` points at a secret in DS's existing secrets store (same mechanism used for outbound webhook HMAC keys). Avoid putting raw tokens in JSONB.
6. **Relationship to `rollouts.awaiting_approval`.** Phase-level approval gates already exist in the rollout strategy system. Keep both — they gate different things. Document the distinction clearly.

## Non-goals for v1

- Slack-based approval (Slack button → DS approve). Nice-to-have, later.
- Mobile-app approval flow (DS has a Flutter admin app; add after web UI works).
- Policy-based auto-approval ("approve if tests passed and change < 100 LOC"). Way later.

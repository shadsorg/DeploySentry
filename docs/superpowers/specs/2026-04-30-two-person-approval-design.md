# Two-Person Approval (Org Option) — Future

**Status**: Design (future — not scheduled)
**Date**: 2026-04-30
**Origin**: UI audit §8 (mockup banner: *"Changes to this environment require a 2-person approval"*)

## Goal

Allow an org to require a **second user's approval** before certain destructive or sensitive changes commit to production. Configurable per-org, scoped per-resource-type or per-environment.

## What it gates

Approval-gateable actions (org admin picks which to enable):

- Production-tagged environment writes (any change to a flag's `prod` env state, prod-tagged config setting, prod-only rule)
- Flag archive / hard delete
- API key creation / revocation
- Member role escalation (member → admin, admin → owner)
- Org / project / app deletion

Org configures via a Settings → Compliance tab toggle list. Default off.

## Interaction with staged-changes

This spec **assumes the [staged-changes](./2026-04-30-staged-changes-and-deploy-workflow-design.md) spec ships first**, since that's where the "review and deploy" surface naturally exposes a hand-off point.

When a user clicks Deploy on a staged change that touches an approval-gateable resource, the commit doesn't happen. Instead:

1. The staged_changes rows convert into an `approval_request` row referencing all selected staged changes.
2. The user is shown an "Approval requested" confirmation; the staged rows now display a "Pending approval from {anyone except you}" badge.
3. Notification fires via the existing notification subsystem (Slack / email / PagerDuty channels enabled for the org) listing the requested changes.
4. Any other org member with `approver` permission opens `/orgs/:orgSlug/approvals/:requestId`, sees the same diff view as the review page, and can either Approve (commits the changes) or Reject (discards the request and the staged rows go back to "draft").

The original requester cannot approve their own request.

## Schema sketch

```sql
CREATE TABLE approval_requests (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  requester_id    UUID NOT NULL REFERENCES users(id),
  approver_id     UUID REFERENCES users(id),     -- NULL until decided
  status          TEXT NOT NULL,                  -- 'pending' | 'approved' | 'rejected' | 'expired'
  requested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  decided_at      TIMESTAMPTZ,
  decision_notes  TEXT,
  staged_change_ids UUID[] NOT NULL              -- references staged_changes.id at time of request
);

CREATE TABLE org_approval_policies (
  org_id        UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
  resource_type TEXT NOT NULL,
  action        TEXT,                             -- NULL = any action on this resource_type
  scope         TEXT,                             -- 'env=prod' | NULL (any scope)
  enabled       BOOLEAN NOT NULL DEFAULT true,
  PRIMARY KEY (org_id, resource_type, action, scope)
);
```

Approval requests expire after 7 days (configurable). On expiry, staged_changes are returned to draft on the requester's account.

## Permission model

Two new perms:

- `approval:request` — granted to anyone with mutation perm on a gateable resource. Lets them initiate a request.
- `approval:approve` — separate explicit perm. Org admins by default; can be granted to additional users.

The same user can hold both, but the request handler refuses self-approval.

## Audit

Every transition is audit-logged:

- `approval.requested` — `new_value` is the request payload, `resource_id` is the request id.
- `approval.approved` / `approval.rejected` — written by the approver. The eventual flag/rule/etc. commits write their normal audit rows on top.
- `approval.expired` — written by the cleanup job.

## UI

- New page `/orgs/:orgSlug/approvals` listing pending + recent decisions, filterable by status.
- Inline indicator on resource detail pages ("This resource has 1 pending approval request").
- Dot indicator in the global header ("Approvals (2)") for users with `approval:approve` perm when there are pending requests.

## Out of scope

- More than two parties (3+ approvals, role-of-approver requirements). Single approver per request.
- Approval delegation when the approver is OOO. Manual reassign by org admin only.
- External approver (someone not in the org). Use member invite first.

## Done when

- An org admin can flip a switch in Settings → Compliance to require 2-person approval on prod env changes.
- A user staging a change to a gated resource sees "Will require approval" instead of "Deploy" on the review page.
- Approvers see pending requests in `/approvals`, can review the diff, approve or reject with notes.
- All transitions audit-logged; expired requests release their staged rows back to the requester.

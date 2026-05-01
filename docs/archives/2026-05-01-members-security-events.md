# MembersPage Security Events Panel (UI Audit §13)

**Phase**: Complete
**Date**: 2026-05-01
**Spec parent**: [`./2026-04-30-org-audit-and-revert.md`](./2026-04-30-org-audit-and-revert.md) — Follow-up section
**Branch**: `feature/members-security-events` (squash-merged via PR #82)

## Overview

Add a `<RecentMemberActivity>` panel to `MembersPage` that surfaces recent org-scoped audit entries for member lifecycle events. Requires wiring audit writes for `member.added`, `member.removed`, and `member.role_changed` first — none are written today.

## Scope decision

Spec sketch listed `login`, `logout`, `member.role_changed`, `member.added`, `member.removed`. Login/logout are **not org-scoped** in the current data model — a user authenticates once and has memberships in N orgs. Writing an `auth.logged_in` row to N audit_logs per login is wasteful and noisy. Personal-events streams are out of scope; deferring login/logout to a future plan.

**v1 covers member events only.** Panel title: "Recent Member Activity" (truer than "Security Events" given the trimmed scope).

## Checklist

### Backend
- [x] Thread `AuditWriter` interface into `members.Handler` (mirror `flags.Handler` pattern).
- [x] `addOrgMember` writes `member.added` audit row with `new_value = {user_id, role, email}`.
- [x] `updateOrgMemberRole` reads prior role first, writes `member.role_changed` with `old_value = {role: <prior>}` and `new_value = {role: <new>}`.
- [x] `removeOrgMember` reads member row first (so we have the role for old_value), writes `member.removed` with `old_value = {user_id, role}`.
- [x] `entityType = "user"`, `entityID = userID` for all three.
- [x] `cmd/api/main.go`: wire `auditRepo` into `members.NewHandler`.
- [ ] Handler tests: deferred — `internal/members/` has no test infrastructure today; matching that convention. Audit writes will get coverage when the package picks up its first tests.

### Frontend
- [x] `web/src/components/members/RecentMemberActivity.tsx` — compact list, calls `auditApi.query({ entity_type: 'user', limit: 10 })`. Renders When + actor + action label + affected user. No diff, no revert button.
- [x] Mount on `MembersPage.tsx` above the member table.
- [x] Empty state: "No recent member activity."
- [x] "View full audit log" link to `/orgs/:orgSlug/audit?entity_type=user`.
- [x] Action label helper extends `web/src/components/audit/labels.ts`:
  - `member.added` → "Added member"
  - `member.removed` → "Removed member"
  - `member.role_changed` → "Changed member role"

### Verification
- [x] `go test ./...` clean (verified during PR #82)
- [x] `npm run lint`/`build`/`tsc --noEmit` clean (verified during PR #82)
- [x] Manual: panel renders entries for add/role-change/remove; entries also surface on `/orgs/<slug>/audit`.

## Out of scope
- Login/logout audit writes (need personal-events stream design).
- Project member events (project_members handler is a separate domain).
- Revert handlers for member.* (member ops are revertible in principle but registry wiring is a follow-up).

## Completion Record

- **Branch**: `feature/members-security-events` (a duplicate local branch with the same work was abandoned without PR)
- **Committed**: Yes (`263079d` on main)
- **PR**: [#82](https://github.com/shadsorg/DeploySentry/pull/82) — `feat(members): UI audit §13 — RecentMemberActivity panel + audit writes`
- **Pushed**: Yes — merged to `main` 2026-05-01
- **CI Checks**: Pass

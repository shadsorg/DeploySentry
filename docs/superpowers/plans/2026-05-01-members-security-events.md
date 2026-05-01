# MembersPage Security Events Panel (UI Audit §13)

**Phase**: Implementation
**Date**: 2026-05-01
**Spec parent**: [`./2026-04-30-org-audit-and-revert.md`](./2026-04-30-org-audit-and-revert.md) — Follow-up section
**Branch**: `feature/members-security-events`

## Overview

Add a `<RecentMemberActivity>` panel to `MembersPage` that surfaces recent org-scoped audit entries for member lifecycle events. Requires wiring audit writes for `member.added`, `member.removed`, and `member.role_changed` first — none are written today.

## Scope decision

Spec sketch listed `login`, `logout`, `member.role_changed`, `member.added`, `member.removed`. Login/logout are **not org-scoped** in the current data model — a user authenticates once and has memberships in N orgs. Writing an `auth.logged_in` row to N audit_logs per login is wasteful and noisy. Personal-events streams are out of scope; deferring login/logout to a future plan.

**v1 covers member events only.** Panel title: "Recent Member Activity" (truer than "Security Events" given the trimmed scope).

## Checklist

### Backend
- [ ] Thread `AuditWriter` interface into `members.Handler` (mirror `flags.Handler` pattern).
- [ ] `addOrgMember` writes `member.added` audit row with `new_value = {user_id, role, email}`.
- [ ] `updateOrgMemberRole` reads prior role first, writes `member.role_changed` with `old_value = {role: <prior>}` and `new_value = {role: <new>}`.
- [ ] `removeOrgMember` reads member row first (so we have the role for old_value), writes `member.removed` with `old_value = {user_id, role}`.
- [ ] `entityType = "user"`, `entityID = userID` for all three.
- [ ] `cmd/api/main.go`: wire `auditRepo` into `members.NewHandler`.
- [ ] Handler tests: assert audit writes happen on success paths.

### Frontend
- [ ] `web/src/components/members/RecentMemberActivity.tsx` — compact list, calls `auditApi.query({ entity_type: 'user', limit: 10 })`. Renders When + actor + action label + affected user. No diff, no revert button.
- [ ] Mount on `MembersPage.tsx` above the member table.
- [ ] Empty state: "No recent member activity."
- [ ] "View full audit log" link to `/orgs/:orgSlug/audit?entity_type=user`.
- [ ] Action label helper extends `web/src/components/audit/labels.ts`:
  - `member.added` → "Added member"
  - `member.removed` → "Removed member"
  - `member.role_changed` → "Changed member role"

### Verification
- [ ] `go test ./...` clean
- [ ] `npm run lint`/`build`/`tsc --noEmit` clean
- [ ] Manual: add a member → panel shows "Added member"; change role → "Changed member role"; remove → "Removed member"; entry shows up on `/orgs/<slug>/audit` too.

## Out of scope
- Login/logout audit writes (need personal-events stream design).
- Project member events (project_members handler is a separate domain).
- Revert handlers for member.* (member ops are revertible in principle but registry wiring is a follow-up).

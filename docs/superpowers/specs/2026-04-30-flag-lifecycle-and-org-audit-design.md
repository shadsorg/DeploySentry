# Flag Lifecycle (Archive → Hard Delete) + Org-Level Audit & Undo

**Status**: Design
**Date**: 2026-04-30
**Severity**: medium — UX bug + missing-feature

## Problem

Three connected gaps surfaced while reviewing flag management:

1. **Archive UI is unsafe.** `FlagDetailPage`'s Danger Zone block currently renders **outside the tab switch**, so it appears on Rules, Environments, YAML, Settings, Lifecycle, and History. Archive itself is a single click — no confirmation, no slug-typed verification. One stray click can take a production flag offline.
2. **Hard delete doesn't exist.** `internal/flags/repository.go:DeleteFlag` is comment-as-soft-delete: it sets `archived_at = now()` rather than actually removing the row. There is no path to permanently remove an archived flag, and the `scheduled_removal_at` lifecycle column only emits a webhook when its timer fires — it does not delete the row. So "archive" is terminal in the data model, and rows accumulate forever.
3. **No org-level audit / undo surface.** The `audit_log` table has been recording `(action, old_value, new_value, user_id, created_at)` since migration 008, and there's a `GET /audit-log` endpoint, but **there is no UI** consuming it and **no revert path** — even though the schema captures everything needed to reverse a change.

This spec covers all three: tighten the archive UX, add an explicit hard-delete pathway, and surface the existing audit log as a first-class org page with one-click undo.

## Scope

Three deliverables, separable into their own PRs:

### Deliverable 1 — Frontend: tighten archive UX

In scope:
- Move the Danger Zone block into the **Settings tab only** on `FlagDetailPage`. Hide on every other tab.
- Add a **typed-slug confirmation modal** for Archive. The user must type the flag's `key` (its slug) and tick a checkbox acknowledging "this flag is no longer in use or needed" before the Archive button submits.
- Update the post-archive empty state on the Detail page so the user sees an "Archived on YYYY-MM-DD by <name>" banner with a "Restore" button.

### Deliverable 2 — Backend + UI: hard delete with retention

The product question "when can an archived flag actually be deleted?" needs a concrete answer in the data model. We pick **Option A** (retention-gated hard delete) because it preserves the existing archive workflow, gives operators a clear undo window, and matches industry expectation for soft-delete + later compaction.

Options considered:

- **Option A — Retention-gated hard delete (chosen)**. Archived flags become eligible for permanent deletion **30 days** after `archived_at`. UI surfaces a "Permanently Delete" button on the flag's Settings tab once the retention window has elapsed, gated by typed-slug confirmation. A nightly job (`flag_retention_sweep`) optionally hard-deletes flags whose `archived_at + retention < now()` AND `delete_after IS NOT NULL` (operator opt-in via a flag-level "queue for deletion" action).
- **Option B — Lifecycle-driven**. Wire `scheduled_removal_at` to actually drop the row when the lifecycle scheduler fires. Webhook still emits before row deletion. Rejected because it conflates "remove from product surface" (the lifecycle layer's purpose) with "drop the database row" (a compaction concern).
- **Option C — Archive-only forever**. Make explicit in the UI. Rejected because it doesn't answer the user's question and `audit_log` rows depend on `flag_id` for resource_id linking, which becomes orphaned-but-not-cleanable.

Backend in Option A:

- Migration `060_add_flag_delete_after.up.sql`:
  ```sql
  ALTER TABLE feature_flags
    ADD COLUMN delete_after TIMESTAMPTZ,
    ADD COLUMN deleted_at TIMESTAMPTZ;

  CREATE INDEX idx_feature_flags_delete_after
    ON feature_flags (delete_after)
    WHERE delete_after IS NOT NULL AND deleted_at IS NULL;
  ```
- New endpoints:
  - `POST /flags/:id/queue-deletion` — operator opt-in. Sets `delete_after = archived_at + 30 days`. Requires `flag:archive` perm. Audit-logged.
  - `DELETE /flags/:id?force=true` — immediate hard delete, gated to **archived flags whose retention has elapsed** (`archived_at + 30 days < now()`). Requires `flag:archive` perm + typed-slug confirmation header `X-Confirm-Slug: <flag-key>`. Audit-logged.
  - `POST /flags/:id/restore` — un-archive. Clears `archived_at`, `delete_after`, `deleted_at`. Requires `flag:update`. Audit-logged.
- Repo change: `DeleteFlag` is renamed to `ArchiveFlag` (matches what it actually does). New `HardDeleteFlag(ctx, id)` does `DELETE FROM feature_flags WHERE id = $1 AND archived_at IS NOT NULL AND archived_at + interval '30 days' < now()` — the retention check is enforced in SQL, not just the handler.
- Sweep job (`internal/flags/retention_sweep.go`): periodic ticker (default 6h) that calls `HardDeleteFlag` for every flag whose `delete_after < now() AND deleted_at IS NULL`. Sets `deleted_at` rather than removing the row outright (tombstone), so the `audit_log` foreign keys still work; a separate quarterly compaction job can drop tombstoned rows older than the audit-log retention window.

UI in Option A:

- **Settings tab on `FlagDetailPage`** gains a `Lifecycle` panel (or extends the existing Danger Zone after Deliverable 1):
  - **Active flag** → "Archive Flag" button (typed-slug modal).
  - **Archived flag, within retention** → banner ("Archived on … — eligible for deletion on …") + "Restore" button + "Queue for Deletion" button.
  - **Archived flag, retention elapsed** → "Restore" button + "Permanently Delete" button (typed-slug modal, separate from archive).

### Deliverable 3 — Org-level Audit page with revert

In scope:
- New page `web/src/pages/OrgAuditPage.tsx` mounted at `/orgs/:orgSlug/audit`. Sidebar entry "Audit" (`history` icon) at the org level, visible to org `owner` and `admin` roles.
- Surfaces every row in `audit_log` for the org, newest first, with these columns:
  - **When** — `created_at` relative + ISO tooltip
  - **Who** — actor name + email (resolved via `user_id` LEFT JOIN, already wired by the flag-detail-enhancements PR — see `WriteAuditLog` actor-name plumbing)
  - **What** — human-readable action (e.g. "Archived flag `enable-dark-mode`", "Updated rule on `<key>`", "Created environment `staging`")
  - **Where** — project / app / env breadcrumb when the resource has those (resolved client-side by joining `resource_type` + `resource_id` against the cached entity hierarchy)
  - **Diff** — collapsible side-by-side `old_value` vs `new_value` JSON, pretty-printed with key-level highlighting
  - **Actions** — `Revert` button when the entry is revertible (see below)
- Filter rail (left): action type, resource type, project, user, date range. Each filter URL-serialized so the view is shareable. Mirrors `OrgDeploymentsPage` filter pattern.

Audit log changes already merged earlier (per the flag-detail-enhancements work):
- `audit_handler.go` already filters by org/project/user/resource/date range.
- `WriteAuditLog` already resolves actor names.

What's new in this deliverable:

**Revertibility metadata**. Not every audit entry is automatically revertible. We add a `revertible` boolean to the API response, computed server-side:

| Resource × Action | Revertible? | How |
|---|---|---|
| `flag.archived` | yes | `POST /flags/:id/restore` |
| `flag.hard_deleted` | **no** — tombstone unrecoverable; UI shows "Cannot revert: hard-deleted" |
| `flag.created` | yes | archive the flag |
| `flag.updated` | yes (single-field changes only) | apply `old_value` as new value |
| `flag.toggled` | yes | toggle back |
| `rule.created` | yes | delete the rule |
| `rule.deleted` | yes (within 30 days) | recreate from `old_value` |
| `rule.updated` | yes | apply `old_value` |
| `environment.created` | conditional (only if env has no deploys yet) | delete env |
| `environment.deleted` | no — soft-deleted envs already restorable via Settings | n/a |
| `member.added` / `member.removed` | yes | inverse op |
| `member.role_changed` | yes | apply `old_value.role` |
| `apikey.created` / `apikey.revoked` | no — keys are sensitive, must be created fresh | n/a |
| `deployment.*` | no — deployments are immutable history | n/a |
| Anything else | default no | safer fallback |

**Revert implementation**:
- New endpoint `POST /audit-log/:entryId/revert`. Backend looks up the entry, validates revertibility, and dispatches to a small registry of revert handlers keyed by `(resource_type, action)`. Each handler is a 5–15 line function that calls the existing service-layer method. **No new revert DSL** — we just call the existing API endpoints under the hood.
- Each successful revert writes a new audit_log row with `action = "<original_action>.reverted"`, `resource_id` = original entry id, so the audit trail stays append-only and the revert itself is auditable.
- Reverting an already-reverted entry is allowed (it'll re-do the original action) but the UI tracks chains via `metadata.reverted_by` / `metadata.reverts` to avoid display whiplash.

**UI affordances**:
- Each row's "Revert" button opens a small confirmation popover summarizing the action and showing `old_value` highlighted as the post-revert state.
- Non-revertible rows show a tooltip explaining why ("Hard-deleted flags cannot be restored", etc.).
- A successful revert toasts "Reverted" with an "Undo Revert" link that re-applies the original change (it's just another revert from the perspective of the registry).

## Out of scope

- Diff visualization beyond pretty-printed JSON. A semantic diff (e.g. "rule order changed: A→B→C became B→A→C") is a follow-up if someone files a request for it.
- Bulk revert. Single-row revert only.
- Audit log retention / archival. The `audit_log` table grows linearly today; retention is a separate concern (probably driven by storage cost, not this work).
- Hard-deleting a flag that has open rollouts, releases, or active flag_ratings rows. The `HardDeleteFlag` SQL keeps `ON DELETE CASCADE` semantics — operators get warned in the modal but the cascade is what cleans up. Out of scope: surfacing a pre-flight "this will also remove N rules, M ratings" preview (worth a follow-up).
- A `flag.purge` audit row recorded in some external WORM bucket so the audit history outlives the row tombstone — separate compliance ticket.

## Approach summary

Three PRs, in order:

1. **`feature/flag-archive-ux`** (Deliverable 1) — frontend-only. Ship this first; it fixes the immediate UX bug regardless of what the data model decides.
2. **`feature/flag-hard-delete`** (Deliverable 2) — migration 060, new endpoints, retention sweep, Settings panel update.
3. **`feature/org-audit-revert`** (Deliverable 3) — `OrgAuditPage` + revert endpoint + revertibility registry.

PRs 2 and 3 are independent: revert can ship without hard delete (it just won't have a revert handler for `flag.hard_deleted`, which is correctly non-revertible anyway), and hard delete can ship without revert (it just doesn't appear on a non-existent audit page).

## Done when

- `FlagDetailPage` Danger Zone is settings-tab-only, archives only via typed-slug confirmation, and surfaces a Restore button after archive.
- An archived flag can be permanently deleted via Settings tab once 30 days have elapsed (or queued for deletion explicitly).
- `/orgs/:orgSlug/audit` shows every audit entry for the org with old/new diff and a working Revert button for revertible actions.
- Reverts are themselves audit-logged.

## Risks & callouts

- **Cascade scope**: hard-deleting a flag triggers `ON DELETE CASCADE` on `flag_ratings`, `flag_evaluation_log`, `flag_targeting_rules`, `flag_lifecycle` rows. The migration must verify these FKs are intact before we ship the endpoint; otherwise a hard delete leaves dangling rows.
- **Revert race**: between displaying the audit row and clicking Revert, another change to the same resource may have landed. The revert handler should detect this (e.g. compare current `value` vs `new_value` from the entry) and warn "this revert would overwrite a newer change" before applying.
- **Sensitive data in `old_value`**: API keys, secrets, etc. are already redacted before write per existing audit conventions. Confirm none of the new actions (`flag.hard_deleted`, `flag.queued_for_deletion`) leak fields they shouldn't.
- **Revertibility drift**: the registry must stay in sync with new audit actions added in future PRs. Add a unit test that fails CI when an action with no entry in the revertibility map is observed in production fixtures.

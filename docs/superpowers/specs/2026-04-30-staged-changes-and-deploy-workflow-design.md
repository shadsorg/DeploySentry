# Staged Changes + "Deploy" Review Workflow

**Status**: Design
**Date**: 2026-04-30
**Severity**: high — changes the fundamental commit semantics of the dashboard

## Problem

Every UI mutation today (toggling a flag, editing a rule, changing a setting) writes to the production tables on the click. That's risky: a stray click on the wrong row hits production immediately, and there's no way to batch a coherent set of edits — toggle a flag *and* change its rule *and* update a setting — and review them together before they go live.

The user-described workflow is closer to a database transaction with a review step, except the "transaction" is per-user and persists across page navigations until the user explicitly deploys it or discards it.

## Approach

Introduce a **per-user staging layer**: any mutation on a configurable resource writes to a temporary `staged_changes` table keyed by `(user_id, org_id)` instead of the production table directly. The dashboard reads "production layer + my pending changes overlaid on top" so the user sees their own pending edits applied. A header chrome element ("Deploy N changes") opens a review page where the user picks which staged changes to commit and which to discard.

This is **only the UI commit boundary**. SDKs, the CLI, the API, and webhook integrations still write directly to production — they're not user-facing edit surfaces and they need atomic immediate writes. Staging is a UI-only concern.

## In scope

Resources that get staged:

- Feature flags (create, update, toggle, archive, restore)
- Targeting rules (create, update, delete, reorder)
- Per-environment flag state (enabled/value)
- Per-environment per-rule state (enabled)
- Settings (project, app, env, monitoring links)
- Members (role changes — *not* invites; invites still send email immediately)
- Strategies (create, update steps)
- Strategy defaults / rollout policy

Resources that **bypass staging** (write straight through):

- Deployments (always atomic, always immediate; staging would break the rollout state machine)
- API keys (creation must reveal the plaintext once; can't be staged)
- Org / project / app / environment **creation** (creates affect IDs that other staged rows reference; staging this would require ID-allocation gymnastics — out of scope for v1)
- Webhooks firing
- Audit log writes (the staging layer itself emits its own audit rows; see "Audit interaction")

## Schema sketch

```sql
CREATE TABLE staged_changes (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  resource_type TEXT NOT NULL,
  resource_id   UUID,                 -- NULL when staging a CREATE (provisional id below)
  provisional_id UUID,                -- new UUID assigned for staged creates so other staged rows can reference it
  action        TEXT NOT NULL,        -- 'create' | 'update' | 'delete' | 'archive' | 'restore' | 'toggle' | ...
  field_path    TEXT,                 -- when action='update', dotted path being changed (NULL for whole-row actions)
  old_value     JSONB,                -- snapshot at time of staging
  new_value     JSONB,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- Latest-edit-wins per (user, resource, field): an upsert overwrites the previous staged
  -- change for the same (user_id, org_id, resource_type, resource_id, field_path) tuple
  -- so the user can keep tweaking a value without piling up rows.
  UNIQUE (user_id, org_id, resource_type, resource_id, field_path)
);

CREATE INDEX idx_staged_changes_user_org ON staged_changes (user_id, org_id);
CREATE INDEX idx_staged_changes_resource ON staged_changes (resource_type, resource_id);
```

`provisional_id` lets a staged create be referenced by another staged change in the same batch (e.g., create flag + add rule to it before deploying). On commit, the provisional id is replaced with the real id everywhere it's referenced.

## Read path overlay

Every list / detail read path that the dashboard hits gains an optional `?include_my_staged=true` query param. When present, the handler:

1. Loads the production rows.
2. Loads the user's staged_changes for the same resource type within the same org.
3. Applies the staged changes in memory before returning. Updates patch field-by-field; deletes filter the row out; creates append a synthetic row keyed by `provisional_id`.

The dashboard always sets `include_my_staged=true`; the SDK never does.

## Write path

Mutation endpoints gain a behavior switch:

- **Default behavior (UI-originated request)**: the mutation goes to `staged_changes` as an upsert. The handler returns 200 with the staged row id. The mutation does not touch the production table.
- **Bypass (SDK / CLI / webhook / API key auth)**: the mutation goes straight to production as today. Detected via auth mode (`api_key`, `webhook_signature`) or explicit `?immediate=true` query param.

The dashboard always uses the staging path. The CLI gets a new `--stage` flag for symmetry; default for the CLI stays immediate.

## Header chrome

When the current user has any staged_changes for the active org, every page renders a sticky header banner:

```
[!] You have 5 pending changes  [Review & Deploy →]  [Discard all]
```

`[Review & Deploy →]` opens `/orgs/:orgSlug/deploy-changes`. The banner shows the count, animates briefly on each new staged change.

## Review page

`/orgs/:orgSlug/deploy-changes` shows all the user's staged changes for that org, grouped by resource. Each row:

- Resource name + breadcrumb (project / app / env where applicable)
- Action verb + field path
- Diff view: `old_value` → `new_value` rendered side-by-side (reuse the audit-page diff component from the org-audit deliverable)
- Per-row checkbox (default checked)
- Per-row "Discard" button

Header of the review page:

- Bulk select all / none
- "Deploy Selected" button (primary)
- "Discard Selected" button (secondary danger)
- A summary counter ("3 of 5 selected → Deploy")

On Deploy: the backend commits the selected staged_changes in a single DB transaction:

- For each row, dispatch to the resource's existing service-layer method (same registry pattern as the org-audit revert handlers, just inverted — the registry maps `(resource_type, action) → commit handler`).
- If any handler errors, the transaction rolls back, the staged_changes rows stay in place, and the user sees an error banner indicating which row failed.
- On success, the committed rows are removed from `staged_changes` and audit-log entries are written for each committed change as if the user had made it directly. The audit row's `metadata.staged_at` records when the change was originally staged so the trail is faithful.

Discard is a hard delete from `staged_changes` for that user. No audit log row.

## Audit interaction

Staged changes themselves are *not* audit-logged — they're per-user scratch space. The audit log is written when (and only when) a staged change is committed via Deploy. The audit row records both `created_at` (commit time, same as today) and `staged_at` (when the user first made the change in the UI) so the trail explains "this change was queued at 10:02 and deployed at 10:14 by alice."

The org-audit revert deliverable still applies: a deployed change is revertable through the audit page exactly like a direct change. Reverting a deployed change creates a *new* staged change (an inverse) on the reverter's account, surfaced in their pending banner — they then deploy it like any other change. This keeps reverts in the staging boundary instead of bypassing it.

## Conflict resolution

Two cases:

1. **User A and User B both staged a change to the same field on the same resource**. Both deploys will succeed in commit order; the second commit overwrites the first. The review page warns user B at deploy time when their staged value has been superseded by a more recent commit (compare `old_value` to current production value before commit; if mismatched, the row is flagged "may overwrite a newer change" and stays unchecked by default).
2. **User A staged a change, then changes the field again before deploying**. The upsert in `staged_changes` replaces the prior staged row. The diff shown is `production-current → latest-staged-value` so the user always reviews against current production.

## Cleanup

- Per-user expiration: staged changes older than 30 days auto-discard. Quiet operation, no notification (logged for audit-trail completeness).
- On user removal from an org: their staged changes for that org are discarded (cascade via FK).
- The "Discard all" header action confirms via `ConfirmDialog` (no typed confirmation for discards; only deploys are destructive in the production sense).

## Out of scope

- Multi-user collaboration on the *same* batch (two reviewers approving one user's staged changes). This is one user's working tree; sharing a draft is a future feature.
- Branch-and-merge semantics. Staging is single-track; no fork/merge.
- Pre-deploy simulation ("what would this look like for user X?"). Already partially covered by the "evaluate this flag for these attrs" debugger; not part of this spec.
- Two-person approval. Tracked separately at [`2026-04-30-two-person-approval-design.md`](./2026-04-30-two-person-approval-design.md). When that ships, deploying a staged change that touches a flagged-for-approval resource opens an approval request instead of committing immediately. The two specs interact at exactly that one seam.
- CLI / SDK staging UX. CLI stays immediate by default; `--stage` flag is a future ergonomic.

## Approach summary

Three PRs:

1. **Backend foundation**: `staged_changes` migration, read-path overlay middleware, write-path branching, commit-handler registry. No UI changes — gate via a feature flag (irony noted) so we can test in isolation.
2. **Header chrome + Review page**: surfaces the staging layer in the UI, dispatches Deploy to the commit endpoint.
3. **Per-resource integration**: walk the dashboard pages and switch their mutation calls to the staging path. This is the bulk of the user-facing change and lands page-by-page so it can be reverted granularly if a regression appears.

## Done when

- Toggling a flag, editing a rule, or changing a setting from the dashboard does **not** touch production until Deploy is clicked.
- The header banner shows pending count, links to the review page, and goes away when nothing is staged.
- The review page shows old → new diffs and lets the user select / discard per row.
- Deploy commits the selected rows atomically, with an audit log entry per committed change.
- SDK and CLI behavior is unchanged.

## Risks & callouts

- **Data model footprint**: every read on staged-eligible resources now includes a join + overlay computation. Profile under realistic data volumes; the JSON `new_value` overlay should be cheap if `staged_changes` rows for a single user are bounded (they are — 30-day cleanup + UI usage patterns).
- **SDK divergence**: SDK reads continue going to production; staged changes are invisible to SDK clients until deployed. This is correct (we don't want pending experiments visible to end users) but operators may be confused. Surface this clearly in the review page and the docs.
- **Provisional id leakage**: provisional UUIDs must never escape the staging boundary into webhook payloads, audit log resource_ids, or analytics events. Sanity-check all egress paths before shipping.
- **Permission checks**: a staged change that the user has perm to make at staging time but not at commit time (e.g., role downgrade between staging and Deploy) must re-check the perm at commit. The commit handler registry runs the same RBAC middleware as the direct mutation endpoint.

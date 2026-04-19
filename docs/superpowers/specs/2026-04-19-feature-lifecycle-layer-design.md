# Feature Lifecycle Layer — Design

**Status**: Design
**Date**: 2026-04-19

## Context

CrowdSoft's feature-agent drives end-to-end feature rollouts: generate code, deploy, run smoke tests, prompt users for sign-off, and schedule removal of the guarding flag once the feature has proven itself. DeploySentry already owns the flag, its targeting rules, its webhooks, and the SDK delivery path. This design adds a thin lifecycle layer on top of the existing flag model so the agent (and any other controller) can drive validated rollouts without DeploySentry having to know anything about the agent's internals.

The layer is purely additive. All new fields default to `NULL` / `false` / `0`, and a flag with no lifecycle data behaves exactly as today: evaluated, toggled, and targeted without any new logic firing.

## New flag fields

All optional, default `NULL` (or `0` / `false` for non-nullable scalars):

| Field | Type | Default | Meaning |
|---|---|---|---|
| `smoke_test_status` | `text` (`pending|pass|fail`) | `NULL` | Latest smoke-test outcome reported by the agent |
| `user_test_status` | `text` (`pending|pass|fail`) | `NULL` | Latest user sign-off outcome |
| `scheduled_removal_at` | `timestamptz` | `NULL` | When the flag is queued for removal (cron emits `flag.scheduled_for_removal.due`) |
| `iteration_count` | `int` | `0` | Incremented on every failing smoke/user test — tracks correction cycles |
| `iteration_exhausted` | `bool` | `false` | Agent flips this on after 3 failed iterations |
| `last_smoke_test_notes` | `text` | `NULL` | Free-form notes from the most recent smoke-test result |
| `last_user_test_notes` | `text` | `NULL` | Free-form notes from the most recent user-test result |

## New endpoints

All under `/api/v1/flags/:key/...`, same ApiKey auth as existing SDK-facing endpoints. `:key` resolves against the API key's `project_id` scope.

| Method | Path | Body | Purpose |
|---|---|---|---|
| POST | `/flags/:key/smoke-test-result` | `{ status: "pass"|"fail", notes?, test_run_url? }` | Record smoke-test outcome |
| POST | `/flags/:key/user-test-result` | `{ status: "pass"|"fail", notes?, userId }` — `notes` required on `fail` | Record user sign-off |
| POST | `/flags/:key/schedule-removal` | `{ days: number }` | Set `scheduled_removal_at = now() + days` |
| DELETE | `/flags/:key/schedule-removal` | — | Clear `scheduled_removal_at` |
| POST | `/flags/:key/mark-exhausted` | — | Set `iteration_exhausted = true` after agent gives up |

## State transitions

1. **Either smoke or user test → `fail`**
   - Increment `iteration_count` by 1.
   - Immediately disable the flag across all environments (set `feature_flags.enabled = false` AND disable every `flag_environment_state.enabled`).
   - Store notes on the relevant `last_*_notes` column.
   - Emit `flag.smoke_test.failed` or `flag.user_test.failed`.
2. **Either smoke or user test → `pass`**
   - Persist status + notes. Do not touch `iteration_count`.
   - Emit `flag.smoke_test.passed` or `flag.user_test.passed`.
3. **Both statuses `pass`**
   - Flag is considered validated. Scheduled removal becomes meaningful.
4. **`mark-exhausted`** → set `iteration_exhausted = true`, emit `flag.iteration_exhausted`.
5. **`schedule-removal`** → set timestamp, emit `flag.scheduled_for_removal.set`.
6. **Cron every minute** → for each flag with `scheduled_removal_at <= now()`, emit `flag.scheduled_for_removal.due` once (tracked via `scheduled_removal_fired_at` column, so a follow-up cron tick doesn't re-fire).

## Webhook events

Reuse the existing HMAC-SHA256 signing / delivery pipeline. New event names:

- `flag.smoke_test.passed`
- `flag.smoke_test.failed`
- `flag.user_test.passed`
- `flag.user_test.failed`
- `flag.scheduled_for_removal.set`
- `flag.scheduled_for_removal.cancelled`
- `flag.scheduled_for_removal.due`
- `flag.iteration_exhausted`

All payloads follow the existing `WebhookEventPayload` envelope (`{ event, timestamp, org_id, project_id, data, user_id, metadata }`). The `data` object contains, at minimum:

```json
{
  "flag_id": "uuid",
  "flag_key": "string",
  "flag_name": "string",
  "project_id": "uuid",
  "iteration_count": 2,
  "smoke_test_status": "fail",
  "user_test_status": null,
  "notes": "string?",
  "test_run_url": "string?",
  "scheduled_removal_at": "rfc3339?"
}
```

Payload schema is stable: the CrowdSoft portal reads these same events for its feed, so the agent and the portal share one source of truth. Additions are backwards-compatible; fields are only ever added.

## Dashboard

Flag detail page (`web/src/pages/FlagDetailPage.tsx`) gets a **Lifecycle** tab showing:

- Smoke-test status pill (`pending | pass | fail`) + last notes
- User-test status pill + last notes
- Iteration count and an "exhausted" banner when applicable
- Scheduled-removal countdown (`in 3 days 4 hours`) or "not scheduled"

Timeline view: a chronological feed of lifecycle events for the flag, rendered from the webhook delivery history filtered to that flag's events. The CrowdSoft portal consumes the same feed — keep field names stable.

## Smoke-test targeting convention

Consumer apps set `{ is_smoke_test: true }` on the `EvaluationContext.Attributes` map when running smoke tests. The convention is delivered via a request-scoped `X-DS-Test-Context` header + HMAC-signed `X-DS-Test-Signature`, verified by consumer-side middleware and folded into the DS evaluation context. **No DS work needed for this.** We document the convention in `docs/Rollout_Strategies.md` (targeting-rules section) so users creating rules know the attribute name.

## Backward compatibility

- Every new column is nullable or zero-valued by default.
- Existing flag creation, update, toggle, archive, and evaluation paths are untouched.
- Existing webhook subscribers that only listen to `flag.created` / `flag.toggled` / etc. see no new traffic.
- Existing SDKs keep working: lifecycle fields are not read by the evaluator.

## Out of scope

- The feature-agent service itself (CrowdSoft repo)
- `@crowdsoft/feature-request` consumer SDK
- Header-middleware in consumer apps

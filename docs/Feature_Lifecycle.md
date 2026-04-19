# Feature Lifecycle Layer

**Status**: Live (migration 052)

The feature-lifecycle layer sits on top of the existing DeploySentry flag model. It lets controllers — most prominently the **CrowdSoft feature-agent** — drive a validated rollout end-to-end: deploy, smoke test, gather user sign-off, schedule flag removal, and detect iteration exhaustion after repeated failures.

The layer is **additive only**. Every new field defaults to `NULL` (or `0` / `false`). A flag that was created before migration 052 behaves exactly as it always has; nothing in the evaluator changed.

## Data model

| Field | Type | Default | Meaning |
|---|---|---|---|
| `smoke_test_status` | `pending | pass | fail` | `NULL` | Latest smoke-test outcome |
| `user_test_status`  | `pending | pass | fail` | `NULL` | Latest user sign-off outcome |
| `scheduled_removal_at` | `timestamptz` | `NULL` | When the flag is queued for removal |
| `iteration_count` | `int` | `0` | Bumped on every failing test |
| `iteration_exhausted` | `bool` | `false` | Agent sets this after giving up (typically 3 fails) |
| `last_smoke_test_notes` | `text` | `NULL` | Free-form notes from the last smoke run |
| `last_user_test_notes` | `text` | `NULL` | Free-form notes from the last user-sign-off attempt |

## Endpoints

All under `/api/v1/flags/:id/...`, authenticated with either a Bearer JWT or an `ApiKey` (same chain as the existing flag endpoints). The `:id` segment accepts **either a flag UUID or a flag key**. Keys are resolved against the project scope attached to the API key.

| Method | Path | Body | Effect |
|---|---|---|---|
| POST | `/flags/:id/smoke-test-result` | `{ "status": "pass"|"fail", "notes"?: string, "test_run_url"?: string }` | Records smoke-test outcome. On `fail`: `iteration_count += 1`, flag disabled in every environment, webhook `flag.smoke_test.failed`. On `pass`: webhook `flag.smoke_test.passed`. |
| POST | `/flags/:id/user-test-result` | `{ "status": "pass"|"fail", "notes"?: string, "userId": string }` (notes **required** when `status=fail`) | Records user sign-off. Same side effects as smoke on `fail`. |
| POST | `/flags/:id/schedule-removal` | `{ "days": number }` (must be positive) | Sets `scheduled_removal_at = now() + days`. Webhook `flag.scheduled_for_removal.set`. |
| DELETE | `/flags/:id/schedule-removal` | — | Clears `scheduled_removal_at`. Webhook `flag.scheduled_for_removal.cancelled`. |
| POST | `/flags/:id/mark-exhausted` | — | Sets `iteration_exhausted = true`. Webhook `flag.iteration_exhausted`. |

### State transitions

```
 pass
  ├── smoke_test_status = "pass"         (webhook: flag.smoke_test.passed)
  └── user_test_status  = "pass"         (webhook: flag.user_test.passed)
                            │
                            └── both pass → flag validated; eligible for scheduled removal

 fail
  ├── iteration_count += 1
  ├── enabled = false (flag + every per-env override)
  └── webhook: flag.{smoke_test|user_test}.failed
```

The scheduler is a background goroutine started inside `cmd/api`. It runs every minute, picks every flag whose `scheduled_removal_at <= now()` and whose `scheduled_removal_fired_at IS NULL`, and emits `flag.scheduled_for_removal.due`. The marker column guarantees a single webhook per scheduled removal even if multiple API instances race.

## Webhook events

All lifecycle events reuse the existing HMAC-SHA256 signing and delivery pipeline (see [`docs/Bootstrap_My_App.md`](./Bootstrap_My_App.md) for the signing scheme). New event names:

- `flag.smoke_test.passed`
- `flag.smoke_test.failed`
- `flag.user_test.passed`
- `flag.user_test.failed`
- `flag.scheduled_for_removal.set`
- `flag.scheduled_for_removal.cancelled`
- `flag.scheduled_for_removal.due`
- `flag.iteration_exhausted`

### Payload shape

Every lifecycle event uses the standard `WebhookEventPayload` envelope:

```json
{
  "event": "flag.smoke_test.failed",
  "timestamp": "2026-04-19T14:32:11Z",
  "org_id":   "…",
  "project_id": "…",
  "user_id":  "…?",
  "data": {
    "flag_id":              "…",
    "flag_key":             "checkout_v2",
    "flag_name":            "Checkout redesign",
    "project_id":           "…",
    "iteration_count":      2,
    "iteration_exhausted":  false,
    "smoke_test_status":    "fail",
    "user_test_status":     null,
    "last_smoke_test_notes": "login spec timed out",
    "last_user_test_notes":  null,
    "scheduled_removal_at":  null,
    "notes":                "login spec timed out",
    "test_run_url":         "https://ci…/runs/842"
  }
}
```

**Schema stability**: the CrowdSoft portal and the CrowdSoft feature-agent both consume these events. Fields are never removed and never renamed; only additions are allowed. If you need to ship a breaking change, add a new event name (e.g. `flag.smoke_test.failed.v2`) alongside the old one.

## Smoke-test targeting convention

When a consumer app runs smoke tests, it should tag its evaluation context with:

```json
{ "is_smoke_test": true }
```

i.e. `EvaluationContext.Attributes["is_smoke_test"] = "true"`.

The CrowdSoft consumer SDK (`@crowdsoft/feature-request`) and middleware ship this attribute for you. The delivery channel is:

- Consumer app receives a request with `X-DS-Test-Context` and `X-DS-Test-Signature` (HMAC-SHA256) headers set by the feature-agent.
- Consumer-side middleware verifies the signature, parses the test context, and folds `is_smoke_test: true` into the DS evaluation context for the duration of that request.

**DeploySentry does not verify these headers** — the contract is between the agent and the consumer app. We document the attribute name here so users writing targeting rules know how to route traffic:

```yaml
rules:
  - attribute: is_smoke_test
    operator: equals
    target_values: ["true"]
    value: "true"            # force-on for smoke tests
    priority: 1
```

A rule like the above lets the agent safely force a flag on inside a scoped smoke test even when the flag is otherwise disabled.

## Timeline view

The dashboard's `FlagDetailPage` Lifecycle tab shows the current status pills (smoke, user), the iteration count, an exhausted banner when applicable, and a scheduled-removal countdown. The History tab already surfaces every lifecycle audit event in chronological order — the CrowdSoft portal renders the same information by subscribing to the webhook feed above.

## Backward compatibility

- Every new column is nullable (or zero-valued). No existing row needs a backfill.
- The evaluator does not read any lifecycle field.
- Existing flag CRUD endpoints are unchanged.
- SDKs keep working unmodified.
- Rolling back `052_add_flag_lifecycle.up.sql` drops the columns; the service methods become unreachable because their routes require those columns to exist.

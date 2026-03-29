# Flag Ratings & Error Tracking System

## Overview

A marketplace-style ratings and error tracking system for shared feature flags. In the CrowdSoft ecosystem, subscribers install flag definitions (+ targeting rules) into their own environments from other subscribers. This system enables users to rate flags they've tried (1-5 stars + comments) and tracks per-flag error rates via SDK-reported telemetry. The entire ratings layer is opt-in per org via a setting toggle.

## Context

- Feature flags in DeploySentry are shared config-level entities — the code behind them is already in the shared application; the flag controls activation.
- Subscribers install flag definitions + targeting rules into their environments.
- There is no existing rating, review, or feedback system in the codebase.
- The existing `settings` table (JSONB key-value, scoped to org/project/app/env) will store the opt-in toggle.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Data storage | Dedicated tables (`flag_ratings`, `flag_error_stats`) | Clean relational model, proper constraints, follows existing codebase patterns |
| Rating scope | Global across all subscribers | Marketplace-style aggregate quality signal |
| Rating cardinality | One per user per flag (globally) | Individual org members rate; `UNIQUE(flag_id, user_id)` means one rating regardless of org membership. `org_id` records which org context the rating was submitted from. |
| Error signal source | SDK-reported (wrapper function) | Most accurate per-flag attribution |
| Error detail level | Counts only (evaluations + errors) | Sufficient for error % computation without telemetry overhead |
| Error attribution | Per-org tracking (admin-visible) | Admins can drill down; public view shows aggregate only |
| Ratings vs. error tracking | Independent concerns | Ratings are social (opt-in); error stats are operational (always collected when `track()` is used) |
| Flag-to-org resolution | Join through projects table | `feature_flags.project_id -> projects.org_id`; no direct `org_id` on feature_flags |
| Comment length | Max 2000 characters | Prevents abuse; consistent with typical review systems |

## Data Model

### `flag_ratings` table

```sql
CREATE TABLE flag_ratings (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id    UUID        NOT NULL REFERENCES feature_flags(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id     UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    rating     SMALLINT    NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment    TEXT        CHECK (length(comment) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (flag_id, user_id)
);
```

Indexes:
- `flag_id` — for aggregate queries per flag
- `org_id` — for filtering ratings by org
- `(flag_id, user_id)` — unique constraint serves as index for upsert

### `flag_error_stats` table

```sql
CREATE TABLE flag_error_stats (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id           UUID        NOT NULL REFERENCES feature_flags(id) ON DELETE CASCADE,
    environment_id    UUID        NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    org_id            UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    period_start      TIMESTAMPTZ NOT NULL,
    total_evaluations BIGINT      NOT NULL DEFAULT 0,
    error_count       BIGINT      NOT NULL DEFAULT 0,
    UNIQUE (flag_id, environment_id, org_id, period_start)
);
```

Error percentage computed as: `error_count / total_evaluations * 100`

Hourly bucketing keeps table size manageable while providing enough granularity for trend analysis. The API server truncates the current time to the hour (`date_trunc('hour', now())`) when ingesting error reports — the SDK does not send timestamps. Upsert on the unique constraint increments counters for the current bucket.

### Org setting

Existing `settings` table, key: `flag_ratings_enabled`, scoped to `org_id`.

Value: `{"enabled": true}` or absent (defaults to disabled).

Toggled via existing settings API:
```
PUT /api/v1/settings
{ "org_id": "<uuid>", "key": "flag_ratings_enabled", "value": { "enabled": true } }
```
Requires `admin` or org owner role.

## API Endpoints

All under `/api/v1` with existing auth middleware.

### Ratings

Rating endpoints check `flag_ratings_enabled` org setting — return 403 if disabled.

| Method | Endpoint | Description | Permission |
|--------|----------|-------------|------------|
| `POST` | `/flags/:id/ratings` | Submit or update rating (upsert on user+flag) | `flag:update` (`PermFlagUpdate`) |
| `GET` | `/flags/:id/ratings` | List individual ratings (paginated) | `flag:read` (`PermFlagRead`) |
| `GET` | `/flags/:id/ratings/summary` | Aggregate: avg, count, 1-5 histogram | `flag:read` (`PermFlagRead`) |
| `DELETE` | `/flags/:id/ratings` | Delete calling user's rating (returns 204; returns 204 even if no rating exists — idempotent) | `flag:update` (`PermFlagUpdate`) |

### Error Reporting

Error endpoints are always available (not gated by ratings toggle).

| Method | Endpoint | Description | Permission |
|--------|----------|-------------|------------|
| `POST` | `/flags/errors/report` | SDK batch reports error counts | `flag:update` (`PermFlagUpdate`) |
| `GET` | `/flags/:id/errors/summary` | Error % and trend (24h, 7d, 30d) | `flag:read` (`PermFlagRead`) |
| `GET` | `/flags/:id/errors/by-org` | Per-org error breakdown | `org:manage` (`PermOrgManage`) |

### Batch report payload

```json
{
  "project_id": "<uuid>",
  "environment_id": "<uuid>",
  "org_id": "<uuid>",
  "stats": [
    { "flag_key": "some-flag", "evaluations": 1500, "errors": 3 },
    { "flag_key": "other-flag", "evaluations": 800, "errors": 0 }
  ]
}
```

The handler resolves `flag_key` to `flag_id` using the `(project_id, key)` unique constraint on `feature_flags`. The `project_id` is required to disambiguate keys across projects.

### Augmented flag responses

`GET /flags` and `GET /flags/:id` gain two optional fields when applicable:

- `rating_summary`: `{ "average": 4.2, "count": 37 }` — included only when ratings enabled for the requesting org
- `error_rate`: `{ "percentage": 0.2, "period": "7d" }` — always included when data exists

## SDK Error Reporting Hook

### Interface pattern (all SDKs)

A `track()` method wraps flag-gated code, attributing success/failure to the flag:

**Go:**
```go
result, err := client.Track(ctx, "some-flag", evalCtx, func() (interface{}, error) {
    return doSomethingBehindFlag()
})
```

**Node.js:**
```typescript
const result = await client.track('some-flag', evalCtx, async () => {
    return await doSomethingBehindFlag();
});
```

**Python:**
```python
result = client.track("some-flag", ctx, lambda: do_something_behind_flag())
```

### Internal mechanics

1. SDK maintains in-memory buffer: `map[flagKey] -> { evaluations: int, errors: int }`
2. Each `track()` call increments `evaluations` by 1
3. If the wrapped function returns error / throws exception, increment `errors` by 1
4. Errors propagate normally to the caller — the SDK observes, never swallows
5. Background timer flushes buffer to `POST /flags/errors/report` every 60 seconds
6. Buffer resets on flush; final flush on `client.Close()`
7. If flush fails, stats retained and retried next interval (capped to prevent unbounded memory)

## Visibility Rules

When `flag_ratings_enabled` is **off** (or absent) for an org:

| Layer | Behavior |
|-------|----------|
| API | Rating endpoints return 403. Error reporting and summary endpoints work normally. |
| Dashboard | Rating UI (stars, comments, submit form) not rendered. Error % remains visible. |
| SDK | `track()` works regardless — error reporting is operational, not social. |
| Flag responses | `rating_summary` field omitted. `error_rate` still included. |

## Testing Strategy

### Unit tests

- Rating model validation: range (1-5), unique constraint, comment handling
- Error stats aggregation: correct % calculation, hourly bucketing, period rollups
- Org setting middleware: gates rating endpoints, allows error endpoints through
- SDK buffer logic: flush timing, retry, cap, final flush on close

### Integration tests

- Rating CRUD: create, read, upsert update, delete, uniqueness enforcement
- Error report ingestion: SDK batch payload -> stats table -> summary returns correct %
- Admin error-by-org: admin sees per-org breakdown, non-admin gets 403
- Setting toggle: enable -> endpoints work -> disable -> 403, fields disappear from responses
- Aggregates on flag list: `rating_summary` and `error_rate` on `GET /flags` and `GET /flags/:id`

### Out of scope for this phase

- SDK implementations across all 7 languages (separate per-SDK work)
- Dashboard UI components (frontend phase)

## Future Considerations

- **Data retention:** `flag_error_stats` will grow continuously with hourly bucketing. A future migration should add a retention policy (e.g., roll up to daily after 30 days, purge raw hourly data after 90 days).
- **Query parameter opt-in:** If `rating_summary` / `error_rate` on `GET /flags` causes performance issues, add `?include=ratings,errors` query parameter to make them opt-in. Start with always-included and measure.
- **Rate limiting:** The `POST /flags/errors/report` endpoint may need dedicated rate limiting beyond the global middleware if SDK flush volume is high.

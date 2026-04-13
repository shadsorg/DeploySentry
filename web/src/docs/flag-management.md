# Flag Management

This page covers creating flags, configuring targeting rules, and managing the flag lifecycle.

## Creating Flags

You can create flags through the dashboard, the CLI, or the API.

### Dashboard

1. Navigate to your project's **Flags** page
2. Click **New Flag**
3. Fill in the fields:

| Field | Required | Description |
|---|---|---|
| Key | Yes | Unique identifier. Use lowercase with hyphens: `membership-lookup`, `dark-mode` |
| Name | Yes | Human-readable name |
| Description | No | What this flag controls and why it exists |
| Type | Yes | `boolean`, `string`, `integer`, or `json` |
| Category | Yes | `release`, `feature`, `experiment`, `ops`, or `permission` (see below) |
| Default value | Yes | The value returned when no targeting rules match |
| Owners | No | Teams or individuals responsible for this flag |
| Tags | No | Comma-separated labels for filtering: `checkout, membership` |
| Permanent | No | Check for long-lived flags; uncheck to set an expiration date |
| Expires at | Conditional | Required for non-permanent flags. When this flag should be retired |

### CLI

```bash
# Create a temporary release flag with expiration
deploysentry flags create membership-lookup \
  --type boolean \
  --default false \
  --description "Look up customer membership during cart creation" \
  --tag checkout --tag membership \
  --expires 2026-09-01

# Create a permanent feature flag
deploysentry flags create dark-mode \
  --type boolean \
  --default false \
  --description "Enable dark mode for the dashboard" \
  --permanent

# List flags
deploysentry flags list
deploysentry flags list --tag checkout

# Get flag details
deploysentry flags get membership-lookup

# Toggle a flag on in production
deploysentry flags toggle membership-lookup --on --env production

# Toggle a flag off
deploysentry flags toggle membership-lookup --off --env production
```

### API

```bash
curl -X POST https://api.deploysentry.io/api/v1/flags \
  -H "Authorization: Bearer $DS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "membership-lookup",
    "name": "Membership Lookup",
    "flag_type": "boolean",
    "category": "release",
    "default_value": "false",
    "description": "Look up customer membership during cart creation",
    "is_permanent": false,
    "expires_at": "2026-09-01T00:00:00Z",
    "tags": ["checkout", "membership"],
    "project_id": "YOUR_PROJECT_ID",
    "environment_id": "YOUR_ENVIRONMENT_ID"
  }'
```

## Flag Categories

Choose the category that matches the flag's purpose and expected lifecycle:

| Category | Lifetime | Use when |
|---|---|---|
| **release** | Temporary | Shipping new code dark. Must have an expiration date. Retire after rollout. |
| **feature** | Permanent or temporary | Product toggles. Some tenants want it, others don't. Mark permanent if it stays. |
| **experiment** | Temporary | A/B tests. Set an end date. Retire after analysis. |
| **ops** | Permanent or temporary | Circuit breakers, maintenance mode, rate limiters. Often permanent. |
| **permission** | Typically permanent | Role or entitlement gates. Tied to business rules, not deploy cycles. |

## Permanent vs Temporary Flags

When creating a flag, ask: **will this ever be fully rolled out and deleted?**

### Temporary flags (set an expiration)

- **Release flags** — code ships dark, the flag controls the rollout. Once at 100% and validated, retire the flag and the dead code path.
- **Experiment flags** — A/B tests with a defined analysis window. Retire after results are in.
- **Bug fix flags** — gate a fix for gradual rollout, retire once validated.

The [register/dispatch pattern](/docs/sdks#register--dispatch) makes retirement straightforward:
1. Remove the `register` calls for the flagged and default handlers
2. Replace the `dispatch` call with a direct call to the winning function
3. Delete the losing function
4. Archive the flag: `deploysentry flags archive <key>`

### Permanent flags (mark `is_permanent: true`)

- **Feature flags** — product toggles that some tenants want and others don't. These stay in the codebase indefinitely. The register/dispatch pattern keeps them organized — every handler is still registered in one place.
- **Ops flags** — circuit breakers, maintenance mode switches. Need to stay flippable at runtime forever.
- **Permission flags** — role or entitlement gates. Tied to business rules, not deploy cycles.

Permanent flags stay registered and dispatched indefinitely. Do not retire them.

CLI:
```bash
# Permanent
deploysentry flags create dark-mode --type boolean --default false --permanent

# Temporary with expiration
deploysentry flags create new-checkout --type boolean --default false --expires 2026-09-01
```

## Targeting Rules

Targeting rules control which users see which flag value. Add rules via the dashboard (Flag Detail page, Targeting Rules tab), the CLI, or the API.

### Percentage rollout

Roll out to a percentage of users. The `user_id` in your evaluation context is the bucketing key.

```bash
deploysentry flags update membership-lookup \
  --add-rule '{"rule_type": "percentage", "percentage": 10}'
```

### User targeting

Enable for specific user IDs.

```bash
deploysentry flags update membership-lookup \
  --add-rule '{"rule_type": "user_target", "target_values": ["user-123", "user-456"]}'
```

### Attribute rules

Enable based on a context attribute. Supported operators: `eq`, `neq`, `contains`, `starts_with`, `ends_with`, `in`, `not_in`, `gt`, `gte`, `lt`, `lte`.

```bash
# Enable for users on the "pro" plan
deploysentry flags update membership-lookup \
  --add-rule '{"rule_type": "attribute", "attribute": "plan", "operator": "eq", "value": "pro"}'
```

### Segment rules

Enable for a pre-defined user segment.

```bash
deploysentry flags update membership-lookup \
  --add-rule '{"rule_type": "segment", "segment_id": "SEGMENT_UUID"}'
```

### Schedule rules

Enable between two timestamps.

```bash
deploysentry flags update membership-lookup \
  --add-rule '{"rule_type": "schedule", "start_time": "2026-06-01T00:00:00Z", "end_time": "2026-07-01T00:00:00Z"}'
```

### Compound rules

Combine multiple conditions with AND or OR.

```bash
# Enable for pro users in the US
deploysentry flags update membership-lookup \
  --add-rule '{
    "rule_type": "compound",
    "combine_op": "AND",
    "conditions": [
      {"attribute": "plan", "operator": "eq", "value": "pro"},
      {"attribute": "region", "operator": "eq", "value": "us"}
    ]
  }'
```

## Wiring Flags to Code

After creating a flag, wire it into your code using the [register/dispatch pattern](/docs/sdks#register--dispatch):

1. **Create the flag** in the dashboard or CLI
2. **Register handlers** in your code — the flag key in `register('op', handler, 'flag-key')` must match the key you created
3. **Configure targeting rules** to control who sees what
4. **Dispatch** at call sites — the SDK evaluates the flag and returns the right function

See [SDKs](/docs/sdks) for language-specific examples.

## Flag Lifecycle

### Create

Define the flag with a key, type, category, and default value. Set permanent or add an expiration.

### Target

Add targeting rules to control rollout: percentage, user targeting, attributes, segments, schedules, or compound rules.

### Roll out

Gradually increase the percentage or expand targeting. Monitor metrics and errors at each stage.

### Observe

Use the Analytics page to watch per-flag evaluation counts, error rates, and rollout health.

### Retire (temporary flags only)

When a temporary flag is fully rolled out and stable:
1. Remove the `register` calls for both handlers
2. Replace the `dispatch` call with a direct call to the winning function
3. Delete the losing function
4. Archive: `deploysentry flags archive <key>`

Permanent flags skip this step — they stay in the codebase.

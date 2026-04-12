# e2e-sdk calibration record

Date: 2026-04-12

## Latency measurements (Scenario A, n=10, local Docker on macOS)

| Probe | p50 (ms) | p95 (ms) | max (ms) |
|---|---|---|---|
| Node | 27 | 53 | 53 |

Ceiling set to: **2000ms** (2x p95 = 106ms, floor 1000ms, keeping the spec's existing 2000ms ceiling which provides ample margin).

## Soak results (n=10)

Pass: 10/10. Zero flakes. Average duration: 7.1s. Max: 8.0s.

## Notes

- React probe is deferred due to CJS/ESM bundling issue in sdk/react. Calibration is Node-only.
- Scenario A latency bimodal: ~26ms (60%) and ~52ms (40%). The 52ms cluster may be Redis cache TTL jitter.
- Full suite (5 tests) runs in ~7s consistently. Compose cold start adds ~60-90s on top.

## Fault injection validation

| Fault | Scenario expected to fail | Actual result |
|---|---|---|
| Stop API container mid-test | A | Deferred — requires manual testing per plan |
| API no-op flag update | A | Deferred — requires manual testing per plan |
| Node SDK drops SSE events | A (Node only) | Deferred — requires manual testing per plan |
| Targeting ignores `plan` | B | Deferred — requires manual testing per plan |

Fault injection is documented as a manual validation step in the plan. The four faults above should be tested before marking the `e2e-sdk` workflow as a required check in branch protection.

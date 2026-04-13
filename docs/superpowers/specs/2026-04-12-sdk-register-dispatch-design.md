# SDK Register & Dispatch Pattern — Design Spec

**Date:** 2026-04-12
**Status:** Approved (brainstorming complete, ready for implementation plan)
**Related plans:** TBD (writing-plans next)

## Overview

Add a `register` / `dispatch` pattern to all 7 DeploySentry SDKs. Instead of scattering `if/else` flag checks throughout application code, developers register handler functions against a logical **operation name** and associate each handler with a flag. At call sites, `dispatch(operation, context)` evaluates the relevant flags, selects the correct handler, and returns it for the caller to invoke.

This centralizes every flag's code paths in one place, makes cleanup trivial (delete the registration + the dead function), and gives LLM agents a machine-readable map of flag → code.

## Goals

- Every flag-gated code path is discoverable from a single registration site per operation.
- Developers replace `if (flag) { newFn() } else { oldFn() }` with `client.dispatch('op', ctx)(args)`.
- An LLM (or human) retiring a flag can find every affected registration, remove it, and delete the unused function — no archaeology.
- All 7 SDKs ship the feature together for a consistent cross-language story.

## Non-goals

- No API or backend changes. The registry is purely client-side.
- No dashboard UI changes. Flag creation and targeting rules work as-is.
- No changes to existing `boolValue`/`stringValue`/etc. evaluation methods — they remain available and untouched.
- No lint rule implementation in this spec (called out as a follow-up).
- No sample apps in this spec (called out as a future plan item).

## Decisions locked in brainstorming

| # | Decision | Choice |
|---|---|---|
| 1 | Dispatch return behavior | **Pattern B** — dispatch returns the selected function; the caller invokes it |
| 2 | Handler signature constraints | **None (C)** — SDK doesn't constrain handler arity or return type; it's a lookup table |
| 3 | Registration key | **Operation name**, not flag name. Multiple flags can gate different handlers under the same operation |
| 4 | Priority / conflict resolution | **First match wins** (registration order). Default handler registered last |
| 5 | Unregistered dispatch behavior | **Throw/panic** — always a programmer error |
| 6 | Architecture | **Approach 1** — register/dispatch are methods on the existing client class, registry is a private map |
| 7 | SDK rollout scope | **All 7 SDKs** in one pass |

## Core data model

### Registration

Each client instance holds a private map: `operationName → Registration[]`.

A `Registration` is:

```
{
  handler: Function    // the callable the user registered
  flagKey: string?     // null = default (no flag gating)
}
```

- Registrations are stored in **insertion order**.
- Flag-gated handlers should be registered first (highest priority first).
- The **default** (no flag) should be registered **last** — it's the catch-all.
- Registering a second default for the same operation **replaces** the previous default (last-default-wins; there can only be one fallback).
- Multiple flag-gated registrations per operation are expected and supported.

### Dispatch evaluation flow

1. Look up `operation` in the registry map. **Not found → throw/panic.**
2. Walk the registration list in order.
3. For each entry with a `flagKey`, evaluate `boolValue(flagKey, false, context)` using the local flag cache (synchronous — see "Sync cache read" below).
4. **First `true`** → return that handler.
5. Reach the end of the list → return the default (the entry with `flagKey == null`).
6. **No default exists and nothing matched → throw/panic.**

### Context responsibility

The `context` parameter on `dispatch` is the same `EvaluationContext` used by `boolValue` across all SDKs. It flows to flag evaluation, **not** to the handler.

The developer is responsible for populating the context with the attributes their targeting rules in the DeploySentry dashboard evaluate against — user ID, session ID, request headers, percentage-rollout keys, etc. **The context you pass to `dispatch` must include the attributes your targeting rules reference**, or the rules won't match.

The handler receives its arguments from the caller at invocation time — the SDK doesn't touch them.

## Public API — per-SDK signatures

### Node.js / TypeScript

```ts
register<T extends (...args: any[]) => any>(operation: string, handler: T, flagKey?: string): void
dispatch<T extends (...args: any[]) => any>(operation: string, context?: EvaluationContext): T
```

### React (TypeScript)

Same client methods as Node, plus a hook:

```ts
// Client methods (same as Node)
register<T extends (...args: any[]) => any>(operation: string, handler: T, flagKey?: string): void
dispatch<T extends (...args: any[]) => any>(operation: string, context?: EvaluationContext): T

// Hook — reads client from provider context, uses provider's user context
function useDispatch<T extends (...args: any[]) => any>(operation: string): T
```

### Go

```go
func (c *Client) Register(operation string, handler any, flagKey ...string)
func (c *Client) Dispatch(operation string, ctx ...EvaluationContext) any
```

`Dispatch` returns `any`; the caller type-asserts: `c.Dispatch("createCart").(func(Cart, User) Result)(cart, user)`.

Additionally, a package-level generic helper for cleaner call sites:

```go
func Dispatch[T any](c *Client, operation string, ctx ...EvaluationContext) T
```

### Python

```python
def register(self, operation: str, handler: Callable, flag_key: str | None = None) -> None
def dispatch(self, operation: str, context: EvaluationContext | None = None) -> Callable
```

### Java

```java
public <T> void register(String operation, Supplier<T> handler, String flagKey)
public <T> void register(String operation, Supplier<T> handler)  // default overload (no flag)
public <T> Supplier<T> dispatch(String operation, EvaluationContext context)
```

### Ruby

```ruby
def register(operation, handler, flag_key: nil)
def dispatch(operation, context: nil)  # returns a Proc/lambda
```

### Flutter / Dart

```dart
void register<T extends Function>(String operation, T handler, {String? flagKey})
T dispatch<T extends Function>(String operation, {EvaluationContext? context})
```

## Usage pattern (all languages)

```
// At app startup — register handlers for an operation
client.register('createCart', createCartWithMembership, 'membership-lookup')
client.register('createCart', createCartWithLoyalty, 'loyalty-points')
client.register('createCart', createCart)  // default — always last

// At call site — dispatch picks the right handler, caller invokes it
result = client.dispatch('createCart', ctx)(cartItems, user)
```

## Internal implementation details

### Sync cache read for dispatch

Dispatch must be **synchronous** to cleanly return a function reference. All SDKs maintain a local flag cache populated after `initialize()` and kept current via SSE.

For SDKs where `boolValue` is currently async (Node, Python, Flutter): dispatch uses an **internal synchronous cache lookup** instead of the public `boolValue`. This means extracting a private `_boolValueSync(key, defaultValue, context)` method that reads from the already-populated cache without an API fallback.

If the cache is empty (client not initialized), dispatch throws — same as calling any evaluation method before `initialize()`.

### Thread safety

| SDK | Strategy |
|---|---|
| Go | `sync.RWMutex` on the registry map |
| Java | `ConcurrentHashMap<String, List<Registration>>` + `synchronized` on list append |
| Node / React | Single-threaded, no locking needed |
| Python | `threading.Lock` around registry writes |
| Ruby | `Mutex` around registry writes |
| Flutter / Dart | Single-isolate, no locking needed |

### Error messages

Specific, actionable messages:

- **No registration:** `"No handlers registered for operation 'createCart'. Call register() before dispatch()."`
- **No match + no default:** `"No matching handler for operation 'createCart' and no default registered. Register a default handler (no flagKey) as the last registration."`

## Testing strategy

**8 unit tests per SDK, 56 total.** Same scenarios, language-idiomatic:

1. **Register + dispatch happy path** — register a default and a flagged handler, flag is off → returns default. Flag is on → returns flagged handler.
2. **First-match priority** — two flagged handlers, both flags on → returns the first-registered one.
3. **Default-only** — register only a default, dispatch returns it regardless of context.
4. **Multiple operations** — register handlers for `'createCart'` and `'processPayment'`, verify they don't interfere.
5. **Throw on unregistered operation** — dispatch an operation name that was never registered → throws.
6. **Throw on no default + no match** — register only flagged handlers, none match → throws.
7. **Replace default** — register two defaults for the same operation, second replaces first.
8. **Handler receives caller args** — register a handler that takes `(a, b) → a + b`, dispatch and invoke with args, verify result.

## Documentation updates

- **In-app docs** (`web/src/docs/sdks.md`): Add a "Register & Dispatch" section after the current evaluation examples. Show the pattern for Node (primary example) with a note that all SDKs support it. Emphasize: context must include the attributes your targeting rules reference.
- **Each SDK's section in `sdks.md`**: Add a dispatch example alongside the existing `isEnabled`/`boolValue` example.
- **Landing page**: The pillars section and code contrast band already describe the pattern — no changes needed.

## Follow-up items (not in scope for this implementation)

### Lint rule

A static analysis rule that verifies every `dispatch('X')` call has a corresponding `register('X')` in the codebase. Available as:
- ESLint plugin for Node/React/TS
- `go vet` analyzer for Go
- Language-appropriate tooling for Python (pylint/ruff), Ruby (rubocop), Java (ErrorProne), Dart (custom lint)

This catches misspelled operation names and missing registrations at build time rather than runtime.

### Sample applications

One sample app per SDK language demonstrating the recommended register/dispatch pattern as a best practice reference. These serve dual purpose:
- **Documentation** — working code developers can clone and reference.
- **Test automation** — each sample app can be used as an integration test target for end-to-end SDK validation.

Languages: Go, Node.js, Python, Java, Ruby, React (web app), Flutter (mobile app). Each app should demonstrate a realistic scenario (e.g., a simplified POS or e-commerce checkout flow) with multiple operations, flag-gated handlers, and a default fallback per operation.

## File and line references

- Node client: `sdk/node/src/client.ts` — `DeploySentryClient` class
- Node types: `sdk/node/src/types.ts` — `EvaluationContext`, `ClientOptions`
- Go client: `sdk/go/client.go` — `Client` struct
- Python client: `sdk/python/deploysentry/client.py` — `DeploySentryClient` class
- Java client: `sdk/java/src/main/java/io/deploysentry/DeploySentryClient.java`
- React client: `sdk/react/src/client.ts` — `DeploySentryClient` class
- React provider: `sdk/react/src/provider.tsx` — `DeploySentryProvider`
- Ruby client: `sdk/ruby/lib/deploysentry/client.rb` — `DeploySentry::Client`
- Flutter client: `sdk/flutter/lib/src/client.dart` — `DeploySentryClient` class
- In-app docs: `web/src/docs/sdks.md` — SDK documentation stubs

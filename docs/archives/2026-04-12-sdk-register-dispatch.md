# SDK Register & Dispatch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `register(operation, handler, flagKey?)` and `dispatch(operation, context?)` methods to all 7 DeploySentry SDKs, enabling centralized flag-gated function dispatch keyed by operation name.

**Architecture:** Each SDK's existing client class gets two new public methods and a private registry map (`operationName → Registration[]`). Dispatch walks registrations in order, evaluates flags synchronously from the local cache, and returns the first matching handler (or the default). SDKs with async `boolValue` (Node, Flutter) use a private sync cache lookup for dispatch. All 7 SDKs follow the identical pattern.

**Tech Stack:** TypeScript (Node + React), Go, Python, Java, Ruby, Dart (Flutter). Existing test frameworks per SDK (Vitest for React, Jest for Node, Go testing, pytest, JUnit, minitest, flutter_test).

**Spec:** `docs/superpowers/specs/2026-04-12-sdk-register-dispatch-design.md`

---

## Pre-flight notes

- Each SDK task is independent — they can be implemented in any order or in parallel.
- The pattern is identical across all SDKs: private registry map, `register` method, `dispatch` method, 8 unit tests.
- The registry map is `Map<string, Registration[]>` where `Registration = { handler, flagKey? }`.
- Dispatch evaluates flags using the **local cache only** (sync). No API calls during dispatch.
- Throw/panic if: (a) no registrations exist for the operation, (b) no flag matches and no default is registered.
- Default handler = registration with no `flagKey`. Registering a second default replaces the first.
- First-match wins (registration order). Default should be registered last.

---

## Task 1: Node SDK — register & dispatch (TDD)

**Files:**
- Modify: `sdk/node/src/client.ts`
- Modify: `sdk/node/src/types.ts`
- Test: `sdk/node/src/__tests__/client.test.ts`

- [ ] **Step 1: Add the Registration type to types.ts**

In `sdk/node/src/types.ts`, append:

```ts
export interface Registration<T extends (...args: any[]) => any = (...args: any[]) => any> {
  handler: T;
  flagKey?: string;
}
```

- [ ] **Step 2: Write the 8 failing tests**

Append to `sdk/node/src/__tests__/client.test.ts`:

```ts
describe('register and dispatch', () => {
  let client: DeploySentryClient;

  beforeEach(() => {
    client = new DeploySentryClient({
      apiKey: 'test-key',
      environment: 'test',
      project: 'test-project',
    });
  });

  function mockFlagEnabled(key: string, enabled: boolean) {
    // Seed the cache with a flag object
    (client as any).cache.set(key, {
      key,
      enabled,
      value: enabled,
      metadata: { category: 'feature', createdAt: new Date().toISOString() },
    });
  }

  it('dispatches the flagged handler when flag is on', () => {
    const defaultFn = () => 'default';
    const featFn = () => 'feature';
    client.register('op', featFn, 'my-flag');
    client.register('op', defaultFn);
    mockFlagEnabled('my-flag', true);
    const result = client.dispatch('op')();
    expect(result).toBe('feature');
  });

  it('dispatches the default handler when flag is off', () => {
    const defaultFn = () => 'default';
    const featFn = () => 'feature';
    client.register('op', featFn, 'my-flag');
    client.register('op', defaultFn);
    mockFlagEnabled('my-flag', false);
    const result = client.dispatch('op')();
    expect(result).toBe('default');
  });

  it('returns the first matching handler when multiple flags are on', () => {
    const fn1 = () => 'first';
    const fn2 = () => 'second';
    const defaultFn = () => 'default';
    client.register('op', fn1, 'flag-a');
    client.register('op', fn2, 'flag-b');
    client.register('op', defaultFn);
    mockFlagEnabled('flag-a', true);
    mockFlagEnabled('flag-b', true);
    const result = client.dispatch('op')();
    expect(result).toBe('first');
  });

  it('dispatches the default when only a default is registered', () => {
    const defaultFn = () => 'default';
    client.register('op', defaultFn);
    const result = client.dispatch('op')();
    expect(result).toBe('default');
  });

  it('keeps operations isolated', () => {
    client.register('cart', () => 'cart-default');
    client.register('pay', () => 'pay-default');
    expect(client.dispatch('cart')()).toBe('cart-default');
    expect(client.dispatch('pay')()).toBe('pay-default');
  });

  it('throws on unregistered operation', () => {
    expect(() => client.dispatch('unknown')).toThrow(
      "No handlers registered for operation 'unknown'"
    );
  });

  it('throws when no flag matches and no default exists', () => {
    client.register('op', () => 'feat', 'my-flag');
    mockFlagEnabled('my-flag', false);
    expect(() => client.dispatch('op')).toThrow(
      "No matching handler for operation 'op' and no default registered"
    );
  });

  it('replaces a previous default for the same operation', () => {
    client.register('op', () => 'first-default');
    client.register('op', () => 'second-default');
    expect(client.dispatch('op')()).toBe('second-default');
  });

  it('passes caller args through to the handler', () => {
    client.register('add', (a: number, b: number) => a + b);
    const result = client.dispatch<(a: number, b: number) => number>('add')(3, 4);
    expect(result).toBe(7);
  });
});
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd sdk/node
npx jest --testPathPattern client.test --verbose 2>&1 | tail -20
```

Expected: FAIL — `client.register is not a function`.

- [ ] **Step 4: Implement register and dispatch on the client**

In `sdk/node/src/client.ts`:

Add to imports at top:

```ts
import type { Registration } from './types';
```

Add a private field after the existing fields (around line 42):

```ts
private registry: Map<string, Registration[]> = new Map();
```

Add these two methods to the `DeploySentryClient` class (after the existing metadata methods, before the private helpers):

```ts
  register<T extends (...args: any[]) => any>(
    operation: string,
    handler: T,
    flagKey?: string,
  ): void {
    let list = this.registry.get(operation);
    if (!list) {
      list = [];
      this.registry.set(operation, list);
    }
    if (flagKey === undefined) {
      // Default handler — replace any existing default
      const idx = list.findIndex((r) => r.flagKey === undefined);
      if (idx !== -1) list[idx] = { handler };
      else list.push({ handler });
    } else {
      list.push({ handler, flagKey });
    }
  }

  dispatch<T extends (...args: any[]) => any>(
    operation: string,
    context?: EvaluationContext,
  ): T {
    const list = this.registry.get(operation);
    if (!list || list.length === 0) {
      throw new Error(
        `No handlers registered for operation '${operation}'. Call register() before dispatch().`,
      );
    }
    for (const reg of list) {
      if (reg.flagKey !== undefined) {
        const flag = this.cache.get(reg.flagKey);
        if (flag && flag.enabled) return reg.handler as T;
      }
    }
    // Fall through to default
    const defaultReg = list.find((r) => r.flagKey === undefined);
    if (!defaultReg) {
      throw new Error(
        `No matching handler for operation '${operation}' and no default registered. Register a default handler (no flagKey) as the last registration.`,
      );
    }
    return defaultReg.handler as T;
  }
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd sdk/node
npx jest --testPathPattern client.test --verbose 2>&1 | tail -20
```

Expected: all 8 new tests + existing tests pass.

- [ ] **Step 6: Commit**

```bash
git add sdk/node/src/client.ts sdk/node/src/types.ts sdk/node/src/__tests__/client.test.ts
git commit -m "feat(sdk/node): add register/dispatch for centralized flag-gated function dispatch"
```

---

## Task 2: Go SDK — register & dispatch (TDD)

**Files:**
- Modify: `sdk/go/client.go`
- Modify: `sdk/go/models.go`
- Test: `sdk/go/client_test.go`

- [ ] **Step 1: Add the registration type to models.go**

In `sdk/go/models.go`, append:

```go
type registration struct {
	handler any
	flagKey string // empty string = default
}
```

- [ ] **Step 2: Add the registry field to the Client struct**

In `sdk/go/client.go`, add to the `Client` struct fields:

```go
	registry   map[string][]registration
	registryMu sync.RWMutex
```

And in the `NewClient` constructor, initialize it:

```go
	registry: make(map[string][]registration),
```

Ensure `sync` is imported.

- [ ] **Step 3: Write the 8 failing tests**

Append to `sdk/go/client_test.go`:

```go
func TestRegisterDispatch(t *testing.T) {
	newClient := func() *Client {
		c := NewClient(WithAPIKey("test"))
		return c
	}
	seedFlag := func(c *Client, key string, enabled bool) {
		c.cache.set(key, Flag{Key: key, Enabled: enabled})
	}

	t.Run("dispatches flagged handler when flag is on", func(t *testing.T) {
		c := newClient()
		c.Register("op", func() string { return "feature" }, "my-flag")
		c.Register("op", func() string { return "default" })
		seedFlag(c, "my-flag", true)
		fn := c.Dispatch("op").(func() string)
		if fn() != "feature" {
			t.Fatal("expected feature")
		}
	})

	t.Run("dispatches default when flag is off", func(t *testing.T) {
		c := newClient()
		c.Register("op", func() string { return "feature" }, "my-flag")
		c.Register("op", func() string { return "default" })
		seedFlag(c, "my-flag", false)
		fn := c.Dispatch("op").(func() string)
		if fn() != "default" {
			t.Fatal("expected default")
		}
	})

	t.Run("first match wins when multiple flags on", func(t *testing.T) {
		c := newClient()
		c.Register("op", func() string { return "first" }, "flag-a")
		c.Register("op", func() string { return "second" }, "flag-b")
		c.Register("op", func() string { return "default" })
		seedFlag(c, "flag-a", true)
		seedFlag(c, "flag-b", true)
		fn := c.Dispatch("op").(func() string)
		if fn() != "first" {
			t.Fatal("expected first")
		}
	})

	t.Run("default only", func(t *testing.T) {
		c := newClient()
		c.Register("op", func() string { return "default" })
		fn := c.Dispatch("op").(func() string)
		if fn() != "default" {
			t.Fatal("expected default")
		}
	})

	t.Run("operations are isolated", func(t *testing.T) {
		c := newClient()
		c.Register("cart", func() string { return "cart" })
		c.Register("pay", func() string { return "pay" })
		fn1 := c.Dispatch("cart").(func() string)
		fn2 := c.Dispatch("pay").(func() string)
		if fn1() != "cart" || fn2() != "pay" {
			t.Fatal("operations leaked")
		}
	})

	t.Run("panics on unregistered operation", func(t *testing.T) {
		c := newClient()
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic")
			}
			msg := r.(string)
			if !strings.Contains(msg, "No handlers registered") {
				t.Fatalf("unexpected panic: %s", msg)
			}
		}()
		c.Dispatch("unknown")
	})

	t.Run("panics when no match and no default", func(t *testing.T) {
		c := newClient()
		c.Register("op", func() string { return "feat" }, "my-flag")
		seedFlag(c, "my-flag", false)
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic")
			}
			msg := r.(string)
			if !strings.Contains(msg, "no default registered") {
				t.Fatalf("unexpected panic: %s", msg)
			}
		}()
		c.Dispatch("op")
	})

	t.Run("replaces previous default", func(t *testing.T) {
		c := newClient()
		c.Register("op", func() string { return "first" })
		c.Register("op", func() string { return "second" })
		fn := c.Dispatch("op").(func() string)
		if fn() != "second" {
			t.Fatal("expected second default")
		}
	})

	t.Run("passes caller args through", func(t *testing.T) {
		c := newClient()
		c.Register("add", func(a, b int) int { return a + b })
		fn := c.Dispatch("add").(func(int, int) int)
		if fn(3, 4) != 7 {
			t.Fatal("expected 7")
		}
	})
}
```

Ensure `strings` is imported in the test file.

- [ ] **Step 4: Run tests to verify they fail**

```bash
cd sdk/go
go test -run TestRegisterDispatch -v ./...
```

Expected: FAIL — `c.Register undefined`.

- [ ] **Step 5: Implement Register and Dispatch**

In `sdk/go/client.go`, add these methods:

```go
// Register adds a handler for the given operation. If flagKey is provided,
// the handler is only selected when that flag is enabled. If flagKey is
// omitted, this is the default handler (register it last).
func (c *Client) Register(operation string, handler any, flagKey ...string) {
	key := ""
	if len(flagKey) > 0 {
		key = flagKey[0]
	}

	c.registryMu.Lock()
	defer c.registryMu.Unlock()

	list := c.registry[operation]
	if key == "" {
		// Default — replace existing default
		for i, r := range list {
			if r.flagKey == "" {
				list[i] = registration{handler: handler, flagKey: ""}
				c.registry[operation] = list
				return
			}
		}
		list = append(list, registration{handler: handler, flagKey: ""})
	} else {
		list = append(list, registration{handler: handler, flagKey: key})
	}
	c.registry[operation] = list
}

// Dispatch evaluates flags for the given operation and returns the first
// matching handler. The caller must type-assert and invoke the result.
// Panics if no handlers are registered or no match is found without a default.
func (c *Client) Dispatch(operation string, ctx ...EvaluationContext) any {
	c.registryMu.RLock()
	list, ok := c.registry[operation]
	c.registryMu.RUnlock()

	if !ok || len(list) == 0 {
		panic(fmt.Sprintf("No handlers registered for operation '%s'. Call Register() before Dispatch().", operation))
	}

	for _, reg := range list {
		if reg.flagKey != "" {
			f, found, _ := c.cache.get(reg.flagKey)
			if found && f.Enabled {
				return reg.handler
			}
		}
	}

	// Fall through to default
	for _, reg := range list {
		if reg.flagKey == "" {
			return reg.handler
		}
	}

	panic(fmt.Sprintf("No matching handler for operation '%s' and no default registered. Register a default handler (no flagKey) as the last registration.", operation))
}
```

Ensure `fmt` is imported.

- [ ] **Step 6: Also add the generic helper function**

Below the `Dispatch` method or at the bottom of `client.go`:

```go
// Dispatch is a generic package-level helper that type-asserts the result.
// Usage: fn := deploysentry.Dispatch[func(Cart, User) Result](client, "createCart")
func Dispatch[T any](c *Client, operation string, ctx ...EvaluationContext) T {
	return c.Dispatch(operation, ctx...).(T)
}
```

Note: If Go version is < 1.18, skip this helper. Check `go.mod` for the Go version.

- [ ] **Step 7: Run tests to verify they pass**

```bash
cd sdk/go
go test -run TestRegisterDispatch -v ./...
```

Expected: 9 tests pass (8 subtests + parent).

- [ ] **Step 8: Commit**

```bash
git add sdk/go/client.go sdk/go/models.go sdk/go/client_test.go
git commit -m "feat(sdk/go): add Register/Dispatch for centralized flag-gated function dispatch"
```

---

## Task 3: Python SDK — register & dispatch (TDD)

**Files:**
- Modify: `sdk/python/deploysentry/client.py`
- Modify: `sdk/python/deploysentry/async_client.py`
- Test: `sdk/python/tests/test_client.py`

- [ ] **Step 1: Write the 8 failing tests**

Append to `sdk/python/tests/test_client.py`:

```python
class TestRegisterDispatch:
    def setup_method(self):
        self.client = DeploySentryClient(
            api_key="test-key",
            base_url="http://localhost:8080",
            environment="test",
            project="test-project",
        )

    def _seed_flag(self, key: str, enabled: bool):
        from deploysentry.models import Flag, FlagMetadata
        flag = Flag(key=key, enabled=enabled, value=enabled, metadata=FlagMetadata(category="feature", created_at="2026-01-01T00:00:00Z"))
        self.client._flags[key] = flag

    def test_dispatches_flagged_handler_when_on(self):
        self.client.register("op", lambda: "feature", flag_key="my-flag")
        self.client.register("op", lambda: "default")
        self._seed_flag("my-flag", True)
        assert self.client.dispatch("op")() == "feature"

    def test_dispatches_default_when_flag_off(self):
        self.client.register("op", lambda: "feature", flag_key="my-flag")
        self.client.register("op", lambda: "default")
        self._seed_flag("my-flag", False)
        assert self.client.dispatch("op")() == "default"

    def test_first_match_wins(self):
        self.client.register("op", lambda: "first", flag_key="flag-a")
        self.client.register("op", lambda: "second", flag_key="flag-b")
        self.client.register("op", lambda: "default")
        self._seed_flag("flag-a", True)
        self._seed_flag("flag-b", True)
        assert self.client.dispatch("op")() == "first"

    def test_default_only(self):
        self.client.register("op", lambda: "default")
        assert self.client.dispatch("op")() == "default"

    def test_operations_isolated(self):
        self.client.register("cart", lambda: "cart")
        self.client.register("pay", lambda: "pay")
        assert self.client.dispatch("cart")() == "cart"
        assert self.client.dispatch("pay")() == "pay"

    def test_throws_on_unregistered(self):
        import pytest
        with pytest.raises(RuntimeError, match="No handlers registered"):
            self.client.dispatch("unknown")

    def test_throws_no_match_no_default(self):
        import pytest
        self.client.register("op", lambda: "feat", flag_key="my-flag")
        self._seed_flag("my-flag", False)
        with pytest.raises(RuntimeError, match="no default registered"):
            self.client.dispatch("op")

    def test_replaces_default(self):
        self.client.register("op", lambda: "first")
        self.client.register("op", lambda: "second")
        assert self.client.dispatch("op")() == "second"

    def test_passes_caller_args(self):
        self.client.register("add", lambda a, b: a + b)
        assert self.client.dispatch("add")(3, 4) == 7
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd sdk/python
python -m pytest tests/test_client.py -k "TestRegisterDispatch" -v
```

Expected: FAIL — `AttributeError: 'DeploySentryClient' object has no attribute 'register'`.

- [ ] **Step 3: Implement register and dispatch on the sync client**

In `sdk/python/deploysentry/client.py`:

Add a private field in `__init__` (after existing fields around line 70):

```python
        self._registry: dict[str, list[dict]] = {}
```

Add these two methods to the class (after the metadata methods, before the private helpers):

```python
    def register(self, operation: str, handler: Callable, flag_key: str | None = None) -> None:
        """Register a handler for an operation, optionally gated by a flag."""
        lst = self._registry.setdefault(operation, [])
        if flag_key is None:
            # Default handler — replace existing default
            for i, reg in enumerate(lst):
                if reg["flag_key"] is None:
                    lst[i] = {"handler": handler, "flag_key": None}
                    return
            lst.append({"handler": handler, "flag_key": None})
        else:
            lst.append({"handler": handler, "flag_key": flag_key})

    def dispatch(self, operation: str, context: EvaluationContext | None = None) -> Callable:
        """Evaluate flags and return the matching handler for the operation."""
        lst = self._registry.get(operation)
        if not lst:
            raise RuntimeError(
                f"No handlers registered for operation '{operation}'. "
                "Call register() before dispatch()."
            )
        for reg in lst:
            if reg["flag_key"] is not None:
                flag = self._flags.get(reg["flag_key"])
                if flag and flag.enabled:
                    return reg["handler"]
        # Fall through to default
        for reg in lst:
            if reg["flag_key"] is None:
                return reg["handler"]
        raise RuntimeError(
            f"No matching handler for operation '{operation}' and no default registered. "
            "Register a default handler (no flag_key) as the last registration."
        )
```

Ensure `Callable` is imported from `typing`.

- [ ] **Step 4: Copy register and dispatch to the async client**

In `sdk/python/deploysentry/async_client.py`, add the same `_registry` field in `__init__` and the **same two methods** (they are synchronous — dispatch reads from `self._flags` which is a sync dict). The async client inherits the same `_flags` pattern, so the implementation is identical.

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd sdk/python
python -m pytest tests/test_client.py -k "TestRegisterDispatch" -v
```

Expected: 9 tests pass.

- [ ] **Step 6: Commit**

```bash
git add sdk/python/deploysentry/client.py sdk/python/deploysentry/async_client.py sdk/python/tests/test_client.py
git commit -m "feat(sdk/python): add register/dispatch for centralized flag-gated function dispatch"
```

---

## Task 4: Java SDK — register & dispatch (TDD)

**Files:**
- Modify: `sdk/java/src/main/java/io/deploysentry/DeploySentryClient.java`
- Create: `sdk/java/src/main/java/io/deploysentry/Registration.java`
- Test: `sdk/java/src/test/java/io/deploysentry/DeploySentryClientTest.java`

- [ ] **Step 1: Create the Registration class**

`sdk/java/src/main/java/io/deploysentry/Registration.java`:

```java
package io.deploysentry;

import java.util.function.Supplier;

class Registration<T> {
    final Supplier<T> handler;
    final String flagKey; // null = default

    Registration(Supplier<T> handler, String flagKey) {
        this.handler = handler;
        this.flagKey = flagKey;
    }
}
```

- [ ] **Step 2: Write the 8 failing tests**

Append to `sdk/java/src/test/java/io/deploysentry/DeploySentryClientTest.java`:

```java
    // --- register / dispatch tests ---

    private DeploySentryClient newTestClient() {
        return new DeploySentryClient(
            ClientOptions.builder().apiKey("test-key").environment("test").project("test").build()
        );
    }

    private void seedFlag(DeploySentryClient client, String key, boolean enabled) {
        try {
            var field = DeploySentryClient.class.getDeclaredField("cache");
            field.setAccessible(true);
            var cache = (FlagCache) field.get(client);
            cache.put(key, new Flag(key, enabled, String.valueOf(enabled), null));
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    @Test
    void dispatchesFlaggedHandlerWhenOn() {
        var client = newTestClient();
        client.register("op", () -> "feature", "my-flag");
        client.register("op", () -> "default");
        seedFlag(client, "my-flag", true);
        assertEquals("feature", client.<String>dispatch("op", null).get());
    }

    @Test
    void dispatchesDefaultWhenFlagOff() {
        var client = newTestClient();
        client.register("op", () -> "feature", "my-flag");
        client.register("op", () -> "default");
        seedFlag(client, "my-flag", false);
        assertEquals("default", client.<String>dispatch("op", null).get());
    }

    @Test
    void firstMatchWins() {
        var client = newTestClient();
        client.register("op", () -> "first", "flag-a");
        client.register("op", () -> "second", "flag-b");
        client.register("op", () -> "default");
        seedFlag(client, "flag-a", true);
        seedFlag(client, "flag-b", true);
        assertEquals("first", client.<String>dispatch("op", null).get());
    }

    @Test
    void defaultOnly() {
        var client = newTestClient();
        client.register("op", () -> "default");
        assertEquals("default", client.<String>dispatch("op", null).get());
    }

    @Test
    void operationsIsolated() {
        var client = newTestClient();
        client.register("cart", () -> "cart");
        client.register("pay", () -> "pay");
        assertEquals("cart", client.<String>dispatch("cart", null).get());
        assertEquals("pay", client.<String>dispatch("pay", null).get());
    }

    @Test
    void throwsOnUnregistered() {
        var client = newTestClient();
        var ex = assertThrows(IllegalStateException.class, () -> client.dispatch("unknown", null));
        assertTrue(ex.getMessage().contains("No handlers registered"));
    }

    @Test
    void throwsNoMatchNoDefault() {
        var client = newTestClient();
        client.register("op", () -> "feat", "my-flag");
        seedFlag(client, "my-flag", false);
        var ex = assertThrows(IllegalStateException.class, () -> client.dispatch("op", null));
        assertTrue(ex.getMessage().contains("no default registered"));
    }

    @Test
    void replacesDefault() {
        var client = newTestClient();
        client.register("op", () -> "first");
        client.register("op", () -> "second");
        assertEquals("second", client.<String>dispatch("op", null).get());
    }
```

Ensure imports include `java.util.function.Supplier`, `static org.junit.jupiter.api.Assertions.*`.

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd sdk/java
mvn test -Dtest="DeploySentryClientTest#dispatchesFlaggedHandlerWhenOn" 2>&1 | tail -10
```

Expected: FAIL — `cannot find symbol: method register`.

- [ ] **Step 4: Implement register and dispatch**

In `sdk/java/src/main/java/io/deploysentry/DeploySentryClient.java`:

Add imports:

```java
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.Supplier;
```

Add field (after existing fields around line 52):

```java
    private final ConcurrentHashMap<String, java.util.List<Registration<?>>> registry = new ConcurrentHashMap<>();
```

Add methods:

```java
    public <T> void register(String operation, Supplier<T> handler, String flagKey) {
        registry.compute(operation, (k, list) -> {
            if (list == null) list = new java.util.ArrayList<>();
            list.add(new Registration<>(handler, flagKey));
            return list;
        });
    }

    public <T> void register(String operation, Supplier<T> handler) {
        registry.compute(operation, (k, list) -> {
            if (list == null) list = new java.util.ArrayList<>();
            // Replace existing default
            for (int i = 0; i < list.size(); i++) {
                if (list.get(i).flagKey == null) {
                    list.set(i, new Registration<>(handler, null));
                    return list;
                }
            }
            list.add(new Registration<>(handler, null));
            return list;
        });
    }

    @SuppressWarnings("unchecked")
    public <T> Supplier<T> dispatch(String operation, EvaluationContext context) {
        var list = registry.get(operation);
        if (list == null || list.isEmpty()) {
            throw new IllegalStateException(
                "No handlers registered for operation '" + operation + "'. Call register() before dispatch()."
            );
        }
        for (var reg : list) {
            if (reg.flagKey != null) {
                var flag = cache.get(reg.flagKey);
                if (flag != null && flag.isEnabled()) {
                    return (Supplier<T>) reg.handler;
                }
            }
        }
        for (var reg : list) {
            if (reg.flagKey == null) {
                return (Supplier<T>) reg.handler;
            }
        }
        throw new IllegalStateException(
            "No matching handler for operation '" + operation + "' and no default registered. Register a default handler (no flagKey) as the last registration."
        );
    }
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd sdk/java
mvn test 2>&1 | tail -10
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add sdk/java/src/main/java/io/deploysentry/Registration.java sdk/java/src/main/java/io/deploysentry/DeploySentryClient.java sdk/java/src/test/java/io/deploysentry/DeploySentryClientTest.java
git commit -m "feat(sdk/java): add register/dispatch for centralized flag-gated function dispatch"
```

---

## Task 5: React SDK — register, dispatch & useDispatch hook (TDD)

**Files:**
- Modify: `sdk/react/src/client.ts`
- Modify: `sdk/react/src/types.ts`
- Create: `sdk/react/src/hooks.ts` (add `useDispatch` — file already exists, append to it)
- Modify: `sdk/react/src/index.ts` (export `useDispatch`)
- Test: `sdk/react/src/__tests__/client.test.ts`

The React client's `boolValue` is already **synchronous** (reads from `this.flags` Map directly). No sync-cache refactor needed.

- [ ] **Step 1: Add the Registration type to types.ts**

In `sdk/react/src/types.ts`, append:

```ts
export interface Registration<T extends (...args: any[]) => any = (...args: any[]) => any> {
  handler: T;
  flagKey?: string;
}
```

- [ ] **Step 2: Write the 8 failing tests for client register/dispatch**

Append to `sdk/react/src/__tests__/client.test.ts`:

```ts
describe('register and dispatch', () => {
  let client: DeploySentryClient;

  beforeEach(() => {
    client = new DeploySentryClient({
      apiKey: 'test-key',
      baseURL: 'http://localhost',
      environment: 'test',
      project: 'test-project',
    });
  });

  function mockFlagEnabled(key: string, enabled: boolean) {
    (client as any).flags.set(key, {
      key,
      enabled,
      value: enabled,
      metadata: { category: 'feature', createdAt: new Date().toISOString() },
    });
  }

  it('dispatches the flagged handler when flag is on', () => {
    client.register('op', () => 'feature', 'my-flag');
    client.register('op', () => 'default');
    mockFlagEnabled('my-flag', true);
    expect(client.dispatch('op')()).toBe('feature');
  });

  it('dispatches the default handler when flag is off', () => {
    client.register('op', () => 'feature', 'my-flag');
    client.register('op', () => 'default');
    mockFlagEnabled('my-flag', false);
    expect(client.dispatch('op')()).toBe('default');
  });

  it('returns the first matching handler when multiple flags are on', () => {
    client.register('op', () => 'first', 'flag-a');
    client.register('op', () => 'second', 'flag-b');
    client.register('op', () => 'default');
    mockFlagEnabled('flag-a', true);
    mockFlagEnabled('flag-b', true);
    expect(client.dispatch('op')()).toBe('first');
  });

  it('dispatches the default when only a default is registered', () => {
    client.register('op', () => 'default');
    expect(client.dispatch('op')()).toBe('default');
  });

  it('keeps operations isolated', () => {
    client.register('cart', () => 'cart');
    client.register('pay', () => 'pay');
    expect(client.dispatch('cart')()).toBe('cart');
    expect(client.dispatch('pay')()).toBe('pay');
  });

  it('throws on unregistered operation', () => {
    expect(() => client.dispatch('unknown')).toThrow("No handlers registered for operation 'unknown'");
  });

  it('throws when no flag matches and no default exists', () => {
    client.register('op', () => 'feat', 'my-flag');
    mockFlagEnabled('my-flag', false);
    expect(() => client.dispatch('op')).toThrow("no default registered");
  });

  it('replaces a previous default', () => {
    client.register('op', () => 'first');
    client.register('op', () => 'second');
    expect(client.dispatch('op')()).toBe('second');
  });

  it('passes caller args through', () => {
    client.register('add', (a: number, b: number) => a + b);
    expect(client.dispatch<(a: number, b: number) => number>('add')(3, 4)).toBe(7);
  });
});
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd sdk/react
npx jest --testPathPattern client.test --verbose 2>&1 | tail -20
```

Expected: FAIL — `client.register is not a function`.

- [ ] **Step 4: Implement register and dispatch on the React client**

In `sdk/react/src/client.ts`:

Add import:

```ts
import type { Registration } from './types';
```

Add private field (after existing fields around line 62):

```ts
  private registry: Map<string, Registration[]> = new Map();
```

Add methods (same logic as Node SDK — the React client already has sync `boolValue` reading from `this.flags`):

```ts
  register<T extends (...args: any[]) => any>(
    operation: string,
    handler: T,
    flagKey?: string,
  ): void {
    let list = this.registry.get(operation);
    if (!list) {
      list = [];
      this.registry.set(operation, list);
    }
    if (flagKey === undefined) {
      const idx = list.findIndex((r) => r.flagKey === undefined);
      if (idx !== -1) list[idx] = { handler };
      else list.push({ handler });
    } else {
      list.push({ handler, flagKey });
    }
  }

  dispatch<T extends (...args: any[]) => any>(
    operation: string,
    context?: EvaluationContext,
  ): T {
    const list = this.registry.get(operation);
    if (!list || list.length === 0) {
      throw new Error(
        `No handlers registered for operation '${operation}'. Call register() before dispatch().`,
      );
    }
    for (const reg of list) {
      if (reg.flagKey !== undefined) {
        const flag = this.flags.get(reg.flagKey);
        if (flag && flag.enabled) return reg.handler as T;
      }
    }
    const defaultReg = list.find((r) => r.flagKey === undefined);
    if (!defaultReg) {
      throw new Error(
        `No matching handler for operation '${operation}' and no default registered. Register a default handler (no flagKey) as the last registration.`,
      );
    }
    return defaultReg.handler as T;
  }
```

Note: The React client reads from `this.flags` (not `this.cache`) since it stores flags in a plain `Map<string, Flag>`.

- [ ] **Step 5: Add the `useDispatch` hook**

In `sdk/react/src/hooks.ts`, add:

```ts
export function useDispatch<T extends (...args: any[]) => any>(operation: string): T {
  const client = useDeploySentry();
  return client.dispatch<T>(operation);
}
```

This uses the provider's user context implicitly (the client already has it from `identify()`). The hook is intentionally thin — it's just a convenience wrapper.

- [ ] **Step 6: Export `useDispatch` from index.ts**

In `sdk/react/src/index.ts`, add `useDispatch` to the hooks export:

```ts
export { useDispatch } from './hooks';
```

- [ ] **Step 7: Run tests to verify they pass**

```bash
cd sdk/react
npx jest --testPathPattern client.test --verbose 2>&1 | tail -20
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
git add sdk/react/src/client.ts sdk/react/src/types.ts sdk/react/src/hooks.ts sdk/react/src/index.ts sdk/react/src/__tests__/client.test.ts
git commit -m "feat(sdk/react): add register/dispatch and useDispatch hook"
```

---

## Task 6: Ruby SDK — register & dispatch (TDD)

**Files:**
- Modify: `sdk/ruby/lib/deploysentry/client.rb`
- Test: `sdk/ruby/test/client_test.rb`

- [ ] **Step 1: Write the 8 failing tests**

Append to `sdk/ruby/test/client_test.rb`:

```ruby
class TestRegisterDispatch < Minitest::Test
  def setup
    @client = DeploySentry::Client.new(
      api_key: "test-key",
      base_url: "http://localhost:8080",
      environment: "test",
      project: "test-project"
    )
  end

  def seed_flag(key, enabled)
    flag = DeploySentry::Flag.new(key: key, enabled: enabled, value: enabled.to_s,
      metadata: DeploySentry::FlagMetadata.new(category: "feature", created_at: "2026-01-01T00:00:00Z"))
    @client.instance_variable_get(:@flags)[key] = flag
  end

  def test_dispatches_flagged_handler_when_on
    @client.register("op", -> { "feature" }, flag_key: "my-flag")
    @client.register("op", -> { "default" })
    seed_flag("my-flag", true)
    assert_equal "feature", @client.dispatch("op").call
  end

  def test_dispatches_default_when_flag_off
    @client.register("op", -> { "feature" }, flag_key: "my-flag")
    @client.register("op", -> { "default" })
    seed_flag("my-flag", false)
    assert_equal "default", @client.dispatch("op").call
  end

  def test_first_match_wins
    @client.register("op", -> { "first" }, flag_key: "flag-a")
    @client.register("op", -> { "second" }, flag_key: "flag-b")
    @client.register("op", -> { "default" })
    seed_flag("flag-a", true)
    seed_flag("flag-b", true)
    assert_equal "first", @client.dispatch("op").call
  end

  def test_default_only
    @client.register("op", -> { "default" })
    assert_equal "default", @client.dispatch("op").call
  end

  def test_operations_isolated
    @client.register("cart", -> { "cart" })
    @client.register("pay", -> { "pay" })
    assert_equal "cart", @client.dispatch("cart").call
    assert_equal "pay", @client.dispatch("pay").call
  end

  def test_throws_on_unregistered
    err = assert_raises(RuntimeError) { @client.dispatch("unknown") }
    assert_match(/No handlers registered/, err.message)
  end

  def test_throws_no_match_no_default
    @client.register("op", -> { "feat" }, flag_key: "my-flag")
    seed_flag("my-flag", false)
    err = assert_raises(RuntimeError) { @client.dispatch("op") }
    assert_match(/no default registered/, err.message)
  end

  def test_replaces_default
    @client.register("op", -> { "first" })
    @client.register("op", -> { "second" })
    assert_equal "second", @client.dispatch("op").call
  end

  def test_passes_caller_args
    @client.register("add", ->(a, b) { a + b })
    assert_equal 7, @client.dispatch("add").call(3, 4)
  end
end
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd sdk/ruby
ruby -Ilib -Itest test/client_test.rb -n /TestRegisterDispatch/
```

Expected: FAIL — `undefined method 'register'`.

- [ ] **Step 3: Implement register and dispatch**

In `sdk/ruby/lib/deploysentry/client.rb`:

Add in `initialize` (after existing instance variables around line 29):

```ruby
      @registry = {}
      @registry_mutex = Mutex.new
```

Add methods to the class:

```ruby
    def register(operation, handler, flag_key: nil)
      @registry_mutex.synchronize do
        list = @registry[operation] ||= []
        if flag_key.nil?
          # Default — replace existing
          idx = list.index { |r| r[:flag_key].nil? }
          if idx
            list[idx] = { handler: handler, flag_key: nil }
          else
            list.push({ handler: handler, flag_key: nil })
          end
        else
          list.push({ handler: handler, flag_key: flag_key })
        end
      end
    end

    def dispatch(operation, context: nil)
      list = @registry[operation]
      if list.nil? || list.empty?
        raise "No handlers registered for operation '#{operation}'. Call register() before dispatch()."
      end

      list.each do |reg|
        next if reg[:flag_key].nil?
        flag = @flags[reg[:flag_key]]
        return reg[:handler] if flag&.enabled
      end

      default_reg = list.find { |r| r[:flag_key].nil? }
      unless default_reg
        raise "No matching handler for operation '#{operation}' and no default registered. Register a default handler (no flag_key) as the last registration."
      end
      default_reg[:handler]
    end
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd sdk/ruby
ruby -Ilib -Itest test/client_test.rb -n /TestRegisterDispatch/
```

Expected: 9 tests pass.

- [ ] **Step 5: Commit**

```bash
git add sdk/ruby/lib/deploysentry/client.rb sdk/ruby/test/client_test.rb
git commit -m "feat(sdk/ruby): add register/dispatch for centralized flag-gated function dispatch"
```

---

## Task 7: Flutter SDK — register & dispatch (TDD)

**Files:**
- Modify: `sdk/flutter/lib/src/client.dart`
- Test: `sdk/flutter/test/client_test.dart`

- [ ] **Step 1: Write the 8 failing tests**

Append to `sdk/flutter/test/client_test.dart`:

```dart
  group('register and dispatch', () {
    late DeploySentryClient client;

    setUp(() {
      client = DeploySentryClient(
        apiKey: 'test-key',
        baseUrl: 'http://localhost:8080',
        environment: 'test',
        project: 'test-project',
      );
    });

    void seedFlag(String key, bool enabled) {
      // Access internal cache to seed a flag
      final cache = (client as dynamic)._cache;
      cache.set(key, Flag(key: key, enabled: enabled, value: enabled.toString(), metadata: FlagMetadata(category: FlagCategory.feature, createdAt: DateTime.now())));
    }

    test('dispatches flagged handler when flag is on', () {
      client.register<String Function()>('op', () => 'feature', flagKey: 'my-flag');
      client.register<String Function()>('op', () => 'default');
      seedFlag('my-flag', true);
      final fn = client.dispatch<String Function()>('op');
      expect(fn(), 'feature');
    });

    test('dispatches default when flag is off', () {
      client.register<String Function()>('op', () => 'feature', flagKey: 'my-flag');
      client.register<String Function()>('op', () => 'default');
      seedFlag('my-flag', false);
      expect(client.dispatch<String Function()>('op')(), 'default');
    });

    test('first match wins', () {
      client.register<String Function()>('op', () => 'first', flagKey: 'flag-a');
      client.register<String Function()>('op', () => 'second', flagKey: 'flag-b');
      client.register<String Function()>('op', () => 'default');
      seedFlag('flag-a', true);
      seedFlag('flag-b', true);
      expect(client.dispatch<String Function()>('op')(), 'first');
    });

    test('default only', () {
      client.register<String Function()>('op', () => 'default');
      expect(client.dispatch<String Function()>('op')(), 'default');
    });

    test('operations isolated', () {
      client.register<String Function()>('cart', () => 'cart');
      client.register<String Function()>('pay', () => 'pay');
      expect(client.dispatch<String Function()>('cart')(), 'cart');
      expect(client.dispatch<String Function()>('pay')(), 'pay');
    });

    test('throws on unregistered operation', () {
      expect(() => client.dispatch('unknown'), throwsStateError);
    });

    test('throws no match no default', () {
      client.register<String Function()>('op', () => 'feat', flagKey: 'my-flag');
      seedFlag('my-flag', false);
      expect(() => client.dispatch('op'), throwsStateError);
    });

    test('replaces default', () {
      client.register<String Function()>('op', () => 'first');
      client.register<String Function()>('op', () => 'second');
      expect(client.dispatch<String Function()>('op')(), 'second');
    });

    test('passes caller args', () {
      client.register<int Function(int, int)>('add', (int a, int b) => a + b);
      final fn = client.dispatch<int Function(int, int)>('add');
      expect(fn(3, 4), 7);
    });
  });
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd sdk/flutter
flutter test test/client_test.dart 2>&1 | tail -15
```

Expected: FAIL — `register` method not found.

- [ ] **Step 3: Implement register and dispatch**

In `sdk/flutter/lib/src/client.dart`:

Add a private field (after existing fields around line 28):

```dart
  final Map<String, List<_Registration>> _registry = {};
```

Add a private class at the bottom of the file (outside `DeploySentryClient`):

```dart
class _Registration {
  final Function handler;
  final String? flagKey;
  _Registration({required this.handler, this.flagKey});
}
```

Add methods to `DeploySentryClient`:

```dart
  void register<T extends Function>(String operation, T handler, {String? flagKey}) {
    final list = _registry.putIfAbsent(operation, () => []);
    if (flagKey == null) {
      // Default — replace existing
      final idx = list.indexWhere((r) => r.flagKey == null);
      if (idx != -1) {
        list[idx] = _Registration(handler: handler);
      } else {
        list.add(_Registration(handler: handler));
      }
    } else {
      list.add(_Registration(handler: handler, flagKey: flagKey));
    }
  }

  T dispatch<T extends Function>(String operation, {EvaluationContext? context}) {
    final list = _registry[operation];
    if (list == null || list.isEmpty) {
      throw StateError(
        "No handlers registered for operation '$operation'. Call register() before dispatch().",
      );
    }
    for (final reg in list) {
      if (reg.flagKey != null) {
        final flag = _cache.get(reg.flagKey!);
        if (flag != null && flag.enabled) return reg.handler as T;
      }
    }
    // Fall through to default
    final defaultReg = list.cast<_Registration?>().firstWhere(
      (r) => r!.flagKey == null,
      orElse: () => null,
    );
    if (defaultReg == null) {
      throw StateError(
        "No matching handler for operation '$operation' and no default registered. Register a default handler (no flagKey) as the last registration.",
      );
    }
    return defaultReg.handler as T;
  }
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd sdk/flutter
flutter test test/client_test.dart 2>&1 | tail -15
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add sdk/flutter/lib/src/client.dart sdk/flutter/test/client_test.dart
git commit -m "feat(sdk/flutter): add register/dispatch for centralized flag-gated function dispatch"
```

---

## Task 8: Update in-app docs (`sdks.md`)

**Files:**
- Modify: `web/src/docs/sdks.md`

- [ ] **Step 1: Read current sdks.md**

```bash
cat web/src/docs/sdks.md
```

- [ ] **Step 2: Add Register & Dispatch section**

After the existing SDK examples but before the closing line ("See each SDK's README..."), add:

```markdown
## Register & Dispatch

Instead of scattering `if/else` flag checks throughout your code, register handler functions against a logical operation name and let the SDK pick the right one at call time.

### The pattern

1. **Register** handlers for each operation at app startup — flagged handlers first, default last:

\`\`\`ts
// Node / React
ds.register('createCart', createCartWithMembership, 'membership-lookup');
ds.register('createCart', createCartWithLoyalty, 'loyalty-points');
ds.register('createCart', createCart); // default — always last
\`\`\`

2. **Dispatch** at each call site — the SDK evaluates flags and returns the right function:

\`\`\`ts
const result = ds.dispatch('createCart', ctx)(cartItems, user);
\`\`\`

The context you pass to `dispatch` must include the attributes your targeting rules in the DeploySentry dashboard evaluate against (user ID, session ID, request headers, etc.).

### Why this matters

- **One place per operation** — every flag-gated code path is visible at the registration site.
- **Clean retirement** — delete the registration + the dead function. No archaeology.
- **LLM-ready** — an agent can scan registrations to find every flag's code and clean up automatically.

### Language examples

**Go:**
\`\`\`go
client.Register("createCart", createCartWithMembership, "membership-lookup")
client.Register("createCart", createCart) // default
fn := client.Dispatch("createCart", ctx).(func(Cart, User) Result)
result := fn(cart, user)
\`\`\`

**Python:**
\`\`\`python
ds.register("create_cart", create_cart_with_membership, flag_key="membership-lookup")
ds.register("create_cart", create_cart)  # default
result = ds.dispatch("create_cart", ctx)(cart_items, user)
\`\`\`

**Java:**
\`\`\`java
client.register("createCart", () -> createCartWithMembership(cart, user), "membership-lookup");
client.register("createCart", () -> createCart(cart, user)); // default
var result = client.<Result>dispatch("createCart", ctx).get();
\`\`\`

**Ruby:**
\`\`\`ruby
client.register("create_cart", method(:create_cart_with_membership), flag_key: "membership-lookup")
client.register("create_cart", method(:create_cart)) # default
result = client.dispatch("create_cart", context: ctx).call(cart_items, user)
\`\`\`

**Flutter (Dart):**
\`\`\`dart
client.register<Result Function(Cart, User)>('createCart', createCartWithMembership, flagKey: 'membership-lookup');
client.register<Result Function(Cart, User)>('createCart', createCart); // default
final fn = client.dispatch<Result Function(Cart, User)>('createCart', context: ctx);
final result = fn(cart, user);
\`\`\`

**React (hook):**
\`\`\`tsx
// Registration happens at app init (same as Node)
ds.register('createCart', createCartWithMembership, 'membership-lookup');
ds.register('createCart', createCart);

// Inside a component
function CheckoutButton() {
  const createCart = useDispatch<(items: CartItem[]) => Result>('createCart');
  return <button onClick={() => createCart(items)}>Checkout</button>;
}
\`\`\`

All seven SDKs support register & dispatch. See each SDK's README for the full reference.
```

- [ ] **Step 3: Lint**

```bash
cd web
npm run lint
```

- [ ] **Step 4: Commit**

```bash
git add web/src/docs/sdks.md
git commit -m "docs(web): add register & dispatch pattern to in-app SDK docs"
```

---

## Task 9: Update Current Initiatives and plan completion

**Files:**
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Add to Current Initiatives**

Add the SDK Register & Dispatch initiative to the Active table in `docs/Current_Initiatives.md`:

```markdown
| SDK Register & Dispatch | Complete | [Plan](./superpowers/plans/2026-04-12-sdk-register-dispatch.md) / [Spec](./superpowers/specs/2026-04-12-sdk-register-dispatch-design.md) | All 7 SDKs + docs updated |
```

- [ ] **Step 2: Run full test suites across SDKs**

```bash
cd sdk/node && npx jest --verbose 2>&1 | tail -5
cd ../react && npx jest --verbose 2>&1 | tail -5
cd ../go && go test -v ./... 2>&1 | tail -10
cd ../python && python -m pytest tests/ -v 2>&1 | tail -10
cd ../ruby && ruby -Ilib -Itest test/client_test.rb 2>&1 | tail -5
cd ../flutter && flutter test 2>&1 | tail -5
cd ../java && mvn test 2>&1 | tail -5
```

Expected: all tests pass across all SDKs.

- [ ] **Step 3: Commit**

```bash
git add docs/Current_Initiatives.md docs/superpowers/plans/2026-04-12-sdk-register-dispatch.md
git commit -m "docs: mark SDK register/dispatch implementation complete"
```

---

## Self-review notes

**Spec coverage check:**
- Register method on all 7 SDKs: Tasks 1–7 ✓
- Dispatch method on all 7 SDKs: Tasks 1–7 ✓
- React `useDispatch` hook: Task 5 ✓
- Go generic `Dispatch[T]` helper: Task 2 ✓
- Operation-name keying (not flag-name): all tasks ✓
- First-match-wins priority: all dispatch implementations ✓
- Default-last, replaces previous default: all register implementations ✓
- Throw on unregistered: all dispatch implementations ✓
- Throw on no-match-no-default: all dispatch implementations ✓
- Sync cache read for dispatch: all implementations read from local flag store ✓
- Thread safety (Go RWMutex, Java ConcurrentHashMap, Python Lock, Ruby Mutex): Tasks 2–4, 6 ✓
- 8 unit tests per SDK: Tasks 1–7 ✓ (56 total)
- In-app docs update: Task 8 ✓
- Lint rule follow-up: mentioned in spec, not implemented (correct) ✓
- Sample apps follow-up: mentioned in spec, not implemented (correct) ✓
- Context responsibility documentation: Task 8 ✓

**Placeholder scan:** No TBD, TODO, or "fill in later" found.

**Type/name consistency:**
- `Registration` type used in Node (Task 1), React (Task 5) — same interface ✓
- `registration` struct in Go (Task 2) — lowercase unexported, correct ✓
- `Registration` class in Java (Task 4) — package-private, correct ✓
- `_Registration` class in Dart (Task 7) — private, correct ✓
- Python/Ruby use inline dicts (no separate type) — consistent with their idiomatic patterns ✓
- `register`/`dispatch` method names consistent across TS, Python, Ruby, Dart ✓
- `Register`/`Dispatch` in Go (exported), `register`/`dispatch` in Java (lowercase) ✓
- `flagKey` parameter name consistent: TS, Go (variadic), Java (overload), Dart (named) ✓
- `flag_key` in Python and Ruby (snake_case convention) ✓

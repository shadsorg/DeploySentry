# SDKs

DeploySentry ships seven first-party SDKs. Each follows the same shape: instantiate a client, evaluate flags, and (optionally) register dispatch handlers.

| Language | Package | Status |
|---|---|---|
| Go | `github.com/shadsorg/deploysentry-go` | Stable |
| Node | `@deploysentry/node` | Stable |
| Python | `deploysentry` | Stable |
| Java | `io.deploysentry:deploysentry` | Stable |
| Ruby | `deploysentry` | Stable |
| React | `@deploysentry/react` | Stable |
| Flutter | `deploysentry` | Stable |

## Node

```ts
import { DeploySentryClient } from '@deploysentry/sdk';

const ds = new DeploySentryClient({
  apiKey: process.env.DS_API_KEY!,
  environment: 'production',
  project: 'my-project',
  application: 'my-web-app',
});
await ds.initialize();
const isOn = await ds.boolValue('new-checkout', false, { userId: '42' });
```

## Go

```go
client := deploysentry.New(deploysentry.Options{
  APIKey:       os.Getenv("DS_API_KEY"),
  Environment:  "production",
  Project:      "my-project",
  Application:  "my-web-app",
})
on, _ := client.IsEnabled(ctx, "new-checkout", map[string]any{"userId": "42"})
```

## Python

```python
from deploysentry import DeploySentry

ds = DeploySentry(
    api_key=os.environ["DS_API_KEY"],
    environment="production",
    project="my-project",
    application="my-web-app",
)
on = ds.is_enabled("new-checkout", {"user_id": "42"})
```

## Register & Dispatch

Instead of scattering `if/else` flag checks throughout your code, register handler functions against a logical operation name and let the SDK pick the right one at call time.

### The pattern

1. **Register** handlers for each operation at app startup — flagged handlers first, default last:

```ts
// Node / React
ds.register('createCart', createCartWithMembership, 'membership-lookup');
ds.register('createCart', createCartWithLoyalty, 'loyalty-points');
ds.register('createCart', createCart); // default — always last
```

2. **Dispatch** at each call site — the SDK evaluates flags and returns the right function:

```ts
const result = ds.dispatch('createCart', ctx)(cartItems, user);
```

The context you pass to `dispatch` must include the attributes your targeting rules in the DeploySentry dashboard evaluate against (user ID, session ID, request headers, etc.).

### Why this matters

- **One place per operation** — every flag-gated code path is visible at the registration site.
- **Clean retirement** — delete the registration + the dead function. No archaeology.
- **LLM-ready** — an agent can scan registrations to find every flag's code and clean up automatically.

### Language examples

**Go:**
```go
client.Register("createCart", createCartWithMembership, "membership-lookup")
client.Register("createCart", createCart) // default
fn := client.Dispatch("createCart", ctx).(func(Cart, User) Result)
result := fn(cart, user)
```

**Python:**
```python
ds.register("create_cart", create_cart_with_membership, flag_key="membership-lookup")
ds.register("create_cart", create_cart)  # default
result = ds.dispatch("create_cart", ctx)(cart_items, user)
```

**Java:**
```java
client.register("createCart", () -> createCartWithMembership(cart, user), "membership-lookup");
client.register("createCart", () -> createCart(cart, user)); // default
var result = client.<Result>dispatch("createCart", ctx).get();
```

**Ruby:**
```ruby
client.register("create_cart", method(:create_cart_with_membership), flag_key: "membership-lookup")
client.register("create_cart", method(:create_cart)) # default
result = client.dispatch("create_cart", context: ctx).call(cart_items, user)
```

**Flutter (Dart):**
```dart
client.register<Result Function(Cart, User)>('createCart', createCartWithMembership, flagKey: 'membership-lookup');
client.register<Result Function(Cart, User)>('createCart', createCart); // default
final fn = client.dispatch<Result Function(Cart, User)>('createCart', context: ctx);
final result = fn(cart, user);
```

**React (hook):**
```tsx
// Registration happens at app init (same as Node)
ds.register('createCart', createCartWithMembership, 'membership-lookup');
ds.register('createCart', createCart);

// Inside a component
function CheckoutButton() {
  const createCart = useDispatch<(items: CartItem[]) => Result>('createCart');
  return <button onClick={() => createCart(items)}>Checkout</button>;
}
```

All seven SDKs support register & dispatch.

See each SDK's README in the [GitHub repo](https://github.com/shadsorg/DeploySentry/tree/main/sdk) for the full reference.

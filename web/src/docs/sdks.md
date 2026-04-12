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
import { DeploySentry } from '@deploysentry/node';

const ds = new DeploySentry({ apiKey: process.env.DS_API_KEY! });
const isOn = await ds.isEnabled('new-checkout', { userId: '42' });
```

## Go

```go
client := deploysentry.New(deploysentry.Options{APIKey: os.Getenv("DS_API_KEY")})
on, _ := client.IsEnabled(ctx, "new-checkout", map[string]any{"userId": "42"})
```

## Python

```python
from deploysentry import DeploySentry

ds = DeploySentry(api_key=os.environ["DS_API_KEY"])
on = ds.is_enabled("new-checkout", {"user_id": "42"})
```

See each SDK's README in the [GitHub repo](https://github.com/shadsorg/DeploySentry/tree/main/sdk) for the full reference.

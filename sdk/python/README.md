# DeploySentry Python SDK

Official Python SDK for integrating with the DeploySentry platform.

## Installation

```bash
pip install deploysentry
```

## Quick Start

```python
from deploysentry import DeploySentry

client = DeploySentry(api_key="your-api-key")

# Evaluate a feature flag
if client.flags.is_enabled("my-feature", context={"user_id": "user-123"}):
    # New feature code path
    pass
```

## Documentation

Full documentation is available at [docs.deploysentry.io/sdk/python](https://docs.deploysentry.io/sdk/python).

## License

Apache-2.0

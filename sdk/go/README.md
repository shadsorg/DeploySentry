# DeploySentry Go SDK

Official Go SDK for integrating with the DeploySentry platform.

## Installation

```bash
go get github.com/deploysentry/deploysentry-go
```

## Quick Start

```go
package main

import (
    deploysentry "github.com/deploysentry/deploysentry-go"
)

func main() {
    client := deploysentry.NewClient("your-api-key")

    // Evaluate a feature flag
    enabled, err := client.Flags.IsEnabled("my-feature", deploysentry.Context{
        UserID: "user-123",
    })
    if err != nil {
        panic(err)
    }

    if enabled {
        // New feature code path
    }
}
```

## Documentation

Full documentation is available at [docs.deploysentry.io/sdk/go](https://docs.deploysentry.io/sdk/go).

## License

Apache-2.0

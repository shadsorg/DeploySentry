# DeploySentry Java SDK

Official Java SDK for integrating with the DeploySentry platform.

## Installation

### Maven

```xml
<dependency>
    <groupId>io.deploysentry</groupId>
    <artifactId>deploysentry-java</artifactId>
    <version>0.1.0</version>
</dependency>
```

### Gradle

```groovy
implementation 'io.deploysentry:deploysentry-java:0.1.0'
```

## Quick Start

```java
import io.deploysentry.DeploySentry;
import io.deploysentry.FlagContext;

DeploySentry client = DeploySentry.builder()
    .apiKey("your-api-key")
    .build();

// Evaluate a feature flag
boolean enabled = client.flags().isEnabled("my-feature",
    FlagContext.builder().userId("user-123").build());

if (enabled) {
    // New feature code path
}
```

## Documentation

Full documentation is available at [docs.deploysentry.io/sdk/java](https://docs.deploysentry.io/sdk/java).

## License

Apache-2.0

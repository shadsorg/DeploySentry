# DeploySentry Flutter SDK

Official Flutter SDK for integrating with the DeploySentry feature flag platform. Supports typed flag evaluation, rich metadata, in-memory caching, and real-time SSE streaming.

## Installation

Add to your `pubspec.yaml`:

```yaml
dependencies:
  deploysentry_flutter: ^1.0.0
```

## Quick Start

```dart
import 'package:deploysentry_flutter/deploysentry_flutter.dart';

final client = DeploySentryClient(
  apiKey: 'your-api-key',
  baseUrl: 'https://api.deploysentry.io',
  environment: 'production',
  project: 'my-project',
);

await client.initialize();

// Boolean flag
final enabled = await client.boolValue('dark-mode', defaultValue: false);

// String flag with evaluation context
final variant = await client.stringValue(
  'checkout-flow',
  defaultValue: 'control',
  context: EvaluationContext(userId: 'user-123', orgId: 'org-456'),
);

// Integer flag
final maxRetries = await client.intValue('max-retries', defaultValue: 3);

// JSON flag
final config = await client.jsonValue('ui-config', defaultValue: {});

// Full evaluation detail with metadata
final result = await client.detail('new-pricing');
print(result.metadata.category);  // FlagCategory.experiment
print(result.metadata.owners);    // ['team-growth']
print(result.reason);             // 'TARGETING_MATCH'

// Clean up
client.close();
```

## Widget Tree Integration

Use `DeploySentryProvider` to make the client available throughout your widget tree:

```dart
void main() async {
  final client = DeploySentryClient(
    apiKey: 'your-api-key',
    baseUrl: 'https://api.deploysentry.io',
    project: 'my-project',
  );
  await client.initialize();

  runApp(
    DeploySentryProvider(
      client: client,
      child: MyApp(),
    ),
  );
}

// Access from any widget
class MyWidget extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final ds = DeploySentry.of(context);

    return FutureBuilder<bool>(
      future: ds.boolValue('show-banner'),
      builder: (context, snapshot) {
        if (snapshot.data == true) {
          return Banner();
        }
        return SizedBox.shrink();
      },
    );
  }
}
```

## Metadata Queries

```dart
// Get all flags in a category
final experiments = client.flagsByCategory(FlagCategory.experiment);

// Find expired flags
final expired = client.expiredFlags();

// Get flag owners
final owners = client.flagOwners('checkout-flow');
```

## Flag Categories

- `FlagCategory.release` -- Gradual rollouts and releases
- `FlagCategory.feature` -- Feature toggles
- `FlagCategory.experiment` -- A/B tests and experiments
- `FlagCategory.ops` -- Operational flags (kill switches, rate limits)
- `FlagCategory.permission` -- Permission and entitlement flags

## Offline Mode

```dart
final client = DeploySentryClient(
  apiKey: 'your-api-key',
  baseUrl: 'https://api.deploysentry.io',
  offlineMode: true,
);
```

In offline mode the client skips network calls and resolves flags only from the local cache.

## API Endpoints

The SDK communicates with the following DeploySentry API endpoints:

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/v1/flags?project_id=X` | Fetch all flags |
| POST | `/api/v1/flags/evaluate` | Evaluate a single flag |
| GET | `/api/v1/flags/stream` | SSE stream for real-time updates |

## License

Apache-2.0

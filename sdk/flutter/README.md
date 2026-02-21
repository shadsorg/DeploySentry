# DeploySentry Flutter SDK

Official Flutter SDK for integrating with the DeploySentry platform.

## Installation

Add to your `pubspec.yaml`:

```yaml
dependencies:
  deploysentry_flutter: ^0.1.0
```

## Quick Start

```dart
import 'package:deploysentry_flutter/deploysentry_flutter.dart';

final client = DeploySentry(apiKey: 'your-api-key');

// Evaluate a feature flag
final enabled = await client.flags.isEnabled(
  'my-feature',
  context: FlagContext(userId: 'user-123'),
);

if (enabled) {
  // New feature code path
}
```

## Documentation

Full documentation is available at [docs.deploysentry.io/sdk/flutter](https://docs.deploysentry.io/sdk/flutter).

## License

Apache-2.0

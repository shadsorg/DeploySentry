import 'package:flutter/widgets.dart';

import 'client.dart';

/// InheritedWidget that provides a [DeploySentryClient] to the widget tree.
///
/// Wrap your app (or a subtree) with [DeploySentryProvider] and then access the
/// client anywhere below via [DeploySentry.of(context)].
///
/// ```dart
/// DeploySentryProvider(
///   client: myClient,
///   child: MyApp(),
/// )
/// ```
class DeploySentryProvider extends InheritedWidget {
  final DeploySentryClient client;

  const DeploySentryProvider({
    super.key,
    required this.client,
    required super.child,
  });

  @override
  bool updateShouldNotify(DeploySentryProvider oldWidget) {
    return client != oldWidget.client;
  }
}

/// Static accessor for retrieving the [DeploySentryClient] from the widget tree.
class DeploySentry {
  DeploySentry._();

  /// Retrieve the [DeploySentryClient] from the nearest [DeploySentryProvider]
  /// ancestor, or throw if none exists.
  static DeploySentryClient of(BuildContext context) {
    final provider =
        context.dependOnInheritedWidgetOfExactType<DeploySentryProvider>();
    assert(provider != null,
        'No DeploySentryProvider found in the widget tree. '
        'Wrap your app with DeploySentryProvider.');
    return provider!.client;
  }

  /// Retrieve the [DeploySentryClient] from the nearest [DeploySentryProvider]
  /// ancestor, or return null if none exists.
  static DeploySentryClient? maybeOf(BuildContext context) {
    final provider =
        context.dependOnInheritedWidgetOfExactType<DeploySentryProvider>();
    return provider?.client;
  }
}

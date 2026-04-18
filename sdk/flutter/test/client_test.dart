import 'package:test/test.dart';
import 'package:dr_sentry_flutter/dr_sentry_flutter.dart';

void main() {
  group('DeploySentryClient', () {
    late DeploySentryClient client;

    setUp(() {
      client = DeploySentryClient(
        apiKey: 'test-api-key',
        baseUrl: 'http://localhost:8080',
        offlineMode: true,
      );
    });

    tearDown(() {
      client.close();
    });

    test('isInitialized is false before initialize()', () {
      expect(client.isInitialized, isFalse);
    });

    test('isInitialized is true after initialize()', () async {
      await client.initialize();
      expect(client.isInitialized, isTrue);
    });

    test('boolValue returns default in offline mode', () async {
      await client.initialize();
      final result = await client.boolValue('nonexistent-flag', defaultValue: true);
      expect(result, isTrue);
    });

    test('boolValue returns false as default when no defaultValue provided', () async {
      await client.initialize();
      final result = await client.boolValue('nonexistent-flag');
      expect(result, isFalse);
    });

    test('stringValue returns default in offline mode', () async {
      await client.initialize();
      final result = await client.stringValue('nonexistent-flag', defaultValue: 'fallback');
      expect(result, equals('fallback'));
    });

    test('stringValue returns empty string as default when no defaultValue provided', () async {
      await client.initialize();
      final result = await client.stringValue('nonexistent-flag');
      expect(result, isEmpty);
    });

    test('intValue returns default in offline mode', () async {
      await client.initialize();
      final result = await client.intValue('nonexistent-flag', defaultValue: 42);
      expect(result, equals(42));
    });

    test('intValue returns 0 as default when no defaultValue provided', () async {
      await client.initialize();
      final result = await client.intValue('nonexistent-flag');
      expect(result, equals(0));
    });
  });

  // ---------------------------------------------------------------------------
  // register / dispatch tests
  // ---------------------------------------------------------------------------

  group('register/dispatch', () {
    late DeploySentryClient client;

    setUp(() {
      client = DeploySentryClient(
        apiKey: 'test-api-key',
        baseUrl: 'http://localhost:8080',
        offlineMode: true,
      );
    });

    tearDown(() {
      client.close();
    });

    void seedFlag(String key, {required bool enabled}) {
      client.seedFlagForTesting(Flag(
        key: key,
        value: enabled,
        valueType: 'boolean',
        enabled: enabled,
      ));
    }

    // 1. flagged-on: when the flag is enabled, the flag-specific handler is returned
    test('dispatch returns flagged handler when flag is enabled', () {
      String Function() flagHandler = () => 'flagged';
      String Function() defaultHandler = () => 'default';
      client.register('send_email', defaultHandler);
      client.register('send_email', flagHandler, flagKey: 'new-mailer');
      seedFlag('new-mailer', enabled: true);

      final result = client.dispatch<String Function()>('send_email');
      expect(result(), equals('flagged'));
    });

    // 2. flagged-off/default: when the flag is disabled, the default handler is returned
    test('dispatch returns default handler when flag is disabled', () {
      String Function() flagHandler = () => 'flagged';
      String Function() defaultHandler = () => 'default';
      client.register('send_email', defaultHandler);
      client.register('send_email', flagHandler, flagKey: 'new-mailer');
      seedFlag('new-mailer', enabled: false);

      final result = client.dispatch<String Function()>('send_email');
      expect(result(), equals('default'));
    });

    // 3. first-match-wins: first enabled flag in registration order wins
    test('dispatch returns first matching enabled flag handler', () {
      String Function() handlerA = () => 'handler_a';
      String Function() handlerB = () => 'handler_b';
      String Function() defaultHandler = () => 'default';
      client.register('op', defaultHandler);
      client.register('op', handlerA, flagKey: 'flag-a');
      client.register('op', handlerB, flagKey: 'flag-b');
      seedFlag('flag-a', enabled: true);
      seedFlag('flag-b', enabled: true);

      final result = client.dispatch<String Function()>('op');
      expect(result(), equals('handler_a'));
    });

    // 4. default-only: works fine when no flag-specific handlers are registered
    test('dispatch works with default-only registration', () {
      String Function() defaultHandler = () => 'only_default';
      client.register('render', defaultHandler);

      final result = client.dispatch<String Function()>('render');
      expect(result(), equals('only_default'));
    });

    // 5. isolation: registrations for different operations are independent
    test('dispatch operations are isolated from each other', () {
      String Function() handlerA = () => 'operation_a';
      String Function() handlerB = () => 'operation_b';
      client.register('op_a', handlerA);
      client.register('op_b', handlerB);

      expect(client.dispatch<String Function()>('op_a')(), equals('operation_a'));
      expect(client.dispatch<String Function()>('op_b')(), equals('operation_b'));
    });

    // 6. throw unregistered: raises when no handlers exist for the operation
    test('dispatch throws StateError when operation not registered', () {
      expect(
        () => client.dispatch<Function>('unknown_op'),
        throwsStateError,
      );
    });

    // 7. throw no-match-no-default: raises when flag is disabled and no default exists
    test('dispatch throws StateError when no default and flag is disabled', () {
      String Function() flagHandler = () => 'flagged';
      client.register('op', flagHandler, flagKey: 'my-flag');
      seedFlag('my-flag', enabled: false);

      expect(
        () => client.dispatch<Function>('op'),
        throwsStateError,
      );
    });

    // 8. replace default: re-registering without flagKey replaces the previous default
    test('register replaces existing default handler', () {
      String Function() oldDefault = () => 'old';
      String Function() newDefault = () => 'new';
      client.register('op', oldDefault);
      client.register('op', newDefault);

      final result = client.dispatch<String Function()>('op');
      expect(result(), equals('new'));
    });

    // 9. pass-through args: returned handler can be called with arguments
    test('dispatched handler accepts arguments', () {
      int Function(int, int) handler = (x, y) => x + y;
      client.register('add', handler);

      final result = client.dispatch<int Function(int, int)>('add');
      expect(result(3, 4), equals(7));
    });
  });
}

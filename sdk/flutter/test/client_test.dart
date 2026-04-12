import 'package:test/test.dart';
import 'package:deploysentry_flutter/deploysentry_flutter.dart';

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
}

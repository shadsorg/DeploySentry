import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

import 'cache.dart';
import 'models.dart';
import 'streaming.dart';

/// Main client for the DeploySentry feature flag service.
///
/// Provides typed flag evaluation, metadata inspection, real-time streaming
/// updates, and an in-memory cache.
class DeploySentryClient {
  final String apiKey;
  final String baseUrl;
  final String? environment;
  final String? project;
  final String? sessionId;
  final Duration cacheTimeout;
  final bool offlineMode;
  final String? sessionId;

  late final FlagCache _cache;
  late final http.Client _httpClient;
  FlagStreamClient? _streamClient;
  StreamSubscription<Flag>? _streamSubscription;
  bool _initialized = false;

  DeploySentryClient({
    required this.apiKey,
    required this.baseUrl,
    this.environment,
    this.project,
    this.sessionId,
    this.cacheTimeout = const Duration(minutes: 5),
    this.offlineMode = false,
    this.sessionId,
  }) {
    _cache = FlagCache(timeout: cacheTimeout);
    _httpClient = http.Client();
  }

  /// Common HTTP headers for API requests.
  Map<String, String> get _headers => {
        'Authorization': 'ApiKey $apiKey',
        'Content-Type': 'application/json',
        if (environment != null) 'X-Environment': environment!,
        if (sessionId != null) 'X-DeploySentry-Session': sessionId!,
      };

  /// Whether the client has been initialized.
  bool get isInitialized => _initialized;

  // ---------------------------------------------------------------------------
  // Lifecycle
  // ---------------------------------------------------------------------------

  /// Initialize the client by fetching all flags and starting the SSE stream.
  Future<void> initialize() async {
    if (_initialized) return;

    if (!offlineMode) {
      await _fetchAllFlags();
      _startStreaming();
    }

    _initialized = true;
  }

  /// Close the client and release all resources.
  void close() {
    _streamSubscription?.cancel();
    _streamSubscription = null;
    _streamClient?.close();
    _streamClient = null;
    _httpClient.close();
    _cache.clear();
    _initialized = false;
  }

  /// Clear the local cache and re-fetch all flags from the API.
  /// Useful when a new session starts and fresh flag state is required.
  Future<void> refreshSession() async {
    _cache.clear();
    await _fetchAllFlags();
  }

  // ---------------------------------------------------------------------------
  // Flag evaluation — typed convenience methods
  // ---------------------------------------------------------------------------

  /// Evaluate a boolean flag.
  Future<bool> boolValue(
    String key, {
    bool defaultValue = false,
    EvaluationContext? context,
  }) async {
    final result = await _evaluate(key, context: context);
    if (result == null) return defaultValue;
    if (result.value is bool) return result.value as bool;
    return defaultValue;
  }

  /// Evaluate a string flag.
  Future<String> stringValue(
    String key, {
    String defaultValue = '',
    EvaluationContext? context,
  }) async {
    final result = await _evaluate(key, context: context);
    if (result == null) return defaultValue;
    return result.value?.toString() ?? defaultValue;
  }

  /// Evaluate an integer flag.
  Future<int> intValue(
    String key, {
    int defaultValue = 0,
    EvaluationContext? context,
  }) async {
    final result = await _evaluate(key, context: context);
    if (result == null) return defaultValue;
    if (result.value is int) return result.value as int;
    if (result.value is num) return (result.value as num).toInt();
    return int.tryParse(result.value.toString()) ?? defaultValue;
  }

  /// Evaluate a JSON (map) flag.
  Future<Map<String, dynamic>> jsonValue(
    String key, {
    Map<String, dynamic> defaultValue = const {},
    EvaluationContext? context,
  }) async {
    final result = await _evaluate(key, context: context);
    if (result == null) return defaultValue;
    if (result.value is Map<String, dynamic>) {
      return result.value as Map<String, dynamic>;
    }
    if (result.value is String) {
      try {
        return jsonDecode(result.value as String) as Map<String, dynamic>;
      } catch (_) {
        return defaultValue;
      }
    }
    return defaultValue;
  }

  /// Evaluate a flag and return the full [EvaluationResult] with metadata.
  Future<EvaluationResult> detail(
    String key, {
    EvaluationContext? context,
  }) async {
    final result = await _evaluate(key, context: context);
    return result ??
        EvaluationResult(
          key: key,
          value: null,
          valueType: 'boolean',
          reason: 'FLAG_NOT_FOUND',
        );
  }

  // ---------------------------------------------------------------------------
  // Metadata queries
  // ---------------------------------------------------------------------------

  /// Return all cached flags that belong to the given [category].
  List<Flag> flagsByCategory(FlagCategory category) {
    return _cache
        .getAll()
        .where((f) => f.metadata.category == category)
        .toList();
  }

  /// Return all cached flags whose expiration date has passed.
  List<Flag> expiredFlags() {
    return _cache.getAll().where((f) => f.metadata.isExpired).toList();
  }

  /// Return the owners list for a specific flag key, or an empty list if
  /// the flag is not cached.
  List<String> flagOwners(String key) {
    final flag = _cache.get(key);
    return flag?.metadata.owners ?? [];
  }

  // ---------------------------------------------------------------------------
  // Internal
  // ---------------------------------------------------------------------------

  Future<EvaluationResult?> _evaluate(
    String key, {
    EvaluationContext? context,
  }) async {
    if (offlineMode) {
      final cached = _cache.get(key);
      if (cached != null) {
        return EvaluationResult(
          key: cached.key,
          value: cached.value,
          valueType: cached.valueType,
          reason: 'CACHE',
          metadata: cached.metadata,
          enabled: cached.enabled,
        );
      }
      return null;
    }

    try {
      final body = {
        'flag_key': key,
        if (project != null) 'project_id': project,
        if (context != null) 'context': context.toJson(),
      };

      final response = await _httpClient.post(
        Uri.parse('$baseUrl/api/v1/flags/evaluate'),
        headers: _headers,
        body: jsonEncode(body),
      );

      if (response.statusCode == 200) {
        final json = jsonDecode(response.body) as Map<String, dynamic>;
        final result = EvaluationResult.fromJson(json);

        // Update the cache with the latest value.
        _cache.put(Flag(
          key: result.key,
          value: result.value,
          valueType: result.valueType,
          enabled: result.enabled,
          metadata: result.metadata,
        ));

        return result;
      }
    } catch (_) {
      // Fall back to cache on network errors.
    }

    // Return cached value as fallback.
    final cached = _cache.get(key);
    if (cached != null) {
      return EvaluationResult(
        key: cached.key,
        value: cached.value,
        valueType: cached.valueType,
        reason: 'CACHE',
        metadata: cached.metadata,
        enabled: cached.enabled,
      );
    }

    return null;
  }

  Future<void> _fetchAllFlags() async {
    try {
      final queryParams = <String, String>{};
      if (project != null) queryParams['project_id'] = project!;

      final uri = Uri.parse('$baseUrl/api/v1/flags')
          .replace(queryParameters: queryParams.isNotEmpty ? queryParams : null);

      final response = await _httpClient.get(uri, headers: _headers);

      if (response.statusCode == 200) {
        final json = jsonDecode(response.body);
        final List<dynamic> flagsList =
            json is List ? json : (json as Map<String, dynamic>)['flags'] ?? [];

        final flags = flagsList
            .map((e) => Flag.fromJson(e as Map<String, dynamic>))
            .toList();

        _cache.putAll(flags);
      }
    } catch (_) {
      // Initialization continues even if the fetch fails; the cache will be
      // empty and flags will be fetched on demand.
    }
  }

  void _startStreaming() {
    final queryParams = <String, String>{};
    if (project != null) queryParams['project_id'] = project!;

    final streamUri = Uri.parse('$baseUrl/api/v1/flags/stream')
        .replace(queryParameters: queryParams.isNotEmpty ? queryParams : null);

    _streamClient = FlagStreamClient(
      url: streamUri.toString(),
      headers: _headers,
    );

    _streamSubscription = _streamClient!.updates.listen((flag) {
      _cache.put(flag);
    });
  }
}

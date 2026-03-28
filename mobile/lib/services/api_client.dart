import 'dart:convert';
import 'dart:io';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import '../models/flag.dart';
import '../models/deployment.dart';
import '../models/release.dart';
import '../models/analytics.dart';

class ApiException implements Exception {
  final String message;
  final int? statusCode;
  final Map<String, dynamic>? details;

  ApiException(this.message, {this.statusCode, this.details});

  @override
  String toString() =>
      'ApiException: $message${statusCode != null ? ' (Status: $statusCode)' : ''}';
}

class ApiClient {
  static const String _baseUrl = 'http://localhost:8080/api/v1';
  static const String _tokenKey = 'ds_token';
  static const _storage = FlutterSecureStorage();

  final http.Client _client = http.Client();

  // Authentication methods
  Future<void> setToken(String token) async {
    await _storage.write(key: _tokenKey, value: token);
  }

  Future<String?> getToken() async {
    return await _storage.read(key: _tokenKey);
  }

  Future<void> clearToken() async {
    await _storage.delete(key: _tokenKey);
  }

  // Private helper methods
  Future<Map<String, String>> _getHeaders() async {
    final token = await getToken();
    final headers = {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
    };

    if (token != null) {
      if (token.startsWith('ds_')) {
        headers['Authorization'] = 'ApiKey $token';
      } else {
        headers['Authorization'] = 'Bearer $token';
      }
    }

    return headers;
  }

  Future<T> _request<T>(
    String method,
    String path, {
    Map<String, dynamic>? body,
    Map<String, String>? queryParams,
    T Function(Map<String, dynamic>)? fromJson,
  }) async {
    final uri = Uri.parse('$_baseUrl$path').replace(queryParameters: queryParams);
    final headers = await _getHeaders();

    late http.Response response;

    try {
      switch (method.toUpperCase()) {
        case 'GET':
          response = await _client.get(uri, headers: headers);
          break;
        case 'POST':
          response = await _client.post(
            uri,
            headers: headers,
            body: body != null ? jsonEncode(body) : null,
          );
          break;
        case 'PUT':
          response = await _client.put(
            uri,
            headers: headers,
            body: body != null ? jsonEncode(body) : null,
          );
          break;
        case 'DELETE':
          response = await _client.delete(uri, headers: headers);
          break;
        default:
          throw ArgumentError('Unsupported HTTP method: $method');
      }
    } on SocketException {
      throw ApiException('Network error: Unable to connect to server');
    } catch (e) {
      throw ApiException('Request failed: $e');
    }

    if (response.statusCode >= 200 && response.statusCode < 300) {
      if (response.body.isEmpty) {
        return {} as T;
      }

      final jsonData = jsonDecode(response.body) as Map<String, dynamic>;

      if (fromJson != null) {
        return fromJson(jsonData);
      }
      return jsonData as T;
    } else {
      Map<String, dynamic> errorBody = {};
      try {
        errorBody = jsonDecode(response.body) as Map<String, dynamic>;
      } catch (_) {
        // Ignore JSON decode errors
      }

      final errorMessage = errorBody['error'] ??
                          errorBody['message'] ??
                          'Request failed: ${response.statusCode}';

      throw ApiException(
        errorMessage,
        statusCode: response.statusCode,
        details: errorBody,
      );
    }
  }

  // Flag API methods
  Future<List<Flag>> listFlags({
    required String projectId,
    String? category,
    bool? archived,
  }) async {
    final queryParams = <String, String>{'project_id': projectId};
    if (category != null) queryParams['category'] = category;
    if (archived != null) queryParams['archived'] = archived.toString();

    final response = await _request<Map<String, dynamic>>(
      'GET',
      '/flags',
      queryParams: queryParams,
    );

    final flagsData = response['flags'] as List<dynamic>;
    return flagsData.map((json) => Flag.fromJson(json as Map<String, dynamic>)).toList();
  }

  Future<Flag> getFlag(String id) async {
    final response = await _request<Map<String, dynamic>>(
      'GET',
      '/flags/$id',
    );
    return Flag.fromJson(response);
  }

  Future<Flag> createFlag(CreateFlagRequest request) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/flags',
      body: request.toJson(),
    );
    return Flag.fromJson(response);
  }

  Future<Flag> updateFlag(String id, UpdateFlagRequest request) async {
    final response = await _request<Map<String, dynamic>>(
      'PUT',
      '/flags/$id',
      body: request.toJson(),
    );
    return Flag.fromJson(response);
  }

  Future<Map<String, dynamic>> toggleFlag(String id, bool enabled) async {
    return await _request<Map<String, dynamic>>(
      'POST',
      '/flags/$id/toggle',
      body: {'enabled': enabled},
    );
  }

  Future<Map<String, dynamic>> archiveFlag(String id) async {
    return await _request<Map<String, dynamic>>(
      'POST',
      '/flags/$id/archive',
    );
  }

  // Deployment API methods
  Future<List<Deployment>> listDeployments(String projectId) async {
    final response = await _request<Map<String, dynamic>>(
      'GET',
      '/deployments',
      queryParams: {'project_id': projectId},
    );

    final deploymentsData = response['deployments'] as List<dynamic>;
    return deploymentsData
        .map((json) => Deployment.fromJson(json as Map<String, dynamic>))
        .toList();
  }

  Future<Deployment> getDeployment(String id) async {
    final response = await _request<Map<String, dynamic>>(
      'GET',
      '/deployments/$id',
    );
    return Deployment.fromJson(response);
  }

  Future<Deployment> createDeployment(CreateDeploymentRequest request) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/deployments',
      body: request.toJson(),
    );
    return Deployment.fromJson(response);
  }

  Future<Deployment> promoteDeployment(String id) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/deployments/$id/promote',
    );
    return Deployment.fromJson(response);
  }

  Future<Deployment> rollbackDeployment(String id) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/deployments/$id/rollback',
    );
    return Deployment.fromJson(response);
  }

  Future<Deployment> pauseDeployment(String id) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/deployments/$id/pause',
    );
    return Deployment.fromJson(response);
  }

  Future<Deployment> resumeDeployment(String id) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/deployments/$id/resume',
    );
    return Deployment.fromJson(response);
  }

  // Deployment API methods
  Future<List<Deployment>> getDeployments({
    required String projectId,
    String? environmentId,
    int? limit,
    int? offset,
  }) async {
    final queryParams = {
      'project_id': projectId,
    };
    if (environmentId != null) queryParams['environment_id'] = environmentId;
    if (limit != null) queryParams['limit'] = limit.toString();
    if (offset != null) queryParams['offset'] = offset.toString();

    final response = await _request<Map<String, dynamic>>(
      'GET',
      '/deployments',
      queryParams: queryParams,
    );

    final deploymentsData = response['deployments'] as List<dynamic>;
    return deploymentsData
        .map((json) => Deployment.fromJson(json as Map<String, dynamic>))
        .toList();
  }

  Future<Deployment> getDeployment(String id) async {
    final response = await _request<Map<String, dynamic>>(
      'GET',
      '/deployments/$id',
    );
    return Deployment.fromJson(response);
  }

  Future<Deployment> createDeployment(CreateDeploymentRequest request) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/deployments',
      body: request.toJson(),
    );
    return Deployment.fromJson(response);
  }

  Future<Deployment> pauseDeployment(String id) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/deployments/$id/pause',
    );
    return Deployment.fromJson(response);
  }

  Future<Deployment> resumeDeployment(String id) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/deployments/$id/resume',
    );
    return Deployment.fromJson(response);
  }

  Future<Deployment> retryDeployment(String id) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/deployments/$id/retry',
    );
    return Deployment.fromJson(response);
  }

  // Release API methods
  Future<List<Release>> getReleases({
    required String projectId,
    int? limit,
    int? offset,
  }) async {
    final queryParams = {
      'project_id': projectId,
    };
    if (limit != null) queryParams['limit'] = limit.toString();
    if (offset != null) queryParams['offset'] = offset.toString();

    final response = await _request<Map<String, dynamic>>(
      'GET',
      '/releases',
      queryParams: queryParams,
    );

    final releasesData = response['releases'] as List<dynamic>;
    return releasesData
        .map((json) => Release.fromJson(json as Map<String, dynamic>))
        .toList();
  }

  Future<List<Release>> listReleases(String projectId) async {
    return getReleases(projectId: projectId);
  }

  Future<Release> getRelease(String id) async {
    final response = await _request<Map<String, dynamic>>(
      'GET',
      '/releases/$id',
    );
    return Release.fromJson(response);
  }

  Future<Release> createRelease(CreateReleaseRequest request) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/releases',
      body: request.toJson(),
    );
    return Release.fromJson(response);
  }

  Future<Release> promoteRelease(String id, String environmentId) async {
    final response = await _request<Map<String, dynamic>>(
      'POST',
      '/releases/$id/promote',
      body: {'environment_id': environmentId},
    );
    return Release.fromJson(response);
  }

  // Analytics API methods
  Future<AnalyticsSummary> getAnalyticsSummary({
    required String projectId,
    required String environmentId,
    required String timeRange,
  }) async {
    final response = await _request<Map<String, dynamic>>(
      'GET',
      '/analytics/summary',
      queryParams: {
        'project_id': projectId,
        'environment_id': environmentId,
        'time_range': timeRange,
      },
    );

    return AnalyticsSummary.fromJson(response['summary'] as Map<String, dynamic>);
  }

  Future<List<FlagStats>> getFlagStats({
    required String projectId,
    required String environmentId,
    required String timeRange,
    int? limit,
  }) async {
    final queryParams = {
      'project_id': projectId,
      'environment_id': environmentId,
      'time_range': timeRange,
    };
    if (limit != null) queryParams['limit'] = limit.toString();

    final response = await _request<Map<String, dynamic>>(
      'GET',
      '/analytics/flags/stats',
      queryParams: queryParams,
    );

    final flagsData = response['flags'] as List<dynamic>;
    return flagsData
        .map((json) => FlagStats.fromJson(json as Map<String, dynamic>))
        .toList();
  }

  Future<SystemHealth> getSystemHealth() async {
    final response = await _request<Map<String, dynamic>>(
      'GET',
      '/analytics/health',
    );
    return SystemHealth.fromJson(response['health'] as Map<String, dynamic>);
  }

  // Health check methods
  Future<Map<String, dynamic>> healthCheck() async {
    return await _request<Map<String, dynamic>>('GET', '/health');
  }

  void dispose() {
    _client.close();
  }
}

// Singleton instance
final apiClient = ApiClient();
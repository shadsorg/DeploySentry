import 'package:json_annotation/json_annotation.dart';

part 'analytics.g.dart';

@JsonSerializable()
class AnalyticsSummary {
  final FlagPerformance flags;
  final DeploymentPerformance deployments;
  final ApiPerformance api;

  const AnalyticsSummary({
    required this.flags,
    required this.deployments,
    required this.api,
  });

  factory AnalyticsSummary.fromJson(Map<String, dynamic> json) =>
      _$AnalyticsSummaryFromJson(json);

  Map<String, dynamic> toJson() => _$AnalyticsSummaryToJson(this);
}

@JsonSerializable()
class FlagPerformance {
  @JsonKey(name: 'total_evaluations')
  final int totalEvaluations;
  @JsonKey(name: 'active_flags')
  final int activeFlags;
  @JsonKey(name: 'cache_hit_rate')
  final double cacheHitRate;
  @JsonKey(name: 'average_latency_ms')
  final double averageLatencyMs;

  const FlagPerformance({
    required this.totalEvaluations,
    required this.activeFlags,
    required this.cacheHitRate,
    required this.averageLatencyMs,
  });

  factory FlagPerformance.fromJson(Map<String, dynamic> json) =>
      _$FlagPerformanceFromJson(json);

  Map<String, dynamic> toJson() => _$FlagPerformanceToJson(this);
}

@JsonSerializable()
class DeploymentPerformance {
  @JsonKey(name: 'total_deployments')
  final int totalDeployments;
  @JsonKey(name: 'success_rate')
  final double successRate;
  @JsonKey(name: 'average_duration_minutes')
  final double averageDurationMinutes;
  @JsonKey(name: 'failed_deployments')
  final int failedDeployments;

  const DeploymentPerformance({
    required this.totalDeployments,
    required this.successRate,
    required this.averageDurationMinutes,
    required this.failedDeployments,
  });

  factory DeploymentPerformance.fromJson(Map<String, dynamic> json) =>
      _$DeploymentPerformanceFromJson(json);

  Map<String, dynamic> toJson() => _$DeploymentPerformanceToJson(this);
}

@JsonSerializable()
class ApiPerformance {
  @JsonKey(name: 'total_requests')
  final int totalRequests;
  @JsonKey(name: 'average_latency_ms')
  final double averageLatencyMs;
  @JsonKey(name: 'error_rate')
  final double errorRate;
  @JsonKey(name: 'requests_per_second')
  final double requestsPerSecond;

  const ApiPerformance({
    required this.totalRequests,
    required this.averageLatencyMs,
    required this.errorRate,
    required this.requestsPerSecond,
  });

  factory ApiPerformance.fromJson(Map<String, dynamic> json) =>
      _$ApiPerformanceFromJson(json);

  Map<String, dynamic> toJson() => _$ApiPerformanceToJson(this);
}

@JsonSerializable()
class FlagStats {
  @JsonKey(name: 'flag_key')
  final String flagKey;
  @JsonKey(name: 'total_evaluations')
  final int totalEvaluations;
  @JsonKey(name: 'cache_hit_rate')
  final double cacheHitRate;
  @JsonKey(name: 'average_latency_ms')
  final double averageLatencyMs;
  @JsonKey(name: 'error_rate')
  final double errorRate;
  @JsonKey(name: 'result_distribution')
  final Map<String, int> resultDistribution;

  const FlagStats({
    required this.flagKey,
    required this.totalEvaluations,
    required this.cacheHitRate,
    required this.averageLatencyMs,
    required this.errorRate,
    required this.resultDistribution,
  });

  factory FlagStats.fromJson(Map<String, dynamic> json) =>
      _$FlagStatsFromJson(json);

  Map<String, dynamic> toJson() => _$FlagStatsToJson(this);
}

@JsonSerializable()
class SystemHealth {
  final ApiHealthMetrics api;
  final DatabaseHealthMetrics database;
  final ResourceMetrics resources;
  final String timestamp;

  const SystemHealth({
    required this.api,
    required this.database,
    required this.resources,
    required this.timestamp,
  });

  factory SystemHealth.fromJson(Map<String, dynamic> json) =>
      _$SystemHealthFromJson(json);

  Map<String, dynamic> toJson() => _$SystemHealthToJson(this);
}

@JsonSerializable()
class ApiHealthMetrics {
  @JsonKey(name: 'requests_per_second')
  final double requestsPerSecond;
  @JsonKey(name: 'avg_latency_ms')
  final double avgLatencyMs;
  @JsonKey(name: 'error_rate')
  final double errorRate;
  @JsonKey(name: 'active_connections')
  final int activeConnections;

  const ApiHealthMetrics({
    required this.requestsPerSecond,
    required this.avgLatencyMs,
    required this.errorRate,
    required this.activeConnections,
  });

  factory ApiHealthMetrics.fromJson(Map<String, dynamic> json) =>
      _$ApiHealthMetricsFromJson(json);

  Map<String, dynamic> toJson() => _$ApiHealthMetricsToJson(this);
}

@JsonSerializable()
class DatabaseHealthMetrics {
  final int connections;
  @JsonKey(name: 'query_latency_ms')
  final double queryLatencyMs;
  @JsonKey(name: 'cache_hit_rate')
  final double cacheHitRate;

  const DatabaseHealthMetrics({
    required this.connections,
    required this.queryLatencyMs,
    required this.cacheHitRate,
  });

  factory DatabaseHealthMetrics.fromJson(Map<String, dynamic> json) =>
      _$DatabaseHealthMetricsFromJson(json);

  Map<String, dynamic> toJson() => _$DatabaseHealthMetricsToJson(this);
}

@JsonSerializable()
class ResourceMetrics {
  @JsonKey(name: 'cpu_usage_percent')
  final double cpuUsagePercent;
  @JsonKey(name: 'memory_usage_percent')
  final double memoryUsagePercent;
  @JsonKey(name: 'memory_usage_bytes')
  final int memoryUsageBytes;
  @JsonKey(name: 'disk_usage_percent')
  final double diskUsagePercent;

  const ResourceMetrics({
    required this.cpuUsagePercent,
    required this.memoryUsagePercent,
    required this.memoryUsageBytes,
    required this.diskUsagePercent,
  });

  factory ResourceMetrics.fromJson(Map<String, dynamic> json) =>
      _$ResourceMetricsFromJson(json);

  Map<String, dynamic> toJson() => _$ResourceMetricsToJson(this);
}
// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'analytics.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

AnalyticsSummary _$AnalyticsSummaryFromJson(Map<String, dynamic> json) =>
    AnalyticsSummary(
      flags: FlagPerformance.fromJson(json['flags'] as Map<String, dynamic>),
      deployments: DeploymentPerformance.fromJson(
          json['deployments'] as Map<String, dynamic>),
      api: ApiPerformance.fromJson(json['api'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$AnalyticsSummaryToJson(AnalyticsSummary instance) =>
    <String, dynamic>{
      'flags': instance.flags,
      'deployments': instance.deployments,
      'api': instance.api,
    };

FlagPerformance _$FlagPerformanceFromJson(Map<String, dynamic> json) =>
    FlagPerformance(
      totalEvaluations: (json['total_evaluations'] as num).toInt(),
      activeFlags: (json['active_flags'] as num).toInt(),
      cacheHitRate: (json['cache_hit_rate'] as num).toDouble(),
      averageLatencyMs: (json['average_latency_ms'] as num).toDouble(),
    );

Map<String, dynamic> _$FlagPerformanceToJson(FlagPerformance instance) =>
    <String, dynamic>{
      'total_evaluations': instance.totalEvaluations,
      'active_flags': instance.activeFlags,
      'cache_hit_rate': instance.cacheHitRate,
      'average_latency_ms': instance.averageLatencyMs,
    };

DeploymentPerformance _$DeploymentPerformanceFromJson(
        Map<String, dynamic> json) =>
    DeploymentPerformance(
      totalDeployments: (json['total_deployments'] as num).toInt(),
      successRate: (json['success_rate'] as num).toDouble(),
      averageDurationMinutes:
          (json['average_duration_minutes'] as num).toDouble(),
      failedDeployments: (json['failed_deployments'] as num).toInt(),
    );

Map<String, dynamic> _$DeploymentPerformanceToJson(
        DeploymentPerformance instance) =>
    <String, dynamic>{
      'total_deployments': instance.totalDeployments,
      'success_rate': instance.successRate,
      'average_duration_minutes': instance.averageDurationMinutes,
      'failed_deployments': instance.failedDeployments,
    };

ApiPerformance _$ApiPerformanceFromJson(Map<String, dynamic> json) =>
    ApiPerformance(
      totalRequests: (json['total_requests'] as num).toInt(),
      averageLatencyMs: (json['average_latency_ms'] as num).toDouble(),
      errorRate: (json['error_rate'] as num).toDouble(),
      requestsPerSecond: (json['requests_per_second'] as num).toDouble(),
    );

Map<String, dynamic> _$ApiPerformanceToJson(ApiPerformance instance) =>
    <String, dynamic>{
      'total_requests': instance.totalRequests,
      'average_latency_ms': instance.averageLatencyMs,
      'error_rate': instance.errorRate,
      'requests_per_second': instance.requestsPerSecond,
    };

FlagStats _$FlagStatsFromJson(Map<String, dynamic> json) => FlagStats(
      flagKey: json['flag_key'] as String,
      totalEvaluations: (json['total_evaluations'] as num).toInt(),
      cacheHitRate: (json['cache_hit_rate'] as num).toDouble(),
      averageLatencyMs: (json['average_latency_ms'] as num).toDouble(),
      errorRate: (json['error_rate'] as num).toDouble(),
      resultDistribution:
          Map<String, int>.from(json['result_distribution'] as Map),
    );

Map<String, dynamic> _$FlagStatsToJson(FlagStats instance) => <String, dynamic>{
      'flag_key': instance.flagKey,
      'total_evaluations': instance.totalEvaluations,
      'cache_hit_rate': instance.cacheHitRate,
      'average_latency_ms': instance.averageLatencyMs,
      'error_rate': instance.errorRate,
      'result_distribution': instance.resultDistribution,
    };

SystemHealth _$SystemHealthFromJson(Map<String, dynamic> json) => SystemHealth(
      api: ApiHealthMetrics.fromJson(json['api'] as Map<String, dynamic>),
      database: DatabaseHealthMetrics.fromJson(
          json['database'] as Map<String, dynamic>),
      resources:
          ResourceMetrics.fromJson(json['resources'] as Map<String, dynamic>),
      timestamp: json['timestamp'] as String,
    );

Map<String, dynamic> _$SystemHealthToJson(SystemHealth instance) =>
    <String, dynamic>{
      'api': instance.api,
      'database': instance.database,
      'resources': instance.resources,
      'timestamp': instance.timestamp,
    };

ApiHealthMetrics _$ApiHealthMetricsFromJson(Map<String, dynamic> json) =>
    ApiHealthMetrics(
      requestsPerSecond: (json['requests_per_second'] as num).toDouble(),
      avgLatencyMs: (json['avg_latency_ms'] as num).toDouble(),
      errorRate: (json['error_rate'] as num).toDouble(),
      activeConnections: (json['active_connections'] as num).toInt(),
    );

Map<String, dynamic> _$ApiHealthMetricsToJson(ApiHealthMetrics instance) =>
    <String, dynamic>{
      'requests_per_second': instance.requestsPerSecond,
      'avg_latency_ms': instance.avgLatencyMs,
      'error_rate': instance.errorRate,
      'active_connections': instance.activeConnections,
    };

DatabaseHealthMetrics _$DatabaseHealthMetricsFromJson(
        Map<String, dynamic> json) =>
    DatabaseHealthMetrics(
      connections: (json['connections'] as num).toInt(),
      queryLatencyMs: (json['query_latency_ms'] as num).toDouble(),
      cacheHitRate: (json['cache_hit_rate'] as num).toDouble(),
    );

Map<String, dynamic> _$DatabaseHealthMetricsToJson(
        DatabaseHealthMetrics instance) =>
    <String, dynamic>{
      'connections': instance.connections,
      'query_latency_ms': instance.queryLatencyMs,
      'cache_hit_rate': instance.cacheHitRate,
    };

ResourceMetrics _$ResourceMetricsFromJson(Map<String, dynamic> json) =>
    ResourceMetrics(
      cpuUsagePercent: (json['cpu_usage_percent'] as num).toDouble(),
      memoryUsagePercent: (json['memory_usage_percent'] as num).toDouble(),
      memoryUsageBytes: (json['memory_usage_bytes'] as num).toInt(),
      diskUsagePercent: (json['disk_usage_percent'] as num).toDouble(),
    );

Map<String, dynamic> _$ResourceMetricsToJson(ResourceMetrics instance) =>
    <String, dynamic>{
      'cpu_usage_percent': instance.cpuUsagePercent,
      'memory_usage_percent': instance.memoryUsagePercent,
      'memory_usage_bytes': instance.memoryUsageBytes,
      'disk_usage_percent': instance.diskUsagePercent,
    };

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'deployment.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

Deployment _$DeploymentFromJson(Map<String, dynamic> json) => Deployment(
      id: json['id'] as String,
      projectId: json['project_id'] as String,
      environmentId: json['environment_id'] as String,
      releaseId: json['release_id'] as String,
      version: json['version'] as String,
      strategy: $enumDecode(_$DeployStrategyEnumMap, json['strategy']),
      status: $enumDecode(_$DeployStatusEnumMap, json['status']),
      targetPercentage: (json['target_percentage'] as num?)?.toDouble(),
      currentPercentage: (json['current_percentage'] as num?)?.toDouble(),
      healthChecks: json['health_checks'] as Map<String, dynamic>?,
      rollbackVersion: json['rollback_version'] as String?,
      notes: json['notes'] as String?,
      createdBy: json['created_by'] as String,
      createdAt: json['created_at'] as String,
      updatedAt: json['updated_at'] as String,
      startedAt: json['started_at'] as String?,
      completedAt: json['completed_at'] as String?,
    );

Map<String, dynamic> _$DeploymentToJson(Deployment instance) =>
    <String, dynamic>{
      'id': instance.id,
      'project_id': instance.projectId,
      'environment_id': instance.environmentId,
      'release_id': instance.releaseId,
      'version': instance.version,
      'strategy': _$DeployStrategyEnumMap[instance.strategy]!,
      'status': _$DeployStatusEnumMap[instance.status]!,
      'target_percentage': instance.targetPercentage,
      'current_percentage': instance.currentPercentage,
      'health_checks': instance.healthChecks,
      'rollback_version': instance.rollbackVersion,
      'notes': instance.notes,
      'created_by': instance.createdBy,
      'created_at': instance.createdAt,
      'updated_at': instance.updatedAt,
      'started_at': instance.startedAt,
      'completed_at': instance.completedAt,
    };

const _$DeployStrategyEnumMap = {
  DeployStrategy.canary: 'canary',
  DeployStrategy.blueGreen: 'blueGreen',
  DeployStrategy.rolling: 'rolling',
};

const _$DeployStatusEnumMap = {
  DeployStatus.pending: 'pending',
  DeployStatus.running: 'running',
  DeployStatus.paused: 'paused',
  DeployStatus.completed: 'completed',
  DeployStatus.failed: 'failed',
  DeployStatus.rolledBack: 'rolledBack',
};

CreateDeploymentRequest _$CreateDeploymentRequestFromJson(
        Map<String, dynamic> json) =>
    CreateDeploymentRequest(
      projectId: json['project_id'] as String,
      environmentId: json['environment_id'] as String,
      releaseId: json['release_id'] as String,
      strategy: $enumDecode(_$DeployStrategyEnumMap, json['strategy']),
      targetPercentage: (json['target_percentage'] as num?)?.toDouble(),
      notes: json['notes'] as String?,
    );

Map<String, dynamic> _$CreateDeploymentRequestToJson(
        CreateDeploymentRequest instance) =>
    <String, dynamic>{
      'project_id': instance.projectId,
      'environment_id': instance.environmentId,
      'release_id': instance.releaseId,
      'strategy': _$DeployStrategyEnumMap[instance.strategy]!,
      'target_percentage': instance.targetPercentage,
      'notes': instance.notes,
    };

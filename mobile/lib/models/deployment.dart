import 'package:json_annotation/json_annotation.dart';

part 'deployment.g.dart';

enum DeployStrategy { canary, blueGreen, rolling }

enum DeployStatus { pending, running, paused, completed, failed, rolledBack }

@JsonSerializable()
class Deployment {
  final String id;
  @JsonKey(name: 'project_id')
  final String projectId;
  @JsonKey(name: 'environment_id')
  final String environmentId;
  @JsonKey(name: 'release_id')
  final String releaseId;
  final String version;
  final DeployStrategy strategy;
  final DeployStatus status;
  @JsonKey(name: 'target_percentage')
  final double? targetPercentage;
  @JsonKey(name: 'current_percentage')
  final double? currentPercentage;
  @JsonKey(name: 'health_checks')
  final Map<String, dynamic>? healthChecks;
  @JsonKey(name: 'rollback_version')
  final String? rollbackVersion;
  final String? notes;
  @JsonKey(name: 'created_by')
  final String createdBy;
  @JsonKey(name: 'created_at')
  final String createdAt;
  @JsonKey(name: 'updated_at')
  final String updatedAt;
  @JsonKey(name: 'started_at')
  final String? startedAt;
  @JsonKey(name: 'completed_at')
  final String? completedAt;

  const Deployment({
    required this.id,
    required this.projectId,
    required this.environmentId,
    required this.releaseId,
    required this.version,
    required this.strategy,
    required this.status,
    this.targetPercentage,
    this.currentPercentage,
    this.healthChecks,
    this.rollbackVersion,
    this.notes,
    required this.createdBy,
    required this.createdAt,
    required this.updatedAt,
    this.startedAt,
    this.completedAt,
  });

  factory Deployment.fromJson(Map<String, dynamic> json) =>
      _$DeploymentFromJson(json);

  Map<String, dynamic> toJson() => _$DeploymentToJson(this);

  Deployment copyWith({
    String? id,
    String? projectId,
    String? environmentId,
    String? releaseId,
    String? version,
    DeployStrategy? strategy,
    DeployStatus? status,
    double? targetPercentage,
    double? currentPercentage,
    Map<String, dynamic>? healthChecks,
    String? rollbackVersion,
    String? notes,
    String? createdBy,
    String? createdAt,
    String? updatedAt,
    String? startedAt,
    String? completedAt,
  }) {
    return Deployment(
      id: id ?? this.id,
      projectId: projectId ?? this.projectId,
      environmentId: environmentId ?? this.environmentId,
      releaseId: releaseId ?? this.releaseId,
      version: version ?? this.version,
      strategy: strategy ?? this.strategy,
      status: status ?? this.status,
      targetPercentage: targetPercentage ?? this.targetPercentage,
      currentPercentage: currentPercentage ?? this.currentPercentage,
      healthChecks: healthChecks ?? this.healthChecks,
      rollbackVersion: rollbackVersion ?? this.rollbackVersion,
      notes: notes ?? this.notes,
      createdBy: createdBy ?? this.createdBy,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      startedAt: startedAt ?? this.startedAt,
      completedAt: completedAt ?? this.completedAt,
    );
  }
}

@JsonSerializable()
class CreateDeploymentRequest {
  @JsonKey(name: 'project_id')
  final String projectId;
  @JsonKey(name: 'environment_id')
  final String environmentId;
  @JsonKey(name: 'release_id')
  final String releaseId;
  final DeployStrategy strategy;
  @JsonKey(name: 'target_percentage')
  final double? targetPercentage;
  final String? notes;

  const CreateDeploymentRequest({
    required this.projectId,
    required this.environmentId,
    required this.releaseId,
    required this.strategy,
    this.targetPercentage,
    this.notes,
  });

  factory CreateDeploymentRequest.fromJson(Map<String, dynamic> json) =>
      _$CreateDeploymentRequestFromJson(json);

  Map<String, dynamic> toJson() => _$CreateDeploymentRequestToJson(this);
}
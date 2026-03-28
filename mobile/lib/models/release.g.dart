// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'release.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

Release _$ReleaseFromJson(Map<String, dynamic> json) => Release(
      id: json['id'] as String,
      projectId: json['project_id'] as String,
      version: json['version'] as String,
      description: json['description'] as String?,
      commitSha: json['commit_sha'] as String?,
      status: $enumDecode(_$ReleaseStatusEnumMap, json['status']),
      artifactUrl: json['artifact_url'] as String?,
      changelogUrl: json['changelog_url'] as String?,
      metadata: json['metadata'] as Map<String, dynamic>?,
      tags: (json['tags'] as List<dynamic>).map((e) => e as String).toList(),
      createdBy: json['created_by'] as String,
      createdAt: json['created_at'] as String,
      updatedAt: json['updated_at'] as String,
    );

Map<String, dynamic> _$ReleaseToJson(Release instance) => <String, dynamic>{
      'id': instance.id,
      'project_id': instance.projectId,
      'version': instance.version,
      'description': instance.description,
      'commit_sha': instance.commitSha,
      'status': _$ReleaseStatusEnumMap[instance.status]!,
      'artifact_url': instance.artifactUrl,
      'changelog_url': instance.changelogUrl,
      'metadata': instance.metadata,
      'tags': instance.tags,
      'created_by': instance.createdBy,
      'created_at': instance.createdAt,
      'updated_at': instance.updatedAt,
    };

const _$ReleaseStatusEnumMap = {
  ReleaseStatus.draft: 'draft',
  ReleaseStatus.staging: 'staging',
  ReleaseStatus.canary: 'canary',
  ReleaseStatus.production: 'production',
  ReleaseStatus.archived: 'archived',
};

CreateReleaseRequest _$CreateReleaseRequestFromJson(
        Map<String, dynamic> json) =>
    CreateReleaseRequest(
      projectId: json['project_id'] as String,
      version: json['version'] as String,
      description: json['description'] as String?,
      commitSha: json['commit_sha'] as String?,
      artifactUrl: json['artifact_url'] as String?,
      changelogUrl: json['changelog_url'] as String?,
      metadata: json['metadata'] as Map<String, dynamic>?,
      tags: (json['tags'] as List<dynamic>?)?.map((e) => e as String).toList(),
    );

Map<String, dynamic> _$CreateReleaseRequestToJson(
        CreateReleaseRequest instance) =>
    <String, dynamic>{
      'project_id': instance.projectId,
      'version': instance.version,
      'description': instance.description,
      'commit_sha': instance.commitSha,
      'artifact_url': instance.artifactUrl,
      'changelog_url': instance.changelogUrl,
      'metadata': instance.metadata,
      'tags': instance.tags,
    };

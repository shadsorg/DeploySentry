import 'package:json_annotation/json_annotation.dart';

part 'release.g.dart';

enum ReleaseStatus { draft, staging, canary, production, archived }

@JsonSerializable()
class Release {
  final String id;
  @JsonKey(name: 'project_id')
  final String projectId;
  final String version;
  final String? description;
  @JsonKey(name: 'commit_sha')
  final String? commitSha;
  final ReleaseStatus status;
  @JsonKey(name: 'artifact_url')
  final String? artifactUrl;
  @JsonKey(name: 'changelog_url')
  final String? changelogUrl;
  final Map<String, dynamic>? metadata;
  final List<String> tags;
  @JsonKey(name: 'created_by')
  final String createdBy;
  @JsonKey(name: 'created_at')
  final String createdAt;
  @JsonKey(name: 'updated_at')
  final String updatedAt;

  const Release({
    required this.id,
    required this.projectId,
    required this.version,
    this.description,
    this.commitSha,
    required this.status,
    this.artifactUrl,
    this.changelogUrl,
    this.metadata,
    required this.tags,
    required this.createdBy,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Release.fromJson(Map<String, dynamic> json) =>
      _$ReleaseFromJson(json);

  Map<String, dynamic> toJson() => _$ReleaseToJson(this);

  Release copyWith({
    String? id,
    String? projectId,
    String? version,
    String? description,
    String? commitSha,
    ReleaseStatus? status,
    String? artifactUrl,
    String? changelogUrl,
    Map<String, dynamic>? metadata,
    List<String>? tags,
    String? createdBy,
    String? createdAt,
    String? updatedAt,
  }) {
    return Release(
      id: id ?? this.id,
      projectId: projectId ?? this.projectId,
      version: version ?? this.version,
      description: description ?? this.description,
      commitSha: commitSha ?? this.commitSha,
      status: status ?? this.status,
      artifactUrl: artifactUrl ?? this.artifactUrl,
      changelogUrl: changelogUrl ?? this.changelogUrl,
      metadata: metadata ?? this.metadata,
      tags: tags ?? this.tags,
      createdBy: createdBy ?? this.createdBy,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }
}

@JsonSerializable()
class CreateReleaseRequest {
  @JsonKey(name: 'project_id')
  final String projectId;
  final String version;
  final String? description;
  @JsonKey(name: 'commit_sha')
  final String? commitSha;
  @JsonKey(name: 'artifact_url')
  final String? artifactUrl;
  @JsonKey(name: 'changelog_url')
  final String? changelogUrl;
  final Map<String, dynamic>? metadata;
  final List<String>? tags;

  const CreateReleaseRequest({
    required this.projectId,
    required this.version,
    this.description,
    this.commitSha,
    this.artifactUrl,
    this.changelogUrl,
    this.metadata,
    this.tags,
  });

  factory CreateReleaseRequest.fromJson(Map<String, dynamic> json) =>
      _$CreateReleaseRequestFromJson(json);

  Map<String, dynamic> toJson() => _$CreateReleaseRequestToJson(this);
}
import 'package:json_annotation/json_annotation.dart';

part 'flag.g.dart';

enum FlagCategory { release, feature, experiment, ops, permission }

enum FlagType { boolean, string, integer, json }

enum RuleType { percentage, user_target, attribute, segment, schedule }

@JsonSerializable()
class FlagMetadata {
  final FlagCategory category;
  final String purpose;
  final List<String> owners;
  @JsonKey(name: 'is_permanent')
  final bool isPermanent;
  @JsonKey(name: 'expires_at')
  final String? expiresAt;
  final List<String> tags;

  const FlagMetadata({
    required this.category,
    required this.purpose,
    required this.owners,
    required this.isPermanent,
    this.expiresAt,
    required this.tags,
  });

  factory FlagMetadata.fromJson(Map<String, dynamic> json) =>
      _$FlagMetadataFromJson(json);

  Map<String, dynamic> toJson() => _$FlagMetadataToJson(this);
}

@JsonSerializable()
class Flag {
  final String id;
  @JsonKey(name: 'project_id')
  final String projectId;
  @JsonKey(name: 'environment_id')
  final String environmentId;
  final String key;
  final String name;
  final String description;
  @JsonKey(name: 'flag_type')
  final FlagType flagType;
  final FlagCategory category;
  final String purpose;
  final List<String> owners;
  @JsonKey(name: 'is_permanent')
  final bool isPermanent;
  @JsonKey(name: 'expires_at')
  final String? expiresAt;
  final bool enabled;
  @JsonKey(name: 'default_value')
  final String defaultValue;
  final bool archived;
  final List<String> tags;
  @JsonKey(name: 'created_by')
  final String createdBy;
  @JsonKey(name: 'created_at')
  final String createdAt;
  @JsonKey(name: 'updated_at')
  final String updatedAt;

  const Flag({
    required this.id,
    required this.projectId,
    required this.environmentId,
    required this.key,
    required this.name,
    required this.description,
    required this.flagType,
    required this.category,
    required this.purpose,
    required this.owners,
    required this.isPermanent,
    this.expiresAt,
    required this.enabled,
    required this.defaultValue,
    required this.archived,
    required this.tags,
    required this.createdBy,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Flag.fromJson(Map<String, dynamic> json) => _$FlagFromJson(json);

  Map<String, dynamic> toJson() => _$FlagToJson(this);

  Flag copyWith({
    String? id,
    String? projectId,
    String? environmentId,
    String? key,
    String? name,
    String? description,
    FlagType? flagType,
    FlagCategory? category,
    String? purpose,
    List<String>? owners,
    bool? isPermanent,
    String? expiresAt,
    bool? enabled,
    String? defaultValue,
    bool? archived,
    List<String>? tags,
    String? createdBy,
    String? createdAt,
    String? updatedAt,
  }) {
    return Flag(
      id: id ?? this.id,
      projectId: projectId ?? this.projectId,
      environmentId: environmentId ?? this.environmentId,
      key: key ?? this.key,
      name: name ?? this.name,
      description: description ?? this.description,
      flagType: flagType ?? this.flagType,
      category: category ?? this.category,
      purpose: purpose ?? this.purpose,
      owners: owners ?? this.owners,
      isPermanent: isPermanent ?? this.isPermanent,
      expiresAt: expiresAt ?? this.expiresAt,
      enabled: enabled ?? this.enabled,
      defaultValue: defaultValue ?? this.defaultValue,
      archived: archived ?? this.archived,
      tags: tags ?? this.tags,
      createdBy: createdBy ?? this.createdBy,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }
}

@JsonSerializable()
class TargetingRule {
  final String id;
  @JsonKey(name: 'flag_id')
  final String flagId;
  @JsonKey(name: 'rule_type')
  final RuleType ruleType;
  final int priority;
  final String value;
  final double? percentage;
  final String? attribute;
  final String? operator;
  @JsonKey(name: 'target_values')
  final List<String>? targetValues;
  @JsonKey(name: 'segment_id')
  final String? segmentId;
  @JsonKey(name: 'start_time')
  final String? startTime;
  @JsonKey(name: 'end_time')
  final String? endTime;

  const TargetingRule({
    required this.id,
    required this.flagId,
    required this.ruleType,
    required this.priority,
    required this.value,
    this.percentage,
    this.attribute,
    this.operator,
    this.targetValues,
    this.segmentId,
    this.startTime,
    this.endTime,
  });

  factory TargetingRule.fromJson(Map<String, dynamic> json) =>
      _$TargetingRuleFromJson(json);

  Map<String, dynamic> toJson() => _$TargetingRuleToJson(this);
}

@JsonSerializable()
class CreateFlagRequest {
  @JsonKey(name: 'project_id')
  final String projectId;
  @JsonKey(name: 'environment_id')
  final String environmentId;
  final String key;
  final String name;
  final String description;
  @JsonKey(name: 'flag_type')
  final FlagType flagType;
  final FlagCategory category;
  final String purpose;
  final List<String> owners;
  @JsonKey(name: 'is_permanent')
  final bool? isPermanent;
  @JsonKey(name: 'expires_at')
  final String? expiresAt;
  @JsonKey(name: 'default_value')
  final String defaultValue;
  final List<String>? tags;

  const CreateFlagRequest({
    required this.projectId,
    required this.environmentId,
    required this.key,
    required this.name,
    required this.description,
    required this.flagType,
    required this.category,
    required this.purpose,
    required this.owners,
    this.isPermanent,
    this.expiresAt,
    required this.defaultValue,
    this.tags,
  });

  factory CreateFlagRequest.fromJson(Map<String, dynamic> json) =>
      _$CreateFlagRequestFromJson(json);

  Map<String, dynamic> toJson() => _$CreateFlagRequestToJson(this);
}

@JsonSerializable()
class UpdateFlagRequest {
  final String? name;
  final String? description;
  final String? purpose;
  final List<String>? owners;
  @JsonKey(name: 'is_permanent')
  final bool? isPermanent;
  @JsonKey(name: 'expires_at')
  final String? expiresAt;
  @JsonKey(name: 'default_value')
  final String? defaultValue;
  final List<String>? tags;

  const UpdateFlagRequest({
    this.name,
    this.description,
    this.purpose,
    this.owners,
    this.isPermanent,
    this.expiresAt,
    this.defaultValue,
    this.tags,
  });

  factory UpdateFlagRequest.fromJson(Map<String, dynamic> json) =>
      _$UpdateFlagRequestFromJson(json);

  Map<String, dynamic> toJson() => _$UpdateFlagRequestToJson(this);
}
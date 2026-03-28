// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'flag.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

FlagMetadata _$FlagMetadataFromJson(Map<String, dynamic> json) => FlagMetadata(
      category: $enumDecode(_$FlagCategoryEnumMap, json['category']),
      purpose: json['purpose'] as String,
      owners:
          (json['owners'] as List<dynamic>).map((e) => e as String).toList(),
      isPermanent: json['is_permanent'] as bool,
      expiresAt: json['expires_at'] as String?,
      tags: (json['tags'] as List<dynamic>).map((e) => e as String).toList(),
    );

Map<String, dynamic> _$FlagMetadataToJson(FlagMetadata instance) =>
    <String, dynamic>{
      'category': _$FlagCategoryEnumMap[instance.category]!,
      'purpose': instance.purpose,
      'owners': instance.owners,
      'is_permanent': instance.isPermanent,
      'expires_at': instance.expiresAt,
      'tags': instance.tags,
    };

const _$FlagCategoryEnumMap = {
  FlagCategory.release: 'release',
  FlagCategory.feature: 'feature',
  FlagCategory.experiment: 'experiment',
  FlagCategory.ops: 'ops',
  FlagCategory.permission: 'permission',
};

Flag _$FlagFromJson(Map<String, dynamic> json) => Flag(
      id: json['id'] as String,
      projectId: json['project_id'] as String,
      environmentId: json['environment_id'] as String,
      key: json['key'] as String,
      name: json['name'] as String,
      description: json['description'] as String,
      flagType: $enumDecode(_$FlagTypeEnumMap, json['flag_type']),
      category: $enumDecode(_$FlagCategoryEnumMap, json['category']),
      purpose: json['purpose'] as String,
      owners:
          (json['owners'] as List<dynamic>).map((e) => e as String).toList(),
      isPermanent: json['is_permanent'] as bool,
      expiresAt: json['expires_at'] as String?,
      enabled: json['enabled'] as bool,
      defaultValue: json['default_value'] as String,
      archived: json['archived'] as bool,
      tags: (json['tags'] as List<dynamic>).map((e) => e as String).toList(),
      createdBy: json['created_by'] as String,
      createdAt: json['created_at'] as String,
      updatedAt: json['updated_at'] as String,
    );

Map<String, dynamic> _$FlagToJson(Flag instance) => <String, dynamic>{
      'id': instance.id,
      'project_id': instance.projectId,
      'environment_id': instance.environmentId,
      'key': instance.key,
      'name': instance.name,
      'description': instance.description,
      'flag_type': _$FlagTypeEnumMap[instance.flagType]!,
      'category': _$FlagCategoryEnumMap[instance.category]!,
      'purpose': instance.purpose,
      'owners': instance.owners,
      'is_permanent': instance.isPermanent,
      'expires_at': instance.expiresAt,
      'enabled': instance.enabled,
      'default_value': instance.defaultValue,
      'archived': instance.archived,
      'tags': instance.tags,
      'created_by': instance.createdBy,
      'created_at': instance.createdAt,
      'updated_at': instance.updatedAt,
    };

const _$FlagTypeEnumMap = {
  FlagType.boolean: 'boolean',
  FlagType.string: 'string',
  FlagType.integer: 'integer',
  FlagType.json: 'json',
};

TargetingRule _$TargetingRuleFromJson(Map<String, dynamic> json) =>
    TargetingRule(
      id: json['id'] as String,
      flagId: json['flag_id'] as String,
      ruleType: $enumDecode(_$RuleTypeEnumMap, json['rule_type']),
      priority: (json['priority'] as num).toInt(),
      value: json['value'] as String,
      percentage: (json['percentage'] as num?)?.toDouble(),
      attribute: json['attribute'] as String?,
      operator: json['operator'] as String?,
      targetValues: (json['target_values'] as List<dynamic>?)
          ?.map((e) => e as String)
          .toList(),
      segmentId: json['segment_id'] as String?,
      startTime: json['start_time'] as String?,
      endTime: json['end_time'] as String?,
    );

Map<String, dynamic> _$TargetingRuleToJson(TargetingRule instance) =>
    <String, dynamic>{
      'id': instance.id,
      'flag_id': instance.flagId,
      'rule_type': _$RuleTypeEnumMap[instance.ruleType]!,
      'priority': instance.priority,
      'value': instance.value,
      'percentage': instance.percentage,
      'attribute': instance.attribute,
      'operator': instance.operator,
      'target_values': instance.targetValues,
      'segment_id': instance.segmentId,
      'start_time': instance.startTime,
      'end_time': instance.endTime,
    };

const _$RuleTypeEnumMap = {
  RuleType.percentage: 'percentage',
  RuleType.user_target: 'user_target',
  RuleType.attribute: 'attribute',
  RuleType.segment: 'segment',
  RuleType.schedule: 'schedule',
};

CreateFlagRequest _$CreateFlagRequestFromJson(Map<String, dynamic> json) =>
    CreateFlagRequest(
      projectId: json['project_id'] as String,
      environmentId: json['environment_id'] as String,
      key: json['key'] as String,
      name: json['name'] as String,
      description: json['description'] as String,
      flagType: $enumDecode(_$FlagTypeEnumMap, json['flag_type']),
      category: $enumDecode(_$FlagCategoryEnumMap, json['category']),
      purpose: json['purpose'] as String,
      owners:
          (json['owners'] as List<dynamic>).map((e) => e as String).toList(),
      isPermanent: json['is_permanent'] as bool?,
      expiresAt: json['expires_at'] as String?,
      defaultValue: json['default_value'] as String,
      tags: (json['tags'] as List<dynamic>?)?.map((e) => e as String).toList(),
    );

Map<String, dynamic> _$CreateFlagRequestToJson(CreateFlagRequest instance) =>
    <String, dynamic>{
      'project_id': instance.projectId,
      'environment_id': instance.environmentId,
      'key': instance.key,
      'name': instance.name,
      'description': instance.description,
      'flag_type': _$FlagTypeEnumMap[instance.flagType]!,
      'category': _$FlagCategoryEnumMap[instance.category]!,
      'purpose': instance.purpose,
      'owners': instance.owners,
      'is_permanent': instance.isPermanent,
      'expires_at': instance.expiresAt,
      'default_value': instance.defaultValue,
      'tags': instance.tags,
    };

UpdateFlagRequest _$UpdateFlagRequestFromJson(Map<String, dynamic> json) =>
    UpdateFlagRequest(
      name: json['name'] as String?,
      description: json['description'] as String?,
      purpose: json['purpose'] as String?,
      owners:
          (json['owners'] as List<dynamic>?)?.map((e) => e as String).toList(),
      isPermanent: json['is_permanent'] as bool?,
      expiresAt: json['expires_at'] as String?,
      defaultValue: json['default_value'] as String?,
      tags: (json['tags'] as List<dynamic>?)?.map((e) => e as String).toList(),
    );

Map<String, dynamic> _$UpdateFlagRequestToJson(UpdateFlagRequest instance) =>
    <String, dynamic>{
      'name': instance.name,
      'description': instance.description,
      'purpose': instance.purpose,
      'owners': instance.owners,
      'is_permanent': instance.isPermanent,
      'expires_at': instance.expiresAt,
      'default_value': instance.defaultValue,
      'tags': instance.tags,
    };

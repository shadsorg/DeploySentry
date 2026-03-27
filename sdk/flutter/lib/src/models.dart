import 'dart:convert';

/// Categories for feature flags.
enum FlagCategory {
  release,
  feature,
  experiment,
  ops,
  permission;

  static FlagCategory? fromString(String value) {
    for (final category in FlagCategory.values) {
      if (category.name == value) return category;
    }
    return null;
  }
}

/// Metadata associated with a feature flag.
class FlagMetadata {
  final FlagCategory? category;
  final String? purpose;
  final List<String> owners;
  final bool isPermanent;
  final DateTime? expiresAt;
  final List<String> tags;

  const FlagMetadata({
    this.category,
    this.purpose,
    this.owners = const [],
    this.isPermanent = false,
    this.expiresAt,
    this.tags = const [],
  });

  factory FlagMetadata.fromJson(Map<String, dynamic> json) {
    return FlagMetadata(
      category: json['category'] != null
          ? FlagCategory.fromString(json['category'] as String)
          : null,
      purpose: json['purpose'] as String?,
      owners: (json['owners'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          const [],
      isPermanent: json['is_permanent'] as bool? ?? false,
      expiresAt: json['expires_at'] != null
          ? DateTime.parse(json['expires_at'] as String)
          : null,
      tags: (json['tags'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          const [],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      if (category != null) 'category': category!.name,
      if (purpose != null) 'purpose': purpose,
      'owners': owners,
      'is_permanent': isPermanent,
      if (expiresAt != null) 'expires_at': expiresAt!.toIso8601String(),
      'tags': tags,
    };
  }

  /// Whether the flag has passed its expiration date.
  bool get isExpired {
    if (expiresAt == null) return false;
    return DateTime.now().isAfter(expiresAt!);
  }
}

/// A feature flag with its current value and metadata.
class Flag {
  final String key;
  final dynamic value;
  final String valueType;
  final bool enabled;
  final FlagMetadata metadata;

  const Flag({
    required this.key,
    required this.value,
    required this.valueType,
    this.enabled = true,
    this.metadata = const FlagMetadata(),
  });

  factory Flag.fromJson(Map<String, dynamic> json) {
    return Flag(
      key: json['key'] as String,
      value: json['value'],
      valueType: json['value_type'] as String? ?? 'boolean',
      enabled: json['enabled'] as bool? ?? true,
      metadata: json['metadata'] != null
          ? FlagMetadata.fromJson(json['metadata'] as Map<String, dynamic>)
          : const FlagMetadata(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'key': key,
      'value': value,
      'value_type': valueType,
      'enabled': enabled,
      'metadata': metadata.toJson(),
    };
  }
}

/// Context provided for flag evaluation, enabling targeting rules.
class EvaluationContext {
  final String? userId;
  final String? orgId;
  final Map<String, dynamic> attributes;

  const EvaluationContext({
    this.userId,
    this.orgId,
    this.attributes = const {},
  });

  Map<String, dynamic> toJson() {
    return {
      if (userId != null) 'user_id': userId,
      if (orgId != null) 'org_id': orgId,
      if (attributes.isNotEmpty) 'attributes': attributes,
    };
  }
}

/// The result of evaluating a feature flag, including the resolved value,
/// the reason for the resolution, and full metadata.
class EvaluationResult {
  final String key;
  final dynamic value;
  final String valueType;
  final String reason;
  final FlagMetadata metadata;
  final bool enabled;

  const EvaluationResult({
    required this.key,
    required this.value,
    required this.valueType,
    this.reason = 'DEFAULT',
    this.metadata = const FlagMetadata(),
    this.enabled = true,
  });

  factory EvaluationResult.fromJson(Map<String, dynamic> json) {
    return EvaluationResult(
      key: json['key'] as String,
      value: json['value'],
      valueType: json['value_type'] as String? ?? 'boolean',
      reason: json['reason'] as String? ?? 'DEFAULT',
      metadata: json['metadata'] != null
          ? FlagMetadata.fromJson(json['metadata'] as Map<String, dynamic>)
          : const FlagMetadata(),
      enabled: json['enabled'] as bool? ?? true,
    );
  }

  /// Convenience getter for boolean flag values.
  bool get boolValue => value as bool? ?? false;

  /// Convenience getter for string flag values.
  String get stringValue => value?.toString() ?? '';

  /// Convenience getter for integer flag values.
  int get intValue => value is int ? value as int : int.tryParse(value.toString()) ?? 0;

  /// Convenience getter for JSON (map) flag values.
  Map<String, dynamic> get jsonValue {
    if (value is Map<String, dynamic>) return value as Map<String, dynamic>;
    if (value is String) {
      try {
        return jsonDecode(value as String) as Map<String, dynamic>;
      } catch (_) {
        return {};
      }
    }
    return {};
  }
}

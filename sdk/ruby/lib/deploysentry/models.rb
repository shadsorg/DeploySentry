# frozen_string_literal: true

module DeploySentry
  module FlagCategory
    RELEASE    = "release"
    FEATURE    = "feature"
    EXPERIMENT = "experiment"
    OPS        = "ops"
    PERMISSION = "permission"

    ALL = [RELEASE, FEATURE, EXPERIMENT, OPS, PERMISSION].freeze

    def self.valid?(category)
      ALL.include?(category)
    end
  end

  FlagMetadata = Struct.new(
    :category,
    :purpose,
    :owners,
    :is_permanent,
    :expires_at,
    :tags,
    keyword_init: true
  ) do
    def initialize(category: nil, purpose: nil, owners: [], is_permanent: false, expires_at: nil, tags: [])
      super(
        category: category,
        purpose: purpose,
        owners: Array(owners),
        is_permanent: is_permanent,
        expires_at: expires_at.is_a?(String) ? Time.parse(expires_at) : expires_at,
        tags: Array(tags)
      )
    end

    def expired?
      return false if is_permanent
      return false if expires_at.nil?

      expires_at < Time.now
    end
  end

  Flag = Struct.new(
    :key,
    :value,
    :type,
    :enabled,
    :metadata,
    keyword_init: true
  ) do
    def initialize(key:, value: nil, type: "boolean", enabled: false, metadata: nil)
      super(
        key: key,
        value: value,
        type: type,
        enabled: enabled,
        metadata: metadata.is_a?(Hash) ? FlagMetadata.new(**metadata) : metadata
      )
    end

    def bool_value
      case value
      when true, false then value
      when "true" then true
      when "false" then false
      else !!value
      end
    end
  end

  EvaluationContext = Struct.new(
    :user_id,
    :org_id,
    :attributes,
    keyword_init: true
  ) do
    def initialize(user_id: nil, org_id: nil, attributes: {})
      super(user_id: user_id, org_id: org_id, attributes: attributes || {})
    end

    def to_h
      hash = {}
      hash[:user_id] = user_id if user_id
      hash[:org_id] = org_id if org_id
      hash[:attributes] = attributes unless attributes.empty?
      hash
    end
  end

  EvaluationResult = Struct.new(
    :key,
    :value,
    :type,
    :reason,
    :flag,
    :metadata,
    :error,
    keyword_init: true
  ) do
    def initialize(key:, value: nil, type: nil, reason: nil, flag: nil, metadata: nil, error: nil)
      super
    end

    def success?
      error.nil?
    end

    def default?
      reason == "DEFAULT"
    end
  end
end

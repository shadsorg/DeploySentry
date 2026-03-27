package io.deploysentry;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;
import java.util.Objects;

/**
 * Contextual information supplied when evaluating a flag, such as the current
 * user, organization, and arbitrary attributes used for targeting rules.
 */
public final class EvaluationContext {

    private final String userId;
    private final String orgId;
    private final Map<String, Object> attributes;

    private EvaluationContext(Builder builder) {
        this.userId = builder.userId;
        this.orgId = builder.orgId;
        this.attributes = Collections.unmodifiableMap(new HashMap<>(builder.attributes));
    }

    /** The user identifier for targeting. */
    public String getUserId() {
        return userId;
    }

    /** The organization identifier for targeting. */
    public String getOrgId() {
        return orgId;
    }

    /** Additional attributes used by targeting rules. */
    public Map<String, Object> getAttributes() {
        return attributes;
    }

    /**
     * Returns the value of a single attribute, or {@code null} if not present.
     */
    public Object getAttribute(String key) {
        return attributes.get(key);
    }

    /** Serializes this context into a map suitable for JSON encoding. */
    public Map<String, Object> toMap() {
        Map<String, Object> map = new HashMap<>();
        if (userId != null) map.put("userId", userId);
        if (orgId != null) map.put("orgId", orgId);
        if (!attributes.isEmpty()) map.put("attributes", attributes);
        return map;
    }

    public static Builder builder() {
        return new Builder();
    }

    @Override
    public String toString() {
        return "EvaluationContext{userId='" + userId +
                "', orgId='" + orgId +
                "', attributes=" + attributes + '}';
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (!(o instanceof EvaluationContext)) return false;
        EvaluationContext that = (EvaluationContext) o;
        return Objects.equals(userId, that.userId)
                && Objects.equals(orgId, that.orgId)
                && Objects.equals(attributes, that.attributes);
    }

    @Override
    public int hashCode() {
        return Objects.hash(userId, orgId, attributes);
    }

    public static final class Builder {
        private String userId;
        private String orgId;
        private final Map<String, Object> attributes = new HashMap<>();

        private Builder() {}

        public Builder userId(String userId) {
            this.userId = userId;
            return this;
        }

        public Builder orgId(String orgId) {
            this.orgId = orgId;
            return this;
        }

        /**
         * Adds a single custom attribute.
         */
        public Builder attribute(String key, Object value) {
            this.attributes.put(key, value);
            return this;
        }

        /**
         * Adds all entries from the given map as custom attributes.
         */
        public Builder attributes(Map<String, Object> attributes) {
            this.attributes.putAll(attributes);
            return this;
        }

        public EvaluationContext build() {
            return new EvaluationContext(this);
        }
    }
}

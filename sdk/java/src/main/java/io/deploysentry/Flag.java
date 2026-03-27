package io.deploysentry;

import java.util.Collections;
import java.util.Map;
import java.util.Objects;

/**
 * Represents a single feature flag with its current value and associated metadata.
 */
public final class Flag {

    private final String key;
    private final Object value;
    private final String type;
    private final boolean enabled;
    private final FlagMetadata metadata;
    private final Map<String, Object> variants;

    private Flag(Builder builder) {
        this.key = Objects.requireNonNull(builder.key, "key must not be null");
        this.value = builder.value;
        this.type = builder.type;
        this.enabled = builder.enabled;
        this.metadata = builder.metadata;
        this.variants = builder.variants == null
                ? Collections.emptyMap()
                : Collections.unmodifiableMap(builder.variants);
    }

    /** The unique identifier of this flag. */
    public String getKey() {
        return key;
    }

    /** The resolved value of this flag. */
    public Object getValue() {
        return value;
    }

    /** The value type descriptor (e.g. "boolean", "string", "integer", "json"). */
    public String getType() {
        return type;
    }

    /** Whether this flag is currently enabled. */
    public boolean isEnabled() {
        return enabled;
    }

    /** Rich metadata describing purpose, ownership, and lifecycle. */
    public FlagMetadata getMetadata() {
        return metadata;
    }

    /** Variant definitions, keyed by variant name. */
    public Map<String, Object> getVariants() {
        return variants;
    }

    public static Builder builder() {
        return new Builder();
    }

    @Override
    public String toString() {
        return "Flag{key='" + key + "', value=" + value +
                ", type='" + type + "', enabled=" + enabled + '}';
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (!(o instanceof Flag)) return false;
        Flag flag = (Flag) o;
        return enabled == flag.enabled
                && Objects.equals(key, flag.key)
                && Objects.equals(value, flag.value)
                && Objects.equals(type, flag.type);
    }

    @Override
    public int hashCode() {
        return Objects.hash(key, value, type, enabled);
    }

    public static final class Builder {
        private String key;
        private Object value;
        private String type;
        private boolean enabled;
        private FlagMetadata metadata;
        private Map<String, Object> variants;

        private Builder() {}

        public Builder key(String key) {
            this.key = key;
            return this;
        }

        public Builder value(Object value) {
            this.value = value;
            return this;
        }

        public Builder type(String type) {
            this.type = type;
            return this;
        }

        public Builder enabled(boolean enabled) {
            this.enabled = enabled;
            return this;
        }

        public Builder metadata(FlagMetadata metadata) {
            this.metadata = metadata;
            return this;
        }

        public Builder variants(Map<String, Object> variants) {
            this.variants = variants;
            return this;
        }

        public Flag build() {
            return new Flag(this);
        }
    }
}

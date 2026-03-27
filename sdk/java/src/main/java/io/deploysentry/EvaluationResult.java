package io.deploysentry;

import java.util.Objects;

/**
 * The full result of a flag evaluation, including the resolved value, the
 * matched variant, reason, and attached metadata.
 *
 * @param <T> the value type
 */
public final class EvaluationResult<T> {

    private final String flagKey;
    private final T value;
    private final String variant;
    private final String reason;
    private final boolean defaulted;
    private final FlagMetadata metadata;
    private final String errorCode;

    private EvaluationResult(Builder<T> builder) {
        this.flagKey = builder.flagKey;
        this.value = builder.value;
        this.variant = builder.variant;
        this.reason = builder.reason;
        this.defaulted = builder.defaulted;
        this.metadata = builder.metadata;
        this.errorCode = builder.errorCode;
    }

    /** The key of the evaluated flag. */
    public String getFlagKey() {
        return flagKey;
    }

    /** The resolved value. */
    public T getValue() {
        return value;
    }

    /** The name of the matched variant, if any. */
    public String getVariant() {
        return variant;
    }

    /** A human-readable reason describing how the value was resolved. */
    public String getReason() {
        return reason;
    }

    /** Whether the default value was returned (flag not found or error). */
    public boolean isDefaulted() {
        return defaulted;
    }

    /** Metadata associated with the flag. */
    public FlagMetadata getMetadata() {
        return metadata;
    }

    /** An error code if evaluation failed, otherwise {@code null}. */
    public String getErrorCode() {
        return errorCode;
    }

    public static <T> Builder<T> builder() {
        return new Builder<>();
    }

    @Override
    public String toString() {
        return "EvaluationResult{flagKey='" + flagKey +
                "', value=" + value +
                ", variant='" + variant +
                "', reason='" + reason +
                "', defaulted=" + defaulted +
                ", errorCode='" + errorCode + "'}";
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (!(o instanceof EvaluationResult)) return false;
        EvaluationResult<?> that = (EvaluationResult<?>) o;
        return defaulted == that.defaulted
                && Objects.equals(flagKey, that.flagKey)
                && Objects.equals(value, that.value)
                && Objects.equals(variant, that.variant)
                && Objects.equals(reason, that.reason)
                && Objects.equals(errorCode, that.errorCode);
    }

    @Override
    public int hashCode() {
        return Objects.hash(flagKey, value, variant, reason, defaulted, errorCode);
    }

    public static final class Builder<T> {
        private String flagKey;
        private T value;
        private String variant;
        private String reason;
        private boolean defaulted;
        private FlagMetadata metadata;
        private String errorCode;

        private Builder() {}

        public Builder<T> flagKey(String flagKey) {
            this.flagKey = flagKey;
            return this;
        }

        public Builder<T> value(T value) {
            this.value = value;
            return this;
        }

        public Builder<T> variant(String variant) {
            this.variant = variant;
            return this;
        }

        public Builder<T> reason(String reason) {
            this.reason = reason;
            return this;
        }

        public Builder<T> defaulted(boolean defaulted) {
            this.defaulted = defaulted;
            return this;
        }

        public Builder<T> metadata(FlagMetadata metadata) {
            this.metadata = metadata;
            return this;
        }

        public Builder<T> errorCode(String errorCode) {
            this.errorCode = errorCode;
            return this;
        }

        public EvaluationResult<T> build() {
            return new EvaluationResult<>(this);
        }
    }
}

package io.deploysentry;

/**
 * Categories for feature flags, defining their operational purpose.
 */
public enum FlagCategory {

    /** Flags gating a phased rollout of new releases. */
    RELEASE("release"),

    /** Flags controlling access to product features. */
    FEATURE("feature"),

    /** Flags used for A/B tests and experiments. */
    EXPERIMENT("experiment"),

    /** Flags for operational controls such as circuit breakers. */
    OPS("ops"),

    /** Flags governing user permissions or entitlements. */
    PERMISSION("permission");

    private final String value;

    FlagCategory(String value) {
        this.value = value;
    }

    /** Returns the wire-format value of this category. */
    public String value() {
        return value;
    }

    /**
     * Parses a wire-format string into a {@link FlagCategory}.
     *
     * @param value the string to parse
     * @return the matching category
     * @throws IllegalArgumentException if the value does not match any category
     */
    public static FlagCategory fromValue(String value) {
        for (FlagCategory category : values()) {
            if (category.value.equalsIgnoreCase(value)) {
                return category;
            }
        }
        throw new IllegalArgumentException("Unknown flag category: " + value);
    }
}

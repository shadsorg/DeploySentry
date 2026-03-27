package io.deploysentry;

import java.time.Instant;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.Objects;

/**
 * Rich metadata attached to a feature flag describing its purpose, ownership,
 * lifecycle, and classification.
 */
public final class FlagMetadata {

    private final FlagCategory category;
    private final String purpose;
    private final List<String> owners;
    private final boolean permanent;
    private final Instant expiresAt;
    private final Map<String, String> tags;

    private FlagMetadata(Builder builder) {
        this.category = builder.category;
        this.purpose = builder.purpose;
        this.owners = builder.owners == null
                ? Collections.emptyList()
                : Collections.unmodifiableList(builder.owners);
        this.permanent = builder.permanent;
        this.expiresAt = builder.expiresAt;
        this.tags = builder.tags == null
                ? Collections.emptyMap()
                : Collections.unmodifiableMap(builder.tags);
    }

    /** The operational category of this flag. */
    public FlagCategory getCategory() {
        return category;
    }

    /** Human-readable description of the flag's purpose. */
    public String getPurpose() {
        return purpose;
    }

    /** Team or individual owners responsible for this flag. */
    public List<String> getOwners() {
        return owners;
    }

    /** Whether this flag is intended to remain indefinitely. */
    public boolean isPermanent() {
        return permanent;
    }

    /** Optional expiration timestamp; {@code null} if the flag does not expire. */
    public Instant getExpiresAt() {
        return expiresAt;
    }

    /** Arbitrary key-value tags for additional classification. */
    public Map<String, String> getTags() {
        return tags;
    }

    /** Whether this flag has passed its expiration date. */
    public boolean isExpired() {
        return expiresAt != null && Instant.now().isAfter(expiresAt);
    }

    public static Builder builder() {
        return new Builder();
    }

    @Override
    public String toString() {
        return "FlagMetadata{" +
                "category=" + category +
                ", purpose='" + purpose + '\'' +
                ", owners=" + owners +
                ", permanent=" + permanent +
                ", expiresAt=" + expiresAt +
                ", tags=" + tags +
                '}';
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (!(o instanceof FlagMetadata)) return false;
        FlagMetadata that = (FlagMetadata) o;
        return permanent == that.permanent
                && category == that.category
                && Objects.equals(purpose, that.purpose)
                && Objects.equals(owners, that.owners)
                && Objects.equals(expiresAt, that.expiresAt)
                && Objects.equals(tags, that.tags);
    }

    @Override
    public int hashCode() {
        return Objects.hash(category, purpose, owners, permanent, expiresAt, tags);
    }

    public static final class Builder {
        private FlagCategory category;
        private String purpose;
        private List<String> owners;
        private boolean permanent;
        private Instant expiresAt;
        private Map<String, String> tags;

        private Builder() {}

        public Builder category(FlagCategory category) {
            this.category = category;
            return this;
        }

        public Builder purpose(String purpose) {
            this.purpose = purpose;
            return this;
        }

        public Builder owners(List<String> owners) {
            this.owners = owners;
            return this;
        }

        public Builder permanent(boolean permanent) {
            this.permanent = permanent;
            return this;
        }

        public Builder expiresAt(Instant expiresAt) {
            this.expiresAt = expiresAt;
            return this;
        }

        public Builder tags(Map<String, String> tags) {
            this.tags = tags;
            return this;
        }

        public FlagMetadata build() {
            return new FlagMetadata(this);
        }
    }
}

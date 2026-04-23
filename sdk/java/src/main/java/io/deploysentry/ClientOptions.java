package io.deploysentry;

import java.time.Duration;
import java.util.Collections;
import java.util.Map;
import java.util.Objects;
import java.util.function.Supplier;

/**
 * Configuration options for a {@link DeploySentryClient}.
 * Construct instances via {@link #builder()}.
 */
public final class ClientOptions {

    private static final String DEFAULT_BASE_URL = "https://api.dr-sentry.com";
    private static final Duration DEFAULT_CACHE_TIMEOUT = Duration.ofMinutes(5);
    private static final Duration DEFAULT_STATUS_INTERVAL = Duration.ofSeconds(30);

    private final String apiKey;
    private final String baseURL;
    private final String environment;
    private final String project;
    private final Duration cacheTimeout;
    private final Duration connectTimeout;
    private final boolean enableSSE;
    private final String sessionId;

    // Agentless status reporting
    private final String applicationId;
    private final boolean reportStatus;
    private final Duration reportStatusInterval;
    private final String reportStatusVersion;
    private final String reportStatusCommitSha;
    private final String reportStatusDeploySlot;
    private final Map<String, String> reportStatusTags;
    private final Supplier<HealthReport> healthProvider;

    private ClientOptions(Builder builder) {
        this.apiKey = Objects.requireNonNull(builder.apiKey, "apiKey must not be null");
        this.baseURL = builder.baseURL == null ? DEFAULT_BASE_URL : builder.baseURL;
        this.environment = builder.environment;
        this.project = builder.project;
        this.cacheTimeout = builder.cacheTimeout == null ? DEFAULT_CACHE_TIMEOUT : builder.cacheTimeout;
        this.connectTimeout = builder.connectTimeout == null ? Duration.ofSeconds(10) : builder.connectTimeout;
        this.enableSSE = builder.enableSSE;
        this.sessionId = builder.sessionId;

        this.applicationId = builder.applicationId;
        this.reportStatus = builder.reportStatus;
        this.reportStatusInterval = builder.reportStatusInterval == null
                ? DEFAULT_STATUS_INTERVAL
                : builder.reportStatusInterval;
        this.reportStatusVersion = builder.reportStatusVersion;
        this.reportStatusCommitSha = builder.reportStatusCommitSha;
        this.reportStatusDeploySlot = builder.reportStatusDeploySlot;
        this.reportStatusTags = builder.reportStatusTags == null
                ? Collections.emptyMap()
                : Collections.unmodifiableMap(builder.reportStatusTags);
        this.healthProvider = builder.healthProvider;
    }

    public String getApiKey() {
        return apiKey;
    }

    public String getBaseURL() {
        return baseURL;
    }

    public String getEnvironment() {
        return environment;
    }

    public String getProject() {
        return project;
    }

    public Duration getCacheTimeout() {
        return cacheTimeout;
    }

    public Duration getConnectTimeout() {
        return connectTimeout;
    }

    public boolean isEnableSSE() {
        return enableSSE;
    }

    public String getSessionId() {
        return sessionId;
    }

    public String getApplicationId() {
        return applicationId;
    }

    public boolean isReportStatus() {
        return reportStatus;
    }

    public Duration getReportStatusInterval() {
        return reportStatusInterval;
    }

    public String getReportStatusVersion() {
        return reportStatusVersion;
    }

    public String getReportStatusCommitSha() {
        return reportStatusCommitSha;
    }

    public String getReportStatusDeploySlot() {
        return reportStatusDeploySlot;
    }

    public Map<String, String> getReportStatusTags() {
        return reportStatusTags;
    }

    public Supplier<HealthReport> getHealthProvider() {
        return healthProvider;
    }

    public static Builder builder() {
        return new Builder();
    }

    public static final class Builder {
        private String apiKey;
        private String baseURL;
        private String environment;
        private String project;
        private Duration cacheTimeout;
        private Duration connectTimeout;
        private boolean enableSSE = true;
        private String sessionId;

        private String applicationId;
        private boolean reportStatus = false;
        private Duration reportStatusInterval;
        private String reportStatusVersion;
        private String reportStatusCommitSha;
        private String reportStatusDeploySlot;
        private Map<String, String> reportStatusTags;
        private Supplier<HealthReport> healthProvider;

        private Builder() {}

        public Builder apiKey(String apiKey) {
            this.apiKey = apiKey;
            return this;
        }

        public Builder baseURL(String baseURL) {
            this.baseURL = baseURL;
            return this;
        }

        public Builder environment(String environment) {
            this.environment = environment;
            return this;
        }

        public Builder project(String project) {
            this.project = project;
            return this;
        }

        public Builder cacheTimeout(Duration cacheTimeout) {
            this.cacheTimeout = cacheTimeout;
            return this;
        }

        public Builder connectTimeout(Duration connectTimeout) {
            this.connectTimeout = connectTimeout;
            return this;
        }

        public Builder enableSSE(boolean enableSSE) {
            this.enableSSE = enableSSE;
            return this;
        }

        public Builder sessionId(String sessionId) {
            this.sessionId = sessionId;
            return this;
        }

        public Builder applicationId(String applicationId) {
            this.applicationId = applicationId;
            return this;
        }

        public Builder reportStatus(boolean enabled) {
            this.reportStatus = enabled;
            return this;
        }

        public Builder reportStatusInterval(Duration interval) {
            this.reportStatusInterval = interval;
            return this;
        }

        public Builder reportStatusVersion(String version) {
            this.reportStatusVersion = version;
            return this;
        }

        public Builder reportStatusCommitSha(String sha) {
            this.reportStatusCommitSha = sha;
            return this;
        }

        public Builder reportStatusDeploySlot(String slot) {
            this.reportStatusDeploySlot = slot;
            return this;
        }

        public Builder reportStatusTags(Map<String, String> tags) {
            this.reportStatusTags = tags;
            return this;
        }

        public Builder healthProvider(Supplier<HealthReport> provider) {
            this.healthProvider = provider;
            return this;
        }

        public ClientOptions build() {
            return new ClientOptions(this);
        }
    }
}

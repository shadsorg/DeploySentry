package io.deploysentry;

import java.time.Duration;
import java.util.Objects;

/**
 * Configuration options for a {@link DeploySentryClient}.
 * Construct instances via {@link #builder()}.
 */
public final class ClientOptions {

    private static final String DEFAULT_BASE_URL = "https://api.dr-sentry.com";
    private static final Duration DEFAULT_CACHE_TIMEOUT = Duration.ofMinutes(5);

    private final String apiKey;
    private final String baseURL;
    private final String environment;
    private final String project;
    private final Duration cacheTimeout;
    private final Duration connectTimeout;
    private final boolean enableSSE;
    private final String sessionId;

    private ClientOptions(Builder builder) {
        this.apiKey = Objects.requireNonNull(builder.apiKey, "apiKey must not be null");
        this.baseURL = builder.baseURL == null ? DEFAULT_BASE_URL : builder.baseURL;
        this.environment = builder.environment;
        this.project = builder.project;
        this.cacheTimeout = builder.cacheTimeout == null ? DEFAULT_CACHE_TIMEOUT : builder.cacheTimeout;
        this.connectTimeout = builder.connectTimeout == null ? Duration.ofSeconds(10) : builder.connectTimeout;
        this.enableSSE = builder.enableSSE;
        this.sessionId = builder.sessionId;
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

        public ClientOptions build() {
            return new ClientOptions(this);
        }
    }
}

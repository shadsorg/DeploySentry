package io.deploysentry;

import org.json.JSONArray;
import org.json.JSONObject;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.function.Supplier;
import java.util.logging.Level;
import java.util.logging.Logger;
import java.util.stream.Collectors;

/**
 * Primary entry point for the DeploySentry Java SDK.
 *
 * <p>Fetches feature flags from the DeploySentry API, caches them locally,
 * and optionally subscribes to real-time updates via Server-Sent Events.
 *
 * <pre>{@code
 * ClientOptions options = ClientOptions.builder()
 *         .apiKey("ds_live_abc123")
 *         .environment("production")
 *         .project("my-app")
 *         .build();
 *
 * try (DeploySentryClient client = new DeploySentryClient(options)) {
 *     client.initialize();
 *
 *     boolean enabled = client.boolValue("dark-mode", false,
 *             EvaluationContext.builder().userId("user-42").build());
 * }
 * }</pre>
 */
public final class DeploySentryClient implements AutoCloseable {

    private static final Logger LOG = Logger.getLogger(DeploySentryClient.class.getName());

    private final ClientOptions options;
    private final HttpClient httpClient;
    private final FlagCache cache;
    private final AtomicBoolean initialized = new AtomicBoolean(false);
    private final ConcurrentHashMap<String, List<Registration<?>>> registry = new ConcurrentHashMap<>();

    private SSEClient sseClient;
    private StatusReporter statusReporter;

    /**
     * Creates a new client with the given options. Call {@link #initialize()}
     * to fetch flags and open the SSE stream.
     */
    public DeploySentryClient(ClientOptions options) {
        this.options = options;
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(options.getConnectTimeout())
                .build();
        this.cache = new FlagCache(options.getCacheTimeout());
    }

    // ------------------------------------------------------------------ lifecycle

    /**
     * Fetches the initial set of flags from the API and, when SSE is enabled,
     * opens a streaming connection for real-time updates.
     *
     * @throws RuntimeException if the initial fetch fails
     */
    public void initialize() {
        if (!initialized.compareAndSet(false, true)) {
            return;
        }

        fetchFlags();

        if (options.isEnableSSE()) {
            startSSE();
        }

        startStatusReporter();

        LOG.info("DeploySentry client initialized (flags cached: " + cache.size() + ")");
    }

    private void startStatusReporter() {
        if (!options.isReportStatus()) return;
        if (options.getApplicationId() == null || options.getApplicationId().isEmpty()) {
            LOG.warning("reportStatus=true but applicationId is empty; status reporter disabled");
            return;
        }
        statusReporter = new StatusReporter(options, httpClient);
        statusReporter.start();
    }

    /**
     * Shuts down the SSE connection and releases resources.
     */
    @Override
    public void close() {
        if (statusReporter != null) {
            statusReporter.stop();
            statusReporter = null;
        }
        if (sseClient != null) {
            sseClient.close();
            sseClient = null;
        }
        cache.clear();
        initialized.set(false);
        LOG.info("DeploySentry client closed");
    }

    /**
     * Clears the local flag cache and re-fetches all flags from the API.
     * Useful when a new session starts and fresh flag state is required.
     */
    public void refreshSession() {
        cache.clear();
        fetchFlags();
    }

    // --------------------------------------------------------- typed evaluations

    /**
     * Evaluates a boolean flag.
     *
     * @param key          flag key
     * @param defaultValue fallback if the flag is missing or not a boolean
     * @param context      evaluation context (may be {@code null})
     * @return the resolved boolean value
     */
    public boolean boolValue(String key, boolean defaultValue, EvaluationContext context) {
        Flag flag = resolveFlag(key);
        if (flag == null || flag.getValue() == null) {
            return defaultValue;
        }
        Object val = flag.getValue();
        if (val instanceof Boolean) {
            return (Boolean) val;
        }
        if (val instanceof String) {
            return Boolean.parseBoolean((String) val);
        }
        return defaultValue;
    }

    /**
     * Evaluates a string flag.
     *
     * @param key          flag key
     * @param defaultValue fallback if the flag is missing or not a string
     * @param context      evaluation context (may be {@code null})
     * @return the resolved string value
     */
    public String stringValue(String key, String defaultValue, EvaluationContext context) {
        Flag flag = resolveFlag(key);
        if (flag == null || flag.getValue() == null) {
            return defaultValue;
        }
        Object val = flag.getValue();
        if (val instanceof String) {
            return (String) val;
        }
        return val.toString();
    }

    /**
     * Evaluates an integer flag.
     *
     * @param key          flag key
     * @param defaultValue fallback if the flag is missing or not numeric
     * @param context      evaluation context (may be {@code null})
     * @return the resolved integer value
     */
    public int intValue(String key, int defaultValue, EvaluationContext context) {
        Flag flag = resolveFlag(key);
        if (flag == null || flag.getValue() == null) {
            return defaultValue;
        }
        Object val = flag.getValue();
        if (val instanceof Number) {
            return ((Number) val).intValue();
        }
        if (val instanceof String) {
            try {
                return Integer.parseInt((String) val);
            } catch (NumberFormatException e) {
                return defaultValue;
            }
        }
        return defaultValue;
    }

    /**
     * Evaluates a JSON flag, returning the raw JSON string.
     *
     * @param key          flag key
     * @param defaultValue fallback if the flag is missing
     * @param context      evaluation context (may be {@code null})
     * @return the resolved JSON string
     */
    public String jsonValue(String key, String defaultValue, EvaluationContext context) {
        Flag flag = resolveFlag(key);
        if (flag == null || flag.getValue() == null) {
            return defaultValue;
        }
        Object val = flag.getValue();
        if (val instanceof String) {
            return (String) val;
        }
        if (val instanceof Map || val instanceof List) {
            return new JSONObject(val.toString()).toString();
        }
        return val.toString();
    }

    /**
     * Returns a full {@link EvaluationResult} including value, variant,
     * reason, and metadata.
     *
     * @param key     flag key
     * @param context evaluation context (may be {@code null})
     * @return the detailed evaluation result
     */
    @SuppressWarnings("unchecked")
    public <T> EvaluationResult<T> detail(String key, EvaluationContext context) {
        Flag flag = resolveFlag(key);
        if (flag == null) {
            return (EvaluationResult<T>) EvaluationResult.builder()
                    .flagKey(key)
                    .value(null)
                    .reason("FLAG_NOT_FOUND")
                    .defaulted(true)
                    .errorCode("FLAG_NOT_FOUND")
                    .build();
        }

        return (EvaluationResult<T>) EvaluationResult.builder()
                .flagKey(key)
                .value(flag.getValue())
                .reason(flag.isEnabled() ? "TARGETING_MATCH" : "DISABLED")
                .defaulted(false)
                .metadata(flag.getMetadata())
                .build();
    }

    // --------------------------------------------------------- metadata queries

    /**
     * Returns all cached flags matching the given {@link FlagCategory}.
     */
    public List<Flag> flagsByCategory(FlagCategory category) {
        return cache.getByCategory(category);
    }

    /**
     * Returns all cached flags whose metadata indicates they have passed their
     * expiration date.
     */
    public List<Flag> expiredFlags() {
        return cache.getExpired();
    }

    /**
     * Returns the list of owners for the given flag key, or an empty list if
     * the flag is not found or has no owners.
     */
    public List<String> flagOwners(String key) {
        Flag flag = cache.get(key);
        if (flag == null || flag.getMetadata() == null) {
            return Collections.emptyList();
        }
        return flag.getMetadata().getOwners();
    }

    // --------------------------------------------------- register / dispatch API

    /**
     * Registers a flag-gated handler for the given {@code operation}.
     * When {@code flagKey} is non-null, this handler is selected only when the
     * corresponding flag is enabled in the cache.
     *
     * @param operation the logical operation name (e.g. "send-email")
     * @param handler   the supplier that executes the operation
     * @param flagKey   the feature flag that gates this handler; {@code null}
     *                  means this is the default handler
     * @param <T>       the return type
     */
    public <T> void register(String operation, Supplier<T> handler, String flagKey) {
        registry.compute(operation, (k, list) -> {
            if (list == null) list = new ArrayList<>();
            list.add(new Registration<>(handler, flagKey));
            return list;
        });
    }

    /**
     * Registers a default (unflagged) handler for the given {@code operation}.
     * If a default handler already exists it is replaced; otherwise the handler
     * is appended to the list.
     *
     * @param operation the logical operation name
     * @param handler   the default supplier to execute
     * @param <T>       the return type
     */
    public <T> void register(String operation, Supplier<T> handler) {
        registry.compute(operation, (k, list) -> {
            if (list == null) list = new ArrayList<>();
            for (int i = 0; i < list.size(); i++) {
                if (list.get(i).flagKey == null) {
                    list.set(i, new Registration<>(handler, null));
                    return list;
                }
            }
            list.add(new Registration<>(handler, null));
            return list;
        });
    }

    /**
     * Selects and returns the best-matching handler for the given
     * {@code operation}.
     *
     * <p>Resolution order:
     * <ol>
     *   <li>First registered handler whose flag is present and enabled in the
     *       cache.</li>
     *   <li>The registered default handler (no flag key).</li>
     * </ol>
     *
     * @param operation the logical operation name
     * @param context   the evaluation context (currently unused but reserved for
     *                  future targeting evaluation)
     * @param <T>       the return type of the handler
     * @return the matching {@link Supplier}
     * @throws IllegalStateException if no handlers are registered for the
     *                               operation, or if no flagged handler matches
     *                               and no default has been registered
     */
    @SuppressWarnings("unchecked")
    public <T> Supplier<T> dispatch(String operation, EvaluationContext context) {
        var list = registry.get(operation);
        if (list == null || list.isEmpty()) {
            throw new IllegalStateException(
                "No handlers registered for operation '" + operation + "'. Call register() before dispatch()."
            );
        }
        // First pass: find an enabled flagged handler
        for (var reg : list) {
            if (reg.flagKey != null) {
                var flag = cache.get(reg.flagKey);
                if (flag != null && flag.isEnabled()) {
                    return (Supplier<T>) reg.handler;
                }
            }
        }
        // Second pass: fall back to default handler
        for (var reg : list) {
            if (reg.flagKey == null) {
                return (Supplier<T>) reg.handler;
            }
        }
        throw new IllegalStateException(
            "No matching handler for operation '" + operation + "' and no default registered. " +
            "Register a default handler (no flagKey) as the last registration."
        );
    }

    // --------------------------------------------------------- internal helpers

    private Flag resolveFlag(String key) {
        return cache.get(key);
    }

    private void fetchFlags() {
        String url = buildUrl("/api/v1/flags");

        HttpRequest.Builder reqBuilder = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .header("Authorization", "ApiKey " + options.getApiKey())
                .header("Accept", "application/json")
                .GET();

        if (options.getSessionId() != null) {
            reqBuilder.header("X-DeploySentry-Session", options.getSessionId());
        }

        HttpRequest request = reqBuilder.build();

        try {
            HttpResponse<String> response = httpClient.send(request,
                    HttpResponse.BodyHandlers.ofString());

            if (response.statusCode() != 200) {
                throw new RuntimeException(
                        "Failed to fetch flags: HTTP " + response.statusCode() + " - " + response.body());
            }

            JSONObject body = new JSONObject(response.body());
            JSONArray flagsArray = body.getJSONArray("flags");

            Map<String, Flag> flags = new HashMap<>();
            for (int i = 0; i < flagsArray.length(); i++) {
                Flag flag = parseFlag(flagsArray.getJSONObject(i));
                flags.put(flag.getKey(), flag);
            }

            cache.putAll(flags);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new RuntimeException("Interrupted while fetching flags", e);
        } catch (Exception e) {
            throw new RuntimeException("Failed to fetch flags", e);
        }
    }

    private Flag fetchSingleFlag(String flagId) {
        String url = buildUrl("/api/v1/flags/" + flagId);

        HttpRequest.Builder reqBuilder = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .header("Authorization", "ApiKey " + options.getApiKey())
                .header("Accept", "application/json")
                .GET();

        if (options.getSessionId() != null) {
            reqBuilder.header("X-DeploySentry-Session", options.getSessionId());
        }

        try {
            HttpResponse<String> response = httpClient.send(reqBuilder.build(),
                    HttpResponse.BodyHandlers.ofString());

            if (response.statusCode() != 200) {
                LOG.warning("Failed to fetch flag " + flagId + ": HTTP " + response.statusCode());
                return null;
            }

            JSONObject body = new JSONObject(response.body());
            return parseFlag(body.getJSONObject("flag"));
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            LOG.warning("Interrupted while fetching flag " + flagId);
            return null;
        } catch (Exception e) {
            LOG.log(Level.WARNING, "Failed to fetch flag " + flagId, e);
            return null;
        }
    }

    private void startSSE() {
        String sseUrl = buildUrl("/api/v1/flags/stream");

        sseClient = new SSEClient(
                httpClient,
                URI.create(sseUrl),
                options.getApiKey(),
                options.getSessionId(),
                this::handleSSEEvent,
                error -> LOG.log(Level.WARNING, "SSE error", error)
        );

        sseClient.connect();
    }

    private void handleSSEEvent(String data) {
        try {
            JSONObject event = new JSONObject(data);
            String type = event.optString("type", "");

            switch (type) {
                case "flag.updated":
                case "flag.created":
                case "flag.toggled": {
                    String flagId = event.optString("flag_id", null);
                    if (flagId != null) {
                        Flag flag = fetchSingleFlag(flagId);
                        if (flag != null) {
                            cache.put(flag.getKey(), flag);
                            LOG.fine("Flag updated via SSE: " + flag.getKey());
                        }
                    }
                    break;
                }
                case "flag.deleted": {
                    String key = event.optString("flag_key", event.optString("key", null));
                    if (key != null) {
                        cache.remove(key);
                        LOG.fine("Flag removed via SSE: " + key);
                    }
                    break;
                }
                case "flags.reload": {
                    fetchFlags();
                    LOG.info("Full flag reload triggered via SSE");
                    break;
                }
                default:
                    LOG.fine("Ignored SSE event type: " + type);
            }
        } catch (Exception e) {
            LOG.log(Level.WARNING, "Failed to process SSE event", e);
        }
    }

    private String buildUrl(String path) {
        StringBuilder sb = new StringBuilder(options.getBaseURL()).append(path);
        String sep = "?";
        if (options.getEnvironment() != null) {
            sb.append(sep).append("environment_id=").append(options.getEnvironment());
            sep = "&";
        }
        if (options.getProject() != null) {
            sb.append(sep).append("project=").append(options.getProject());
        }
        return sb.toString();
    }

    // ----------------------------------------------------------- JSON parsing

    static Flag parseFlag(JSONObject json) {
        Flag.Builder builder = Flag.builder()
                .key(json.getString("key"))
                .enabled(json.optBoolean("enabled", false))
                .type(json.optString("type", "boolean"));

        // Parse value
        if (json.has("value")) {
            Object rawValue = json.get("value");
            builder.value(unwrapJsonValue(rawValue));
        }

        // Parse variants
        if (json.has("variants")) {
            JSONObject variants = json.getJSONObject("variants");
            Map<String, Object> variantMap = new HashMap<>();
            for (String vKey : variants.keySet()) {
                variantMap.put(vKey, unwrapJsonValue(variants.get(vKey)));
            }
            builder.variants(variantMap);
        }

        // Parse metadata
        if (json.has("metadata")) {
            builder.metadata(parseMetadata(json.getJSONObject("metadata")));
        }

        return builder.build();
    }

    static FlagMetadata parseMetadata(JSONObject json) {
        FlagMetadata.Builder mb = FlagMetadata.builder();

        if (json.has("category")) {
            try {
                mb.category(FlagCategory.fromValue(json.getString("category")));
            } catch (IllegalArgumentException ignored) {
                // unknown category, leave null
            }
        }

        mb.purpose(json.optString("purpose", null));
        mb.permanent(json.optBoolean("isPermanent", false));

        if (json.has("expiresAt") && !json.isNull("expiresAt")) {
            mb.expiresAt(Instant.parse(json.getString("expiresAt")));
        }

        if (json.has("owners")) {
            JSONArray ownersArr = json.getJSONArray("owners");
            List<String> owners = new ArrayList<>();
            for (int i = 0; i < ownersArr.length(); i++) {
                owners.add(ownersArr.getString(i));
            }
            mb.owners(owners);
        }

        if (json.has("tags")) {
            JSONObject tagsObj = json.getJSONObject("tags");
            Map<String, String> tags = new HashMap<>();
            for (String tKey : tagsObj.keySet()) {
                tags.put(tKey, tagsObj.getString(tKey));
            }
            mb.tags(tags);
        }

        return mb.build();
    }

    private static Object unwrapJsonValue(Object raw) {
        if (raw instanceof JSONObject) {
            return raw.toString();
        } else if (raw instanceof JSONArray) {
            return raw.toString();
        } else if (JSONObject.NULL.equals(raw)) {
            return null;
        }
        return raw;
    }
}

package io.deploysentry;

import org.json.JSONObject;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.Arrays;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.ThreadFactory;
import java.util.concurrent.TimeUnit;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * Agentless status reporter. Posts periodic
 * {@code POST /api/v1/applications/:id/status} samples to DeploySentry on
 * behalf of a {@link DeploySentryClient}. Failures are logged but never
 * bubble up to the client — flag evaluation is unaffected.
 */
public final class StatusReporter {

    private static final Logger LOG = Logger.getLogger(StatusReporter.class.getName());

    private static final List<String> VERSION_ENV_CHAIN = Arrays.asList(
            "APP_VERSION",
            "GIT_SHA",
            "GIT_COMMIT",
            "SOURCE_COMMIT",
            "RAILWAY_GIT_COMMIT_SHA",
            "RENDER_GIT_COMMIT",
            "VERCEL_GIT_COMMIT_SHA",
            "HEROKU_SLUG_COMMIT"
    );

    private static final long MIN_BACKOFF_MS = 1_000L;
    private static final long MAX_BACKOFF_MS = 5 * 60_000L;

    private final ClientOptions options;
    private final HttpClient httpClient;
    private final ScheduledExecutorService scheduler;
    private volatile long backoffMs = 0L;
    private volatile boolean stopped = false;

    public StatusReporter(ClientOptions options, HttpClient httpClient) {
        this.options = options;
        this.httpClient = httpClient;
        this.scheduler = Executors.newSingleThreadScheduledExecutor(daemonFactory());
    }

    /** Resolve the reported version: explicit -> env chain -> "unknown". */
    public static String resolveVersion(String explicit) {
        if (explicit != null && !explicit.isEmpty()) {
            return explicit;
        }
        for (String name : VERSION_ENV_CHAIN) {
            String v = System.getenv(name);
            if (v != null && !v.isEmpty()) {
                return v;
            }
        }
        String implVersion = StatusReporter.class.getPackage().getImplementationVersion();
        if (implVersion != null && !implVersion.isEmpty()) {
            return implVersion;
        }
        return "unknown";
    }

    public void start() {
        stopped = false;
        // Initial report + interval scheduling.
        scheduler.execute(this::tick);
    }

    public void stop() {
        stopped = true;
        scheduler.shutdownNow();
    }

    /** Send exactly one report. Exposed for tests and explicit callers. */
    public void reportOnce() throws Exception {
        String version = resolveVersion(options.getReportStatusVersion());
        HealthReport health;
        if (options.getHealthProvider() != null) {
            try {
                health = options.getHealthProvider().get();
                if (health == null) {
                    health = new HealthReport("healthy");
                }
            } catch (Throwable err) {
                health = new HealthReport("unknown", null, err.getMessage());
            }
        } else {
            health = new HealthReport("healthy");
        }

        JSONObject body = new JSONObject();
        body.put("version", version);
        body.put("health", health.getState());
        if (health.getScore() != null) body.put("health_score", health.getScore());
        if (health.getReason() != null && !health.getReason().isEmpty()) {
            body.put("health_reason", health.getReason());
        }
        if (options.getReportStatusCommitSha() != null) body.put("commit_sha", options.getReportStatusCommitSha());
        if (options.getReportStatusDeploySlot() != null) body.put("deploy_slot", options.getReportStatusDeploySlot());
        Map<String, String> tags = options.getReportStatusTags();
        if (tags != null && !tags.isEmpty()) body.put("tags", tags);

        String baseURL = options.getBaseURL();
        if (baseURL.endsWith("/")) baseURL = baseURL.substring(0, baseURL.length() - 1);
        URI uri = URI.create(baseURL + "/api/v1/applications/" + options.getApplicationId() + "/status");
        HttpRequest req = HttpRequest.newBuilder()
                .uri(uri)
                .timeout(Duration.ofSeconds(10))
                .header("Authorization", "ApiKey " + options.getApiKey())
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body.toString()))
                .build();

        HttpResponse<String> resp = httpClient.send(req, HttpResponse.BodyHandlers.ofString());
        int code = resp.statusCode();
        if (code < 200 || code >= 300) {
            throw new RuntimeException("status report failed: HTTP " + code);
        }
    }

    private void tick() {
        if (stopped) return;
        try {
            reportOnce();
            backoffMs = 0;
        } catch (Exception err) {
            LOG.log(Level.WARNING, "deploysentry: status report error", err);
            if (backoffMs >= MAX_BACKOFF_MS) {
                // Clamped at max — reset so the next schedule falls back to
                // intervalMs instead of another 5 min. Otherwise a server
                // that recovers mid-outage is noticed up to MAX_BACKOFF_MS
                // late regardless of how tight intervalMs is configured. On
                // the next failure the 1s ladder restarts.
                backoffMs = 0;
                long intervalForLog = options.getReportStatusInterval().toMillis();
                LOG.log(Level.WARNING,
                        "deploysentry: status reporter backoff reset; probing every {0}ms",
                        intervalForLog);
            } else {
                backoffMs = backoffMs == 0 ? MIN_BACKOFF_MS : Math.min(backoffMs * 2, MAX_BACKOFF_MS);
            }
        }
        if (stopped) return;
        long intervalMs = options.getReportStatusInterval().toMillis();
        if (intervalMs <= 0) return; // startup-only
        long delay = backoffMs > 0 ? backoffMs : intervalMs;
        try {
            scheduler.schedule(this::tick, delay, TimeUnit.MILLISECONDS);
        } catch (Exception ignored) {
            // scheduler shut down between checks
        }
    }

    private static ThreadFactory daemonFactory() {
        return r -> {
            Thread t = new Thread(r, "deploysentry-status-reporter");
            t.setDaemon(true);
            return t;
        };
    }
}

package io.deploysentry;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.ThreadLocalRandom;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.function.Consumer;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * Lightweight Server-Sent Events (SSE) client built on {@link java.net.http.HttpClient}.
 *
 * <p>Connects to an SSE endpoint and dispatches events to registered consumers.
 * Automatically reconnects on transient failures with exponential back-off.
 */
public final class SSEClient implements AutoCloseable {

    private static final Logger LOG = Logger.getLogger(SSEClient.class.getName());

    private static final long INITIAL_RETRY_MS = 1_000;
    private static final long MAX_RETRY_MS = 30_000;
    private static final double JITTER_FRACTION = 0.2;

    private final HttpClient httpClient;
    private final URI endpoint;
    private final String apiKey;
    private final Consumer<String> onEvent;
    private final Consumer<Throwable> onError;
    private final AtomicBoolean running = new AtomicBoolean(false);
    private final ExecutorService executor;

    /**
     * @param httpClient shared HTTP client
     * @param endpoint   the SSE stream URL
     * @param apiKey     bearer token for authentication
     * @param onEvent    callback receiving the {@code data:} payload of each event
     * @param onError    callback receiving any errors during streaming
     */
    public SSEClient(HttpClient httpClient,
                     URI endpoint,
                     String apiKey,
                     Consumer<String> onEvent,
                     Consumer<Throwable> onError) {
        this.httpClient = httpClient;
        this.endpoint = endpoint;
        this.apiKey = apiKey;
        this.onEvent = onEvent;
        this.onError = onError;
        this.executor = Executors.newSingleThreadExecutor(r -> {
            Thread t = new Thread(r, "deploysentry-sse");
            t.setDaemon(true);
            return t;
        });
    }

    /**
     * Starts the SSE connection in a background thread. Calling this while
     * already connected is a no-op.
     */
    public void connect() {
        if (running.compareAndSet(false, true)) {
            executor.submit(this::streamLoop);
        }
    }

    /**
     * Disconnects and shuts down the background thread.
     */
    @Override
    public void close() {
        running.set(false);
        executor.shutdownNow();
    }

    /** Whether the SSE connection loop is active. */
    public boolean isConnected() {
        return running.get();
    }

    // ---- internals ----

    private void streamLoop() {
        long retryMs = INITIAL_RETRY_MS;

        while (running.get()) {
            try {
                HttpRequest request = HttpRequest.newBuilder()
                        .uri(endpoint)
                        .header("Authorization", "ApiKey " + apiKey)
                        .header("Accept", "text/event-stream")
                        .header("Cache-Control", "no-cache")
                        .GET()
                        .timeout(Duration.ofSeconds(0).plusMillis(Long.MAX_VALUE)) // no timeout
                        .build();

                HttpResponse<java.io.InputStream> response = httpClient.send(
                        request, HttpResponse.BodyHandlers.ofInputStream());

                if (response.statusCode() != 200) {
                    throw new RuntimeException("SSE connection failed with status " + response.statusCode());
                }

                retryMs = INITIAL_RETRY_MS; // reset on successful connect

                try (var reader = new java.io.BufferedReader(
                        new java.io.InputStreamReader(response.body(), java.nio.charset.StandardCharsets.UTF_8))) {

                    StringBuilder dataBuffer = new StringBuilder();
                    String line;

                    while (running.get() && (line = reader.readLine()) != null) {
                        if (line.startsWith("data:")) {
                            dataBuffer.append(line.substring(5).trim());
                        } else if (line.isEmpty() && dataBuffer.length() > 0) {
                            // End of event
                            try {
                                onEvent.accept(dataBuffer.toString());
                            } catch (Exception e) {
                                LOG.log(Level.WARNING, "Error in SSE event handler", e);
                            }
                            dataBuffer.setLength(0);
                        } else if (line.startsWith("retry:")) {
                            try {
                                retryMs = Long.parseLong(line.substring(6).trim());
                            } catch (NumberFormatException ignored) {
                                // keep current retry
                            }
                        }
                        // ignore comment lines (starting with ':')
                    }
                }
            } catch (Exception e) {
                if (!running.get()) {
                    break;
                }
                LOG.log(Level.WARNING, "SSE connection error, retrying in " + retryMs + "ms", e);
                if (onError != null) {
                    try {
                        onError.accept(e);
                    } catch (Exception ignored) {
                        // don't let error handler blow up the loop
                    }
                }
                try {
                    double jitter = retryMs * JITTER_FRACTION * (2 * ThreadLocalRandom.current().nextDouble() - 1);
                    long jitteredDelay = Math.max(0, retryMs + (long) jitter);
                    Thread.sleep(jitteredDelay);
                } catch (InterruptedException ie) {
                    Thread.currentThread().interrupt();
                    break;
                }
                retryMs = Math.min(retryMs * 2, MAX_RETRY_MS);
            }
        }
    }
}

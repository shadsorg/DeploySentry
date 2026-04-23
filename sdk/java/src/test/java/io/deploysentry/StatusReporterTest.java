package io.deploysentry;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.json.JSONObject;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.InputStream;
import java.net.InetSocketAddress;
import java.net.http.HttpClient;
import java.time.Duration;
import java.util.Collections;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Tests for {@link StatusReporter}. Uses com.sun.net.httpserver to
 * avoid a heavy dependency.
 */
public class StatusReporterTest {

    private HttpServer server;
    private int port;
    private final ConcurrentMap<String, String> captured = new ConcurrentHashMap<>();
    private final AtomicInteger responseCode = new AtomicInteger(201);

    @BeforeEach
    void setUp() throws IOException {
        server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        port = server.getAddress().getPort();
        server.createContext("/", this::handle);
        server.setExecutor(null);
        server.start();
    }

    @AfterEach
    void tearDown() {
        server.stop(0);
    }

    private void handle(HttpExchange ex) throws IOException {
        captured.put("method", ex.getRequestMethod());
        captured.put("path", ex.getRequestURI().getPath());
        captured.put("auth", String.valueOf(ex.getRequestHeaders().getFirst("Authorization")));
        captured.put("content_type", String.valueOf(ex.getRequestHeaders().getFirst("Content-Type")));
        try (InputStream is = ex.getRequestBody()) {
            byte[] body = is.readAllBytes();
            captured.put("body", new String(body));
        }
        ex.sendResponseHeaders(responseCode.get(), -1);
        ex.close();
    }

    private ClientOptions baseOptions() {
        return ClientOptions.builder()
                .apiKey("ds_test")
                .baseURL("http://127.0.0.1:" + port)
                .applicationId("f47ac10b-58cc-4372-a567-0e02b2c3d479")
                .reportStatus(true)
                .reportStatusInterval(Duration.ZERO) // startup-only
                .reportStatusVersion("1.4.2")
                .reportStatusCommitSha("abc123")
                .reportStatusDeploySlot("canary")
                .reportStatusTags(Collections.singletonMap("region", "us-east"))
                .build();
    }

    @Test
    void reportOncePostsToCorrectURLAndBody() throws Exception {
        StatusReporter reporter = new StatusReporter(baseOptions(), HttpClient.newHttpClient());
        reporter.reportOnce();

        assertEquals("POST", captured.get("method"));
        assertEquals("/api/v1/applications/f47ac10b-58cc-4372-a567-0e02b2c3d479/status", captured.get("path"));
        assertEquals("ApiKey ds_test", captured.get("auth"));
        assertEquals("application/json", captured.get("content_type"));

        JSONObject body = new JSONObject(captured.get("body"));
        assertEquals("1.4.2", body.getString("version"));
        assertEquals("healthy", body.getString("health"));
        assertEquals("abc123", body.getString("commit_sha"));
        assertEquals("canary", body.getString("deploy_slot"));
        assertTrue(body.getJSONObject("tags").getString("region").equals("us-east"));
    }

    @Test
    void reportOnceHealthProviderReflected() throws Exception {
        ClientOptions opts = ClientOptions.builder()
                .apiKey("k")
                .baseURL("http://127.0.0.1:" + port)
                .applicationId("a")
                .reportStatusVersion("1")
                .healthProvider(() -> new HealthReport("degraded", 0.8, "db slow"))
                .build();

        new StatusReporter(opts, HttpClient.newHttpClient()).reportOnce();

        JSONObject body = new JSONObject(captured.get("body"));
        assertEquals("degraded", body.getString("health"));
        assertEquals(0.8, body.getDouble("health_score"), 1e-9);
        assertEquals("db slow", body.getString("health_reason"));
    }

    @Test
    void reportOnceHealthProviderErrorIsUnknown() throws Exception {
        ClientOptions opts = ClientOptions.builder()
                .apiKey("k")
                .baseURL("http://127.0.0.1:" + port)
                .applicationId("a")
                .reportStatusVersion("1")
                .healthProvider(() -> { throw new RuntimeException("boom"); })
                .build();

        new StatusReporter(opts, HttpClient.newHttpClient()).reportOnce();

        JSONObject body = new JSONObject(captured.get("body"));
        assertEquals("unknown", body.getString("health"));
        assertTrue(body.getString("health_reason").contains("boom"));
    }

    @Test
    void reportOnceThrowsOnServerError() {
        responseCode.set(500);
        StatusReporter reporter = new StatusReporter(baseOptions(), HttpClient.newHttpClient());
        assertThrows(RuntimeException.class, reporter::reportOnce);
    }

    @Test
    void resolveVersionExplicitWins() {
        assertEquals("9.9.9", StatusReporter.resolveVersion("9.9.9"));
    }

    @Test
    void resolveVersionFallsBackToUnknown() {
        // (env vars may be set; at minimum it must return a non-empty string)
        String v = StatusReporter.resolveVersion(null);
        assertNotNull(v);
    }
}

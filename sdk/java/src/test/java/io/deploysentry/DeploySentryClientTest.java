package io.deploysentry;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Unit tests for {@link DeploySentryClient} covering construction validation
 * and default-value behaviour for typed flag evaluations.
 */
class DeploySentryClientTest {

    // ------------------------------------------------------------------
    // Construction / validation
    // ------------------------------------------------------------------

    @Test
    void clientRequiresApiKey() {
        NullPointerException ex = assertThrows(NullPointerException.class, () -> {
            ClientOptions options = ClientOptions.builder()
                    .build(); // apiKey is null
            new DeploySentryClient(options);
        });
        assertTrue(ex.getMessage().contains("apiKey"),
                "Exception message should mention apiKey");
    }

    @Test
    void clientAcceptsValidApiKey() {
        ClientOptions options = ClientOptions.builder()
                .apiKey("ds_test_key123")
                .enableSSE(false)
                .build();

        // Should construct without throwing
        DeploySentryClient client = new DeploySentryClient(options);
        assertNotNull(client);
        client.close();
    }

    // ------------------------------------------------------------------
    // boolValue returns default for missing flag
    // ------------------------------------------------------------------

    @Test
    void boolValue_returnsTrueDefault_whenFlagMissing() {
        DeploySentryClient client = createUninitializedClient();

        boolean result = client.boolValue("nonexistent-flag", true, null);
        assertTrue(result, "Should return the true default for a missing flag");
        client.close();
    }

    @Test
    void boolValue_returnsFalseDefault_whenFlagMissing() {
        DeploySentryClient client = createUninitializedClient();

        boolean result = client.boolValue("nonexistent-flag", false, null);
        assertFalse(result, "Should return the false default for a missing flag");
        client.close();
    }

    @Test
    void boolValue_returnsFalseDefault_withContext() {
        DeploySentryClient client = createUninitializedClient();
        EvaluationContext ctx = EvaluationContext.builder()
                .userId("user-42")
                .build();

        boolean result = client.boolValue("nonexistent-flag", false, ctx);
        assertFalse(result, "Should return the false default even with a context");
        client.close();
    }

    // ------------------------------------------------------------------
    // stringValue returns default for missing flag
    // ------------------------------------------------------------------

    @Test
    void stringValue_returnsDefault_whenFlagMissing() {
        DeploySentryClient client = createUninitializedClient();

        String result = client.stringValue("nonexistent-flag", "fallback", null);
        assertEquals("fallback", result, "Should return the default string for a missing flag");
        client.close();
    }

    @Test
    void stringValue_returnsNullDefault_whenFlagMissing() {
        DeploySentryClient client = createUninitializedClient();

        String result = client.stringValue("nonexistent-flag", null, null);
        assertNull(result, "Should return null default for a missing flag");
        client.close();
    }

    @Test
    void stringValue_returnsEmptyDefault_whenFlagMissing() {
        DeploySentryClient client = createUninitializedClient();

        String result = client.stringValue("nonexistent-flag", "", null);
        assertEquals("", result, "Should return empty string default for a missing flag");
        client.close();
    }

    // ------------------------------------------------------------------
    // intValue returns default for missing flag
    // ------------------------------------------------------------------

    @Test
    void intValue_returnsDefault_whenFlagMissing() {
        DeploySentryClient client = createUninitializedClient();

        int result = client.intValue("nonexistent-flag", 42, null);
        assertEquals(42, result, "Should return the default int for a missing flag");
        client.close();
    }

    // ------------------------------------------------------------------
    // jsonValue returns default for missing flag
    // ------------------------------------------------------------------

    @Test
    void jsonValue_returnsDefault_whenFlagMissing() {
        DeploySentryClient client = createUninitializedClient();

        String result = client.jsonValue("nonexistent-flag", "{}", null);
        assertEquals("{}", result, "Should return the default JSON for a missing flag");
        client.close();
    }

    // ------------------------------------------------------------------
    // detail returns FLAG_NOT_FOUND for missing flag
    // ------------------------------------------------------------------

    @Test
    void detail_returnsFlagNotFound_whenFlagMissing() {
        DeploySentryClient client = createUninitializedClient();

        EvaluationResult<?> result = client.detail("nonexistent-flag", null);
        assertEquals("nonexistent-flag", result.getFlagKey());
        assertNull(result.getValue());
        assertEquals("FLAG_NOT_FOUND", result.getReason());
        assertTrue(result.isDefaulted());
        assertEquals("FLAG_NOT_FOUND", result.getErrorCode());
        client.close();
    }

    // ------------------------------------------------------------------
    // parseFlag smoke test (package-private static method)
    // ------------------------------------------------------------------

    @Test
    void parseFlag_parsesBooleanFlag() {
        org.json.JSONObject json = new org.json.JSONObject()
                .put("key", "test-flag")
                .put("enabled", true)
                .put("type", "boolean")
                .put("value", true);

        Flag flag = DeploySentryClient.parseFlag(json);
        assertEquals("test-flag", flag.getKey());
        assertTrue(flag.isEnabled());
        assertEquals("boolean", flag.getType());
        assertEquals(true, flag.getValue());
    }

    // ------------------------------------------------------------------
    // Helpers
    // ------------------------------------------------------------------

    /**
     * Creates a client that has NOT been initialized (no HTTP calls),
     * so the flag cache is empty. This lets us test default-value behaviour
     * without needing a running server.
     */
    private DeploySentryClient createUninitializedClient() {
        ClientOptions options = ClientOptions.builder()
                .apiKey("ds_test_key_for_unit_tests")
                .enableSSE(false)
                .build();
        return new DeploySentryClient(options);
    }

    // ------------------------------------------------------------------
    // register / dispatch helpers
    // ------------------------------------------------------------------

    private DeploySentryClient newTestClient() {
        return new DeploySentryClient(
            ClientOptions.builder()
                .apiKey("test-key")
                .environment("test")
                .project("test")
                .enableSSE(false)
                .build()
        );
    }

    /**
     * Seeds a flag directly into the client's cache via reflection so tests
     * don't require a running server.
     */
    private void seedFlag(DeploySentryClient client, String key, boolean enabled) {
        try {
            java.lang.reflect.Field field = DeploySentryClient.class.getDeclaredField("cache");
            field.setAccessible(true);
            FlagCache cache = (FlagCache) field.get(client);
            Flag flag = Flag.builder()
                    .key(key)
                    .enabled(enabled)
                    .value(String.valueOf(enabled))
                    .type("boolean")
                    .build();
            cache.put(key, flag);
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    // ------------------------------------------------------------------
    // register / dispatch tests
    // ------------------------------------------------------------------

    @Test
    void dispatchesFlaggedHandlerWhenOn() {
        DeploySentryClient client = newTestClient();
        seedFlag(client, "new-algo", true);

        client.register("process", () -> "new", "new-algo");
        client.register("process", () -> "default");

        String result = client.<String>dispatch("process", null).get();
        assertEquals("new", result, "Should dispatch flagged handler when flag is enabled");
        client.close();
    }

    @Test
    void dispatchesDefaultWhenFlagOff() {
        DeploySentryClient client = newTestClient();
        seedFlag(client, "new-algo", false);

        client.register("process", () -> "new", "new-algo");
        client.register("process", () -> "default");

        String result = client.<String>dispatch("process", null).get();
        assertEquals("default", result, "Should fall back to default when flag is disabled");
        client.close();
    }

    @Test
    void firstMatchWins() {
        DeploySentryClient client = newTestClient();
        seedFlag(client, "flag-a", true);
        seedFlag(client, "flag-b", true);

        client.register("op", () -> "first", "flag-a");
        client.register("op", () -> "second", "flag-b");
        client.register("op", () -> "default");

        String result = client.<String>dispatch("op", null).get();
        assertEquals("first", result, "First registered matching handler should win");
        client.close();
    }

    @Test
    void defaultOnly() {
        DeploySentryClient client = newTestClient();

        client.register("op", () -> "only-default");

        String result = client.<String>dispatch("op", null).get();
        assertEquals("only-default", result, "Default handler should be returned when no flags registered");
        client.close();
    }

    @Test
    void operationsIsolated() {
        DeploySentryClient client = newTestClient();
        seedFlag(client, "feature-x", true);

        client.register("op-a", () -> "a-flagged", "feature-x");
        client.register("op-a", () -> "a-default");
        client.register("op-b", () -> "b-default");

        assertEquals("a-flagged", client.<String>dispatch("op-a", null).get());
        assertEquals("b-default", client.<String>dispatch("op-b", null).get());
        client.close();
    }

    @Test
    void throwsOnUnregistered() {
        DeploySentryClient client = newTestClient();

        IllegalStateException ex = assertThrows(IllegalStateException.class,
                () -> client.dispatch("never-registered", null));
        assertTrue(ex.getMessage().contains("never-registered"),
                "Exception message should mention the operation name");
        client.close();
    }

    @Test
    void throwsNoMatchNoDefault() {
        DeploySentryClient client = newTestClient();
        seedFlag(client, "inactive-flag", false);

        client.register("op", () -> "flagged", "inactive-flag");
        // No default registered

        IllegalStateException ex = assertThrows(IllegalStateException.class,
                () -> client.dispatch("op", null));
        assertTrue(ex.getMessage().contains("op"),
                "Exception message should mention the operation name");
        client.close();
    }

    @Test
    void replacesDefault() {
        DeploySentryClient client = newTestClient();

        client.register("op", () -> "old-default");
        client.register("op", () -> "new-default"); // should replace the default

        String result = client.<String>dispatch("op", null).get();
        assertEquals("new-default", result, "Second register() call without flagKey should replace the default");
        client.close();
    }
}

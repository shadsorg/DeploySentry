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
}

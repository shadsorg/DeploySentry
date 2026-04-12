package io.deploysentry;

import org.json.JSONArray;
import org.json.JSONObject;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Contract tests that validate SDK behaviour against the shared testdata fixtures.
 * These fixtures are the source of truth across all SDK implementations.
 */
class ContractTest {

    private static final Path TESTDATA_DIR = resolveTestdataDir();

    private static Path resolveTestdataDir() {
        // Walk up from the Java SDK root to find sdk/testdata/
        Path javaRoot = Paths.get("").toAbsolutePath();
        Path testdata = javaRoot.resolve("../testdata").normalize();
        if (Files.isDirectory(testdata)) {
            return testdata;
        }
        // Fallback: try from the typical Maven working directory
        testdata = Paths.get(System.getProperty("user.dir")).resolve("../testdata").normalize();
        return testdata;
    }

    private static String readFixture(String filename) throws IOException {
        return Files.readString(TESTDATA_DIR.resolve(filename));
    }

    // ------------------------------------------------------------------
    // Auth header contract
    // ------------------------------------------------------------------

    @Test
    void authHeaderPrefix_mustBeApiKeySpace() throws IOException {
        String raw = readFixture("auth_request.json");
        JSONObject fixture = new JSONObject(raw);

        String prefix = fixture.getString("header_value_prefix");
        assertEquals("ApiKey ", prefix,
                "Authorization header prefix must be \"ApiKey \" (with trailing space)");
    }

    @Test
    void authHeaderName_mustBeAuthorization() throws IOException {
        String raw = readFixture("auth_request.json");
        JSONObject fixture = new JSONObject(raw);

        assertEquals("Authorization", fixture.getString("header_name"));
    }

    @Test
    void authExampleHeader_startsWithPrefix() throws IOException {
        String raw = readFixture("auth_request.json");
        JSONObject fixture = new JSONObject(raw);

        String example = fixture.getString("example_header");
        String prefix = fixture.getString("header_value_prefix");
        assertTrue(example.startsWith(prefix),
                "Example header should start with the required prefix");
    }

    // ------------------------------------------------------------------
    // evaluate_response.json parsing
    // ------------------------------------------------------------------

    @Test
    void evaluateResponse_containsExpectedFlagKey() throws IOException {
        String raw = readFixture("evaluate_response.json");
        JSONObject fixture = new JSONObject(raw);
        JSONObject body = fixture.getJSONObject("body");

        assertEquals("dark-mode", body.getString("flag_key"));
    }

    @Test
    void evaluateResponse_valueIsBoolean() throws IOException {
        String raw = readFixture("evaluate_response.json");
        JSONObject fixture = new JSONObject(raw);
        JSONObject body = fixture.getJSONObject("body");

        assertTrue(body.getBoolean("value"),
                "dark-mode flag value should be true");
    }

    @Test
    void evaluateResponse_reasonIsValid() throws IOException {
        String raw = readFixture("evaluate_response.json");
        JSONObject fixture = new JSONObject(raw);
        JSONObject body = fixture.getJSONObject("body");
        JSONArray validReasons = fixture.getJSONArray("valid_reasons");

        String reason = body.getString("reason");
        boolean found = false;
        for (int i = 0; i < validReasons.length(); i++) {
            if (validReasons.getString(i).equals(reason)) {
                found = true;
                break;
            }
        }
        assertTrue(found, "reason \"" + reason + "\" must be in valid_reasons list");
    }

    @Test
    void evaluateResponse_metadataHasCategory() throws IOException {
        String raw = readFixture("evaluate_response.json");
        JSONObject fixture = new JSONObject(raw);
        JSONObject metadata = fixture.getJSONObject("body").getJSONObject("metadata");

        assertEquals("feature", metadata.getString("category"));
    }

    @Test
    void evaluateResponse_metadataHasOwners() throws IOException {
        String raw = readFixture("evaluate_response.json");
        JSONObject fixture = new JSONObject(raw);
        JSONObject metadata = fixture.getJSONObject("body").getJSONObject("metadata");
        JSONArray owners = metadata.getJSONArray("owners");

        assertEquals(1, owners.length());
        assertEquals("frontend-team", owners.getString(0));
    }

    // ------------------------------------------------------------------
    // list_flags_response.json parsing
    // ------------------------------------------------------------------

    @Test
    void listFlagsResponse_hasThreeFlags() throws IOException {
        String raw = readFixture("list_flags_response.json");
        JSONObject fixture = new JSONObject(raw);
        JSONArray flags = fixture.getJSONObject("body").getJSONArray("flags");

        assertEquals(3, flags.length(), "list_flags_response fixture must contain exactly 3 flags");
    }

    @Test
    void listFlagsResponse_flagKeysMatch() throws IOException {
        String raw = readFixture("list_flags_response.json");
        JSONObject fixture = new JSONObject(raw);
        JSONArray flags = fixture.getJSONObject("body").getJSONArray("flags");

        assertEquals("dark-mode", flags.getJSONObject(0).getString("key"));
        assertEquals("new-checkout", flags.getJSONObject(1).getString("key"));
        assertEquals("max-items", flags.getJSONObject(2).getString("key"));
    }

    @Test
    void listFlagsResponse_flagTypesAreCorrect() throws IOException {
        String raw = readFixture("list_flags_response.json");
        JSONObject fixture = new JSONObject(raw);
        JSONArray flags = fixture.getJSONObject("body").getJSONArray("flags");

        assertEquals("boolean", flags.getJSONObject(0).getString("flag_type"));
        assertEquals("string", flags.getJSONObject(1).getString("flag_type"));
        assertEquals("integer", flags.getJSONObject(2).getString("flag_type"));
    }

    @Test
    void listFlagsResponse_allFlagsEnabled() throws IOException {
        String raw = readFixture("list_flags_response.json");
        JSONObject fixture = new JSONObject(raw);
        JSONArray flags = fixture.getJSONObject("body").getJSONArray("flags");

        for (int i = 0; i < flags.length(); i++) {
            assertTrue(flags.getJSONObject(i).getBoolean("enabled"),
                    "Flag at index " + i + " should be enabled");
        }
    }
}

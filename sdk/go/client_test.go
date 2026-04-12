package deploysentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient(WithAPIKey("test-key"))

	if c.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q; want %q", c.baseURL, defaultBaseURL)
	}
	if c.apiKey != "test-key" {
		t.Errorf("apiKey = %q; want %q", c.apiKey, "test-key")
	}
}

func TestNewClient_WithSessionID(t *testing.T) {
	c := NewClient(
		WithAPIKey("key"),
		WithSessionID("sess-123"),
	)

	if c.sessionID != "sess-123" {
		t.Errorf("sessionID = %q; want %q", c.sessionID, "sess-123")
	}
}

func TestSetAuthHeaders_WithSessionID(t *testing.T) {
	c := NewClient(
		WithAPIKey("my-key"),
		WithSessionID("sess-abc"),
	)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	c.setAuthHeaders(req)

	if got := req.Header.Get("Authorization"); got != "ApiKey my-key" {
		t.Errorf("Authorization = %q; want %q", got, "ApiKey my-key")
	}
	if got := req.Header.Get("X-DeploySentry-Session"); got != "sess-abc" {
		t.Errorf("X-DeploySentry-Session = %q; want %q", got, "sess-abc")
	}
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q; want %q", got, "application/json")
	}
}

func TestSetAuthHeaders_WithoutSessionID(t *testing.T) {
	c := NewClient(WithAPIKey("my-key"))

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	c.setAuthHeaders(req)

	if got := req.Header.Get("X-DeploySentry-Session"); got != "" {
		t.Errorf("X-DeploySentry-Session = %q; want empty (header should be absent)", got)
	}
}

func TestInitialize_WarmsCacheAndStartsSSE(t *testing.T) {
	// Set up a test server that serves the list-flags response and an SSE
	// endpoint that immediately closes (so the goroutine exits quickly).
	mux := http.NewServeMux()

	flags := []Flag{
		{
			Key:          "test-flag",
			Name:         "Test Flag",
			FlagType:     FlagTypeBoolean,
			Enabled:      true,
			DefaultValue: "true",
		},
	}

	mux.HandleFunc("/api/v1/flags", func(w http.ResponseWriter, r *http.Request) {
		resp := listFlagsResponse{Flags: flags}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/v1/flags/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Close immediately so the SSE goroutine terminates.
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(
		WithAPIKey("test-key"),
		WithBaseURL(srv.URL),
		WithProject("proj-1"),
		WithEnvironment("staging"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}
	defer c.Close()

	// Verify the cache was populated.
	cached := c.AllFlags()
	if len(cached) != 1 {
		t.Fatalf("cache has %d flags; want 1", len(cached))
	}
	if cached[0].Key != "test-flag" {
		t.Errorf("cached flag key = %q; want %q", cached[0].Key, "test-flag")
	}
}

func TestBoolValue_Default(t *testing.T) {
	// Create a test server that returns 404 for the evaluate endpoint so the
	// client falls back to defaults. The cache is empty, so we expect the
	// default value.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	}))
	defer srv.Close()

	c := NewClient(
		WithAPIKey("test-key"),
		WithBaseURL(srv.URL),
	)

	val, err := c.BoolValue(context.Background(), "nonexistent-flag", true, nil)
	if err == nil {
		t.Fatal("expected error for missing flag, got nil")
	}
	if val != true {
		t.Errorf("BoolValue() = %v; want true (default)", val)
	}
}

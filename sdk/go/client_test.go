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

func TestRegisterDispatch(t *testing.T) {
	newClient := func() *Client {
		return NewClient(WithAPIKey("test"))
	}
	seedFlag := func(c *Client, key string, enabled bool) {
		c.cache.set(Flag{Key: key, Enabled: enabled})
	}

	t.Run("dispatches flagged handler when flag is on", func(t *testing.T) {
		c := newClient()
		seedFlag(c, "flag-a", true)
		handlerA := func() string { return "A" }
		handlerDefault := func() string { return "default" }
		c.Register("op", handlerDefault)
		c.Register("op", handlerA, "flag-a")

		got := c.Dispatch("op")
		if fmt.Sprintf("%T", got) != fmt.Sprintf("%T", handlerA) {
			t.Errorf("got unexpected handler type %T", got)
		}
		if got.(func() string)() != "A" {
			t.Errorf("got %v; want handler A", got)
		}
	})

	t.Run("dispatches default when flag is off", func(t *testing.T) {
		c := newClient()
		seedFlag(c, "flag-b", false)
		handlerB := func() string { return "B" }
		handlerDefault := func() string { return "default" }
		c.Register("op", handlerDefault)
		c.Register("op", handlerB, "flag-b")

		got := c.Dispatch("op")
		if got.(func() string)() != "default" {
			t.Errorf("got %v; want default handler", got)
		}
	})

	t.Run("first match wins when multiple flags on", func(t *testing.T) {
		c := newClient()
		seedFlag(c, "flag-x", true)
		seedFlag(c, "flag-y", true)
		handlerX := func() string { return "X" }
		handlerY := func() string { return "Y" }
		handlerDefault := func() string { return "default" }
		c.Register("op", handlerDefault)
		c.Register("op", handlerX, "flag-x")
		c.Register("op", handlerY, "flag-y")

		got := c.Dispatch("op")
		if got.(func() string)() != "X" {
			t.Errorf("got %v; want handler X (first registered flagged handler)", got)
		}
	})

	t.Run("default only", func(t *testing.T) {
		c := newClient()
		handlerDefault := func() string { return "default" }
		c.Register("op", handlerDefault)

		got := c.Dispatch("op")
		if got.(func() string)() != "default" {
			t.Errorf("got %v; want default handler", got)
		}
	})

	t.Run("operations isolated", func(t *testing.T) {
		c := newClient()
		seedFlag(c, "flag-a", true)
		handlerA := func() string { return "A" }
		handlerZ := func() string { return "Z" }
		c.Register("op1", handlerA, "flag-a")
		c.Register("op1", func() string { return "default1" })
		c.Register("op2", handlerZ)

		got1 := c.Dispatch("op1")
		if got1.(func() string)() != "A" {
			t.Errorf("op1: got %v; want A", got1)
		}
		got2 := c.Dispatch("op2")
		if got2.(func() string)() != "Z" {
			t.Errorf("op2: got %v; want Z", got2)
		}
	})

	t.Run("panics on unregistered operation", func(t *testing.T) {
		c := newClient()
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("expected panic for unregistered operation, got none")
			}
		}()
		c.Dispatch("nonexistent")
	})

	t.Run("panics when no match and no default", func(t *testing.T) {
		c := newClient()
		seedFlag(c, "flag-off", false)
		c.Register("op", func() string { return "flagged" }, "flag-off")
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("expected panic when no match and no default, got none")
			}
		}()
		c.Dispatch("op")
	})

	t.Run("replaces previous default", func(t *testing.T) {
		c := newClient()
		handlerFirst := func() string { return "first" }
		handlerSecond := func() string { return "second" }
		c.Register("op", handlerFirst)
		c.Register("op", handlerSecond)

		got := c.Dispatch("op")
		if got.(func() string)() != "second" {
			t.Errorf("got %v; want second (replacement default)", got)
		}
	})

	t.Run("passes caller args through", func(t *testing.T) {
		c := newClient()
		seedFlag(c, "flag-a", true)
		handlerA := func(x int) int { return x * 2 }
		c.Register("op", handlerA, "flag-a")
		c.Register("op", func(x int) int { return x })

		got := c.Dispatch("op")
		fn, ok := got.(func(int) int)
		if !ok {
			t.Fatalf("expected func(int)int, got %T", got)
		}
		if fn(5) != 10 {
			t.Errorf("fn(5) = %d; want 10", fn(5))
		}
	})
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

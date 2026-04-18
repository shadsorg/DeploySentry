package sse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestClientReceivesEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		_, _ = fmt.Fprintf(w, "event: deployment.phase_changed\ndata: {\"traffic_percent\":5}\n\n")
		flusher.Flush()
		_, _ = fmt.Fprintf(w, "event: deployment.phase_changed\ndata: {\"traffic_percent\":25}\n\n")
		flusher.Flush()
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	var mu sync.Mutex
	var got []int

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient(server.URL, "", func(trafficPercent int) {
		mu.Lock()
		got = append(got, trafficPercent)
		if len(got) == 2 {
			cancel()
		}
		mu.Unlock()
	})

	client.Connect(ctx)

	mu.Lock()
	defer mu.Unlock()

	if len(got) != 2 {
		t.Fatalf("expected 2 callbacks, got %d", len(got))
	}
	if got[0] != 5 {
		t.Errorf("expected first callback with 5, got %d", got[0])
	}
	if got[1] != 25 {
		t.Errorf("expected second callback with 25, got %d", got[1])
	}
}

func TestClientSendsAuthHeader(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "event: deployment.phase_changed\ndata: {\"traffic_percent\":10}\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key-123", func(int) {})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client.Connect(ctx)

	if gotAuth != "ApiKey test-key-123" {
		t.Errorf("expected Authorization header 'ApiKey test-key-123', got %q", gotAuth)
	}
}

func TestClientIgnoresHeartbeats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
		flusher.Flush()
		_, _ = fmt.Fprintf(w, "event: deployment.phase_changed\ndata: {\"traffic_percent\":50}\n\n")
		flusher.Flush()
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	var mu sync.Mutex
	var got []int

	client := NewClient(server.URL, "", func(trafficPercent int) {
		mu.Lock()
		got = append(got, trafficPercent)
		mu.Unlock()
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client.Connect(ctx)

	mu.Lock()
	defer mu.Unlock()

	if len(got) != 1 || got[0] != 50 {
		t.Errorf("expected [50], got %v", got)
	}
}

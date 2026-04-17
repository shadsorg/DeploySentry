package reporter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestReporterSendsHeartbeats(t *testing.T) {
	var count atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}
		count.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	agentID := uuid.New()
	r := New(srv.URL, "test-key", agentID, 100*time.Millisecond)
	r.SetWeights(map[string]uint32{"blue": 95, "green": 5})

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	r.Start(ctx)

	got := int(count.Load())
	if got < 2 || got > 5 {
		t.Fatalf("expected 2-5 heartbeats, got %d", got)
	}
}

func TestReporterAuthHeader(t *testing.T) {
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := New(srv.URL, "my-secret", uuid.New(), 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	r.Start(ctx)

	if gotAuth != "ApiKey my-secret" {
		t.Fatalf("expected 'ApiKey my-secret', got %q", gotAuth)
	}
}

func TestComputeTraffic(t *testing.T) {
	result := computeTraffic(map[string]uint32{"blue": 95, "green": 5})
	if result["blue"] != 95.0 {
		t.Fatalf("expected blue=95.0, got %f", result["blue"])
	}
	if result["green"] != 5.0 {
		t.Fatalf("expected green=5.0, got %f", result["green"])
	}
}

func TestComputeTrafficEmpty(t *testing.T) {
	result := computeTraffic(nil)
	if len(result) != 0 {
		t.Fatalf("expected empty map, got %v", result)
	}
}

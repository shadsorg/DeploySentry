package xds

import (
	"testing"

	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

func TestBuildSnapshot(t *testing.T) {
	upstreams := map[string]string{"blue": "app-blue:8081", "green": "app-green:8082"}
	weights := map[string]uint32{"blue": 95, "green": 5}

	snap, err := BuildSnapshot("1", upstreams, weights, 8080)
	if err != nil {
		t.Fatalf("BuildSnapshot returned error: %v", err)
	}

	if err := snap.Consistent(); err != nil {
		t.Fatalf("snapshot not consistent: %v", err)
	}

	clusters := snap.GetResources(resource.ClusterType)
	if got := len(clusters); got != 2 {
		t.Errorf("expected 2 clusters, got %d", got)
	}

	listeners := snap.GetResources(resource.ListenerType)
	if got := len(listeners); got != 1 {
		t.Errorf("expected 1 listener, got %d", got)
	}

	routes := snap.GetResources(resource.RouteType)
	if got := len(routes); got != 1 {
		t.Errorf("expected 1 route config, got %d", got)
	}
}

func TestBuildSnapshotWithHeaderOverrides(t *testing.T) {
	upstreams := map[string]string{"blue": "app-blue:8081", "green": "app-green:8082"}
	weights := map[string]uint32{"blue": 95, "green": 5}
	opts := SnapshotOptions{
		HeaderOverrides: []HeaderOverride{
			{Header: "X-Version", Value: "canary", Upstream: "green"},
		},
	}

	snap, err := BuildSnapshotWithOptions("1", upstreams, weights, 8080, opts)
	if err != nil {
		t.Fatalf("BuildSnapshotWithOptions returned error: %v", err)
	}

	if err := snap.Consistent(); err != nil {
		t.Fatalf("snapshot not consistent: %v", err)
	}

	clusters := snap.GetResources(resource.ClusterType)
	if got := len(clusters); got != 2 {
		t.Errorf("expected 2 clusters, got %d", got)
	}
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		addr     string
		wantHost string
		wantPort uint32
	}{
		{"app-blue:8081", "app-blue", 8081},
		{"localhost:80", "localhost", 80},
		{"noport", "noport", 80},
	}
	for _, tt := range tests {
		host, port := splitHostPort(tt.addr)
		if host != tt.wantHost || port != tt.wantPort {
			t.Errorf("splitHostPort(%q) = (%q, %d), want (%q, %d)",
				tt.addr, host, port, tt.wantHost, tt.wantPort)
		}
	}
}

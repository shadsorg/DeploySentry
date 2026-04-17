package xds

import (
	"context"
	"testing"
	"time"
)

func TestServerStartAndUpdate(t *testing.T) {
	srv, err := NewServer(0) // port 0 lets the OS pick a free port
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	upstreams := map[string]string{"blue": "localhost:8081", "green": "localhost:8082"}
	weights := map[string]uint32{"blue": 95, "green": 5}

	if err := srv.UpdateWeights(upstreams, weights, 8080, SnapshotOptions{}); err != nil {
		t.Fatalf("first UpdateWeights: %v", err)
	}
	if v := srv.ConfigVersion(); v != 1 {
		t.Errorf("expected ConfigVersion 1, got %d", v)
	}

	weights["blue"] = 75
	weights["green"] = 25
	if err := srv.UpdateWeights(upstreams, weights, 8080, SnapshotOptions{}); err != nil {
		t.Fatalf("second UpdateWeights: %v", err)
	}
	if v := srv.ConfigVersion(); v != 2 {
		t.Errorf("expected ConfigVersion 2, got %d", v)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Logf("Start returned: %v (expected after graceful stop)", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop within 3 seconds")
	}
}

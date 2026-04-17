package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/deploysentry/deploysentry/internal/agent"
	"github.com/deploysentry/deploysentry/internal/agent/reporter"
	"github.com/deploysentry/deploysentry/internal/agent/sse"
	"github.com/deploysentry/deploysentry/internal/agent/xds"
	"github.com/google/uuid"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	cfg, err := agent.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	log.Printf("deploysentry-agent starting (app=%s, env=%s, xds=:%d)", cfg.AppID, cfg.Environment, cfg.EnvoyXDSPort)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sigCh; log.Println("shutdown signal received"); cancel() }()

	// Start xDS server
	xdsSrv, err := xds.NewServer(cfg.EnvoyXDSPort)
	if err != nil {
		return fmt.Errorf("creating xDS server: %w", err)
	}
	go func() {
		if err := xdsSrv.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("xDS server error: %v", err)
		}
	}()

	// Push initial equal-weight snapshot
	initialWeights := make(map[string]uint32, len(cfg.Upstreams))
	for name := range cfg.Upstreams {
		initialWeights[name] = 50
	}
	if err := xdsSrv.UpdateWeights(cfg.Upstreams, initialWeights, uint32(cfg.EnvoyListenPort), xds.SnapshotOptions{}); err != nil {
		return fmt.Errorf("initial xDS snapshot: %w", err)
	}

	// Register with the DeploySentry API
	agentID, err := registerAgent(cfg)
	if err != nil {
		log.Printf("warning: agent registration failed (running unregistered): %v", err)
		agentID = uuid.New()
	}

	// Start heartbeat reporter
	rep := reporter.New(cfg.APIURL, cfg.APIKey, agentID, cfg.HeartbeatInterval)
	rep.SetWeights(initialWeights)
	go rep.Start(ctx)

	// SSE callback: update xDS weights when desired state changes
	sseCallback := func(trafficPercent int) {
		log.Printf("SSE: desired traffic percent = %d%%", trafficPercent)
		newWeights := map[string]uint32{
			"blue":  uint32(100 - trafficPercent),
			"green": uint32(trafficPercent),
		}
		if err := xdsSrv.UpdateWeights(cfg.Upstreams, newWeights, uint32(cfg.EnvoyListenPort), xds.SnapshotOptions{}); err != nil {
			log.Printf("xDS update failed: %v", err)
			return
		}
		rep.SetWeights(newWeights)
		rep.SetConfigVersion(xdsSrv.ConfigVersion())
		log.Printf("traffic updated: blue=%d%% green=%d%%", 100-trafficPercent, trafficPercent)
	}

	// Start SSE client
	sseURL := fmt.Sprintf("%s/api/v1/flags/stream?application=%s", cfg.APIURL, cfg.AppID)
	sseClient := sse.NewClient(sseURL, cfg.APIKey, sseCallback)
	go sseClient.Connect(ctx)

	log.Printf("agent running (id=%s)", agentID)
	<-ctx.Done()
	log.Println("agent shutting down")
	return nil
}

// registerAgent calls the DeploySentry API to register this agent.
func registerAgent(cfg *agent.Config) (uuid.UUID, error) {
	upstreamsJSON, _ := json.Marshal(cfg.Upstreams)
	body := fmt.Sprintf(`{"app_id":"%s","environment_id":"%s","version":"0.1.0","upstreams":%s}`,
		cfg.AppID, cfg.AppID, string(upstreamsJSON))

	url := fmt.Sprintf("%s/api/v1/agents/register", cfg.APIURL)
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return uuid.Nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("ApiKey %s", cfg.APIKey))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return uuid.Nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return uuid.Nil, fmt.Errorf("registration returned %d", resp.StatusCode)
	}

	var result struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return uuid.Nil, err
	}
	return result.ID, nil
}

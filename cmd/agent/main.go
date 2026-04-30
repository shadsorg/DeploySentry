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

	"github.com/shadsorg/deploysentry/internal/agent"
	"github.com/shadsorg/deploysentry/internal/agent/reporter"
	"github.com/shadsorg/deploysentry/internal/agent/sse"
	"github.com/shadsorg/deploysentry/internal/agent/xds"
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

	log.Printf("deploysentry-agent starting (env=%s, xds=:%d)", cfg.Environment, cfg.EnvoyXDSPort)

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

	// Register with the DeploySentry API. The response includes scope
	// derived from the API key (app_id, environment_id) when the key is
	// scoped to a specific application.
	reg, err := registerAgent(cfg)
	var agentID, appID uuid.UUID
	var environmentID uuid.UUID
	if err != nil {
		log.Printf("warning: agent registration failed: %v", err)
		if cfg.AppID == nil {
			return fmt.Errorf("registration failed and DS_APP_ID not set: %w", err)
		}
		agentID = uuid.New()
		appID = *cfg.AppID
	} else {
		agentID = reg.AgentID
		appID = reg.AppID
		environmentID = reg.EnvironmentID
	}

	// Explicit config overrides key-derived scope.
	if cfg.AppID != nil {
		appID = *cfg.AppID
	}
	_ = environmentID // used in SSE URL / heartbeat downstream

	log.Printf("agent running (id=%s, app=%s)", agentID, appID)

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
	sseURL := fmt.Sprintf("%s/api/v1/flags/stream?application=%s", cfg.APIURL, appID)
	sseClient := sse.NewClient(sseURL, cfg.APIKey, sseCallback)
	go sseClient.Connect(ctx)

	<-ctx.Done()
	log.Println("agent shutting down")
	return nil
}

// registrationResult holds the outcome of a successful agent registration,
// including scope derived from the API key on the server side.
type registrationResult struct {
	AgentID       uuid.UUID
	AppID         uuid.UUID
	EnvironmentID uuid.UUID
}

// registerAgent calls the DeploySentry API to register this agent.
func registerAgent(cfg *agent.Config) (*registrationResult, error) {
	body := map[string]interface{}{
		"version":   "0.1.0",
		"upstreams": cfg.Upstreams,
	}
	// Only include app_id if explicitly configured (otherwise let server derive from key)
	if cfg.AppID != nil {
		body["app_id"] = cfg.AppID.String()
	}

	bodyBytes, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/v1/agents/register", cfg.APIURL)
	req, err := http.NewRequest("POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("ApiKey %s", cfg.APIKey))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("registration returned %d", resp.StatusCode)
	}

	var result struct {
		ID            uuid.UUID `json:"id"`
		AppID         uuid.UUID `json:"app_id"`
		EnvironmentID uuid.UUID `json:"environment_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &registrationResult{
		AgentID:       result.ID,
		AppID:         result.AppID,
		EnvironmentID: result.EnvironmentID,
	}, nil
}

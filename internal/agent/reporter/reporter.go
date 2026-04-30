package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Reporter periodically sends heartbeat payloads to the DeploySentry API.
type Reporter struct {
	apiURL   string
	apiKey   string
	agentID  uuid.UUID
	interval time.Duration
	client   *http.Client

	mu            sync.Mutex
	weights       map[string]uint32
	deploymentID  *uuid.UUID
	configVersion int64
}

// New creates a Reporter that sends heartbeats every interval.
func New(apiURL, apiKey string, agentID uuid.UUID, interval time.Duration) *Reporter {
	return &Reporter{
		apiURL:   apiURL,
		apiKey:   apiKey,
		agentID:  agentID,
		interval: interval,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// SetWeights updates the current traffic weights. Thread-safe.
func (r *Reporter) SetWeights(w map[string]uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.weights = w
}

// SetDeploymentID updates the current deployment ID. Thread-safe.
func (r *Reporter) SetDeploymentID(id *uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deploymentID = id
}

// SetConfigVersion updates the current config version. Thread-safe.
func (r *Reporter) SetConfigVersion(v int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configVersion = v
}

// Start sends heartbeats at the configured interval until ctx is cancelled.
func (r *Reporter) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.send(ctx); err != nil {
				log.Printf("reporter: heartbeat failed: %v", err)
			}
		}
	}
}

func (r *Reporter) send(ctx context.Context) error {
	payload := r.buildPayload()

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling payload: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/agents/%s/heartbeat", r.apiURL, r.agentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if r.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+r.apiKey)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending heartbeat: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (r *Reporter) buildPayload() models.HeartbeatPayload {
	r.mu.Lock()
	weights := r.weights
	deploymentID := r.deploymentID
	configVersion := r.configVersion
	r.mu.Unlock()

	actual := computeTraffic(weights)
	intWeights := make(map[string]int, len(weights))
	for k, v := range weights {
		intWeights[k] = int(v)
	}

	return models.HeartbeatPayload{
		AgentID:       r.agentID,
		DeploymentID:  deploymentID,
		ConfigVersion: int(configVersion),
		ActualTraffic: actual,
		Upstreams:     map[string]models.UpstreamMetrics{},
		ActiveRules: models.ActiveRules{
			Weights: intWeights,
		},
		EnvoyHealthy: true,
	}
}

// computeTraffic converts integer weights to percentage floats.
func computeTraffic(weights map[string]uint32) map[string]float64 {
	if len(weights) == 0 {
		return map[string]float64{}
	}
	var total uint64
	for _, w := range weights {
		total += uint64(w)
	}
	if total == 0 {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(weights))
	for k, w := range weights {
		out[k] = float64(w) / float64(total) * 100.0
	}
	return out
}

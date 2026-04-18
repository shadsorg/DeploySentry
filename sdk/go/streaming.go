package deploysentry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

const (
	sseStreamPath       = "/api/v1/flags/stream"
	initialBackoff      = 1 * time.Second
	maxBackoff          = 30 * time.Second
	backoffMultiplier   = 2.0
	jitterFraction      = 0.2
)

// sseClient manages a Server-Sent Events connection to the DeploySentry
// streaming endpoint for real-time flag updates.
type sseClient struct {
	baseURL     string
	apiKey      string
	projectID   string
	environment string
	sessionID   string
	httpClient  *http.Client
	onUpdate    func(Flag)
	logger      *log.Logger

	cancel context.CancelFunc
}

// newSSEClient creates an SSE client. Call start() to begin streaming.
func newSSEClient(baseURL, apiKey, projectID, environment, sessionID string, httpClient *http.Client, onUpdate func(Flag), logger *log.Logger) *sseClient {
	return &sseClient{
		baseURL:     baseURL,
		apiKey:      apiKey,
		projectID:   projectID,
		environment: environment,
		sessionID:   sessionID,
		httpClient:  httpClient,
		onUpdate:    onUpdate,
		logger:      logger,
	}
}

// start begins the SSE connection in a background goroutine. It reconnects
// automatically with exponential backoff on failure.
func (s *sseClient) start(ctx context.Context) {
	streamCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go s.connectLoop(streamCtx)
}

// stop terminates the streaming connection.
func (s *sseClient) stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// connectLoop runs the reconnect loop with exponential backoff.
func (s *sseClient) connectLoop(ctx context.Context) {
	backoff := initialBackoff

	for {
		err := s.connect(ctx)
		if ctx.Err() != nil {
			// Context cancelled; stop reconnecting.
			return
		}

		if err != nil {
			s.logger.Printf("deploysentry: SSE connection error: %v; reconnecting in %s", err, backoff)
		}

		// Apply +/- 20% jitter to prevent thundering herd.
		jittered := applyJitter(backoff)

		select {
		case <-ctx.Done():
			return
		case <-time.After(jittered):
		}

		// Exponential backoff with cap.
		backoff = time.Duration(math.Min(
			float64(backoff)*backoffMultiplier,
			float64(maxBackoff),
		))
	}
}

// applyJitter adds +/- 20% randomization to a duration.
func applyJitter(d time.Duration) time.Duration {
	jitter := float64(d) * jitterFraction * (2*rand.Float64() - 1)
	return time.Duration(float64(d) + jitter)
}

// connect opens a single SSE connection and reads events until the
// connection drops or the context is cancelled.
func (s *sseClient) connect(ctx context.Context) error {
	url := fmt.Sprintf("%s%s?project_id=%s&environment_id=%s",
		s.baseURL, sseStreamPath, s.projectID, s.environment)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating SSE request: %w", err)
	}

	req.Header.Set("Authorization", "ApiKey "+s.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if s.sessionID != "" {
		req.Header.Set("X-DeploySentry-Session", s.sessionID)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opening SSE connection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE endpoint returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			dataLines = append(dataLines, data)
			continue
		}

		// Empty line signals end of an event.
		if line == "" && len(dataLines) > 0 {
			payload := strings.Join(dataLines, "\n")
			dataLines = nil
			s.handleEvent(payload)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading SSE stream: %w", err)
	}

	return fmt.Errorf("SSE stream closed by server")
}

// sseEvent represents the payload sent by the server on the SSE stream.
type sseEvent struct {
	Event   string `json:"event"`
	FlagID  string `json:"flag_id"`
	FlagKey string `json:"flag_key"`
}

// handleEvent parses a single SSE event payload, fetches the updated flag
// from the API, and invokes the update callback.
func (s *sseClient) handleEvent(data string) {
	var evt sseEvent
	if err := json.Unmarshal([]byte(data), &evt); err != nil {
		s.logger.Printf("deploysentry: failed to parse SSE event: %v", err)
		return
	}

	if evt.FlagID == "" {
		s.logger.Printf("deploysentry: SSE event missing flag_id, skipping")
		return
	}

	flag, err := s.fetchFlag(evt.FlagID)
	if err != nil {
		s.logger.Printf("deploysentry: failed to fetch flag %s: %v", evt.FlagID, err)
		return
	}

	if s.onUpdate != nil {
		s.onUpdate(flag)
	}
}

// fetchFlag retrieves a single flag by ID from the API.
func (s *sseClient) fetchFlag(flagID string) (Flag, error) {
	url := fmt.Sprintf("%s/api/v1/flags/%s?environment_id=%s", s.baseURL, flagID, s.environment)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Flag{}, fmt.Errorf("creating flag request: %w", err)
	}

	req.Header.Set("Authorization", "ApiKey "+s.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return Flag{}, fmt.Errorf("fetching flag: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Flag{}, fmt.Errorf("flag endpoint returned status %d", resp.StatusCode)
	}

	var flag Flag
	if err := json.NewDecoder(resp.Body).Decode(&flag); err != nil {
		return Flag{}, fmt.Errorf("decoding flag response: %w", err)
	}

	return flag, nil
}

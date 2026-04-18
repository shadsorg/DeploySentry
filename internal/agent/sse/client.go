package sse

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

// TrafficCallback is called when a phase_changed event is received.
type TrafficCallback func(trafficPercent int)

// Client connects to a DeploySentry SSE stream.
type Client struct {
	url      string
	apiKey   string
	callback TrafficCallback
	client   *http.Client
}

// NewClient creates a new SSE client.
func NewClient(url, apiKey string, callback TrafficCallback) *Client {
	return &Client{
		url:      url,
		apiKey:   apiKey,
		callback: callback,
		client:   &http.Client{},
	}
}

type phaseEvent struct {
	TrafficPercent int `json:"traffic_percent"`
}

// Connect opens the SSE stream and processes events. Blocks until ctx is cancelled.
// Reconnects with exponential backoff on disconnection (1s initial, 30s max, 2x factor).
func (c *Client) Connect(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		err := c.stream(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("sse: connection error: %v, reconnecting in %s", err, backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff = time.Duration(math.Min(float64(backoff*2), float64(maxBackoff)))
	}
}

func (c *Client) stream(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("connecting: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	var eventType, data string

	for scanner.Scan() {
		line := scanner.Text()

		// Heartbeat — ignore
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Empty line — dispatch event and reset
		if line == "" {
			if eventType == "deployment.phase_changed" && data != "" {
				var ev phaseEvent
				if err := json.Unmarshal([]byte(data), &ev); err == nil {
					c.callback(ev.TrafficPercent)
				}
			}
			eventType = ""
			data = ""
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		}
	}

	return scanner.Err()
}

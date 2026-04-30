// Package main is the entrypoint for the DeploySentry Reference Consumer.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"
)

type Config struct {
	APIURL          string
	APIKey          string
	AppID           string
	PollInterval    time.Duration
	NginxConfPath   string
	StableBackend   string
	CanaryBackend   string
}

type DesiredStateResponse struct {
	Deployments []struct {
		DesiredTrafficPercent int `json:"desired_traffic_percent"`
		Status                string `json:"status"`
	} `json:"deployments"`
}

type NginxConfig struct {
	StableWeight int
	CanaryWeight int
	StableHost   string
	CanaryHost   string
}

const nginxTemplate = `upstream app {
    server {{.StableHost}} weight={{.StableWeight}};
    server {{.CanaryHost}} weight={{.CanaryWeight}};
}
`

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	cfg := loadConfig()

	log.Printf("Consumer starting - API: %s, App: %s, Poll Interval: %v", cfg.APIURL, cfg.AppID, cfg.PollInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	lastCanaryPercent := -1
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	mu := sync.Mutex{}

	for {
		select {
		case <-sigCh:
			log.Println("Shutdown signal received, exiting gracefully")
			return nil
		case <-ticker.C:
			canaryPercent, err := pollDesiredState(ctx, cfg)
			if err != nil {
				log.Printf("Error polling desired state: %v", err)
				continue
			}

			mu.Lock()
			if canaryPercent != lastCanaryPercent {
				log.Printf("Traffic weight changed: stable=%d%%, canary=%d%%", 100-canaryPercent, canaryPercent)
				if err := updateNginxConfig(cfg, canaryPercent); err != nil {
					log.Printf("Error updating nginx config: %v", err)
				} else {
					if err := reloadNginx(); err != nil {
						log.Printf("Error reloading nginx: %v", err)
					}
				}
				lastCanaryPercent = canaryPercent
			}
			mu.Unlock()
		}
	}
}

func loadConfig() Config {
	return Config{
		APIURL:        getEnv("DS_API_URL", "http://localhost:8080"),
		APIKey:        getEnv("DS_API_KEY", ""),
		AppID:         getEnv("DS_APP_ID", ""),
		PollInterval:  parseDuration(getEnv("DS_POLL_INTERVAL", "5s")),
		NginxConfPath: getEnv("DS_NGINX_CONF_PATH", "/etc/nginx/conf.d/app.conf"),
		StableBackend: getEnv("DS_STABLE_BACKEND", "app-stable:8081"),
		CanaryBackend: getEnv("DS_CANARY_BACKEND", "app-canary:8082"),
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("Invalid duration %q, using 5s default", s)
		return 5 * time.Second
	}
	return d
}

func pollDesiredState(ctx context.Context, cfg Config) (int, error) {
	url := fmt.Sprintf("%s/api/v1/applications/%s/desired-state", cfg.APIURL, cfg.AppID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var response DesiredStateResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0, err
	}

	// Find the first active deployment
	for _, deployment := range response.Deployments {
		if strings.ToLower(deployment.Status) == "active" {
			return deployment.DesiredTrafficPercent, nil
		}
	}

	// Default to 0 if no active deployment
	return 0, nil
}

func updateNginxConfig(cfg Config, canaryPercent int) error {
	// Calculate weights (nginx doesn't allow 0)
	stableWeight := 100 - canaryPercent
	canaryWeight := canaryPercent

	if canaryWeight == 0 {
		canaryWeight = 1
	}
	if stableWeight == 0 {
		stableWeight = 1
	}

	// Parse the backend addresses
	ncfg := NginxConfig{
		StableWeight: stableWeight,
		CanaryWeight: canaryWeight,
		StableHost:   cfg.StableBackend,
		CanaryHost:   cfg.CanaryBackend,
	}

	// Generate config
	tmpl, err := template.New("nginx").Parse(nginxTemplate)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(cfg.NginxConfPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write config file
	file, err := os.Create(cfg.NginxConfPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	if err := tmpl.Execute(file, ncfg); err != nil {
		return err
	}

	log.Printf("Updated nginx config: %s (stable=%d, canary=%d)", cfg.NginxConfPath, stableWeight, canaryWeight)
	return nil
}

func reloadNginx() error {
	// In Docker, nginx reload requires PID namespace sharing.
	// For this demo, we just log the action.
	log.Println("[STUB] nginx reload would execute: nginx -s reload")
	return nil
}

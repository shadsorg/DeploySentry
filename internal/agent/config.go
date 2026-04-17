package agent

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Config holds all agent configuration, loaded from environment variables.
type Config struct {
	APIURL            string
	APIKey            string
	AppID             uuid.UUID
	Environment       string
	Upstreams         map[string]string // name -> host:port
	EnvoyXDSPort      int
	EnvoyListenPort   int
	HeartbeatInterval time.Duration
}

// LoadConfig reads agent configuration from environment variables.
func LoadConfig() (*Config, error) {
	appIDStr := os.Getenv("DS_APP_ID")
	if appIDStr == "" {
		return nil, fmt.Errorf("DS_APP_ID is required")
	}
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		return nil, fmt.Errorf("DS_APP_ID is not a valid UUID: %w", err)
	}

	upstreamStr := os.Getenv("DS_UPSTREAMS")
	if upstreamStr == "" {
		return nil, fmt.Errorf("DS_UPSTREAMS is required")
	}
	upstreams, err := parseUpstreams(upstreamStr)
	if err != nil {
		return nil, fmt.Errorf("DS_UPSTREAMS: %w", err)
	}

	return &Config{
		APIURL:            getEnv("DS_API_URL", "http://localhost:8080"),
		APIKey:            getEnv("DS_API_KEY", ""),
		AppID:             appID,
		Environment:       getEnv("DS_ENVIRONMENT", "production"),
		Upstreams:         upstreams,
		EnvoyXDSPort:      getEnvInt("DS_ENVOY_XDS_PORT", 18000),
		EnvoyListenPort:   getEnvInt("DS_ENVOY_LISTEN_PORT", 8080),
		HeartbeatInterval: getEnvDuration("DS_HEARTBEAT_INTERVAL", 5*time.Second),
	}, nil
}

// parseUpstreams parses the format "blue:host:port,green:host:port".
func parseUpstreams(s string) (map[string]string, error) {
	result := make(map[string]string)
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid upstream %q: expected name:host:port", entry)
		}
		name, host, port := parts[0], parts[1], parts[2]
		if name == "" || host == "" || port == "" {
			return nil, fmt.Errorf("invalid upstream %q: name, host, and port must be non-empty", entry)
		}
		result[name] = host + ":" + port
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no valid upstreams found")
	}
	return result, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// Package config provides application configuration loading and management.
// It uses viper to support environment variables, config files, and defaults.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Redis         RedisConfig
	NATS          NATSConfig
	Auth          AuthConfig
	Log           LogConfig
	Notifications NotificationsConfig
	GitHub        GitHubConfig
	Security      SecurityConfig
	Health        HealthConfig
}

// HealthConfig holds configuration for the health monitoring subsystem.
type HealthConfig struct {
	CheckIntervalSeconds int     `mapstructure:"check_interval_seconds"`
	Threshold            float64 `mapstructure:"threshold"`

	Prometheus PrometheusIntegrationConfig `mapstructure:"prometheus"`
	Datadog    DatadogIntegrationConfig    `mapstructure:"datadog"`
	Sentry     SentryIntegrationConfig     `mapstructure:"sentry"`
}

// PrometheusIntegrationConfig holds configuration for the Prometheus health check.
type PrometheusIntegrationConfig struct {
	BaseURL              string  `mapstructure:"base_url"`
	ErrorRateQuery       string  `mapstructure:"error_rate_query"`
	LatencyQuery         string  `mapstructure:"latency_query"`
	ErrorRateThreshold   float64 `mapstructure:"error_rate_threshold"`
	LatencyThresholdSec  float64 `mapstructure:"latency_threshold_sec"`
}

// DatadogIntegrationConfig holds configuration for the Datadog health check.
type DatadogIntegrationConfig struct {
	APIKey              string  `mapstructure:"api_key"`
	AppKey              string  `mapstructure:"app_key"`
	Site                string  `mapstructure:"site"`
	ErrorRateMetric     string  `mapstructure:"error_rate_metric"`
	LatencyMetric       string  `mapstructure:"latency_metric"`
	ErrorRateThreshold  float64 `mapstructure:"error_rate_threshold"`
	LatencyThresholdSec float64 `mapstructure:"latency_threshold_sec"`
}

// SentryIntegrationConfig holds configuration for the Sentry health check.
type SentryIntegrationConfig struct {
	BaseURL        string `mapstructure:"base_url"`
	AuthToken      string `mapstructure:"auth_token"`
	Organization   string `mapstructure:"organization"`
	Project        string `mapstructure:"project"`
	ErrorThreshold int    `mapstructure:"error_threshold"`
}

// SecurityConfig holds security-related configuration.
type SecurityConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"`
}

// NotificationsConfig holds configuration for all notification channels.
type NotificationsConfig struct {
	Slack    SlackNotificationConfig    `mapstructure:"slack"`
	Email    EmailNotificationConfig    `mapstructure:"email"`
	PagerDuty PagerDutyNotificationConfig `mapstructure:"pagerduty"`
}

// SlackNotificationConfig holds Slack webhook configuration.
type SlackNotificationConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WebhookURL string `mapstructure:"webhook_url"`
	Channel    string `mapstructure:"channel"`
	Username   string `mapstructure:"username"`
}

// EmailNotificationConfig holds SMTP configuration.
type EmailNotificationConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	SMTPHost   string `mapstructure:"smtp_host"`
	SMTPPort   int    `mapstructure:"smtp_port"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	FromName   string `mapstructure:"from_name"`
	FromEmail  string `mapstructure:"from_email"`
}

// PagerDutyNotificationConfig holds PagerDuty configuration.
type PagerDutyNotificationConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	RoutingKey string `mapstructure:"routing_key"`
}

// GitHubConfig holds GitHub integration configuration.
type GitHubConfig struct {
	WebhookSecret      string   `mapstructure:"webhook_secret"`
	AutoDeploy         bool     `mapstructure:"auto_deploy"`
	DeployBranches     []string `mapstructure:"deploy_branches"`
	DefaultProjectID   string   `mapstructure:"default_project_id"`
	DefaultEnvironmentID string `mapstructure:"default_environment_id"`
	DefaultStrategy    string   `mapstructure:"default_strategy"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// DatabaseConfig holds PostgreSQL connection configuration.
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	Schema          string        `mapstructure:"schema"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// DSN returns the PostgreSQL connection string. The search_path is set to the
// configured schema (default: "deploy") so all tables live in the deploy
// namespace rather than public.
func (d DatabaseConfig) DSN() string {
	schema := d.Schema
	if schema == "" {
		schema = "deploy"
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&search_path=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode, schema,
	)
}

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Addr returns the Redis address in host:port format.
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// NATSConfig holds NATS connection configuration.
type NATSConfig struct {
	URL            string        `mapstructure:"url"`
	MaxReconnects  int           `mapstructure:"max_reconnects"`
	ReconnectWait  time.Duration `mapstructure:"reconnect_wait"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
}

// AuthConfig holds authentication and authorization configuration.
type AuthConfig struct {
	JWTSecret         string        `mapstructure:"jwt_secret"`
	JWTExpiration     time.Duration `mapstructure:"jwt_expiration"`
	OAuth2ClientID    string        `mapstructure:"oauth2_client_id"`
	OAuth2Secret      string        `mapstructure:"oauth2_client_secret"`
	OAuth2RedirectURL string        `mapstructure:"oauth2_redirect_url"`
	SessionTTL        time.Duration `mapstructure:"session_ttl"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load reads configuration from environment variables and optional config files.
// Environment variables are prefixed with DS_ and use underscores as separators.
// For example, DS_SERVER_PORT maps to Config.Server.Port.
func Load() (*Config, error) {
	v := viper.New()

	// Set default values.
	setDefaults(v)

	// Read from environment variables with DS_ prefix.
	v.SetEnvPrefix("DS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Optionally read from config file.
	v.SetConfigName(".deploysentry")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME")
	v.AddConfigPath("/etc/deploysentry")

	// Config file is optional; ignore not-found errors.
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Validate the encryption key length now instead of at first write.
	// Writes through crypto.Encrypt require len(key) == 32 — an empty or
	// wrong-length key would produce cryptic "key must be 32 bytes, got N"
	// errors deep in handler paths (webhooks, deploy integrations, etc.)
	// and leave users without a clear remediation.
	if err := validateEncryptionKey(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateEncryptionKey enforces a 32-byte DS_SECURITY_ENCRYPTION_KEY.
// When the key is empty and the environment is "development", generate a
// deterministic dev-only key (with a loud warning) so local contributors
// don't need to configure it to try the stack.
func validateEncryptionKey(cfg *Config) error {
	key := cfg.Security.EncryptionKey
	if len(key) == 32 {
		return nil
	}
	if key == "" {
		if isDevEnvironment() {
			cfg.Security.EncryptionKey = devEncryptionKey
			fmt.Fprintln(os.Stderr,
				"WARNING: DS_SECURITY_ENCRYPTION_KEY is not set; using a built-in development key. "+
					"This is insecure and MUST NOT be used in production. "+
					"Set DS_SECURITY_ENCRYPTION_KEY to a 32-byte secret in any non-dev deployment.",
			)
			return nil
		}
		return fmt.Errorf(
			"DS_SECURITY_ENCRYPTION_KEY is required (32 bytes); generate one with " +
				"`openssl rand -hex 16 | head -c 32` (32 ASCII bytes, safe in env vars), " +
				"or set DS_ENV=development to use a built-in key for local dev",
		)
	}
	return fmt.Errorf(
		"DS_SECURITY_ENCRYPTION_KEY must be exactly 32 bytes (got %d); "+
			"generate one with `openssl rand -hex 16 | head -c 32`",
		len(key),
	)
}

// devEncryptionKey is a fixed 32-byte string used ONLY when DS_ENV=development
// and no explicit key is supplied. It is intentionally obvious so any leak
// into a production artifact is trivially greppable.
const devEncryptionKey = "ds-dev-only-encryption-key-32byt"

// isDevEnvironment inspects DS_ENVIRONMENT (canonical) with a DS_ENV
// fallback for historical CLI usage. Empty or any dev-ish value counts
// as development.
func isDevEnvironment() bool {
	env := strings.ToLower(os.Getenv("DS_ENVIRONMENT"))
	if env == "" {
		env = strings.ToLower(os.Getenv("DS_ENV"))
	}
	return env == "" || env == "dev" || env == "development" || env == "local"
}

// ValidateProduction checks that all required settings for production
// deployment are properly configured and returns an error if any are missing
// or use insecure defaults.
func (c *Config) ValidateProduction() error {
	errors := []string{}

	// Check authentication security
	if c.Auth.JWTSecret == "" || c.Auth.JWTSecret == "change-me-in-production" || c.Auth.JWTSecret == "change-me-in-production-use-a-strong-random-string" {
		errors = append(errors, "DS_AUTH_JWT_SECRET must be set to a strong random string in production")
	}

	// Check database security
	if c.Database.SSLMode == "disable" {
		errors = append(errors, "DS_DATABASE_SSL_MODE should be 'require' or 'verify-full' in production")
	}
	if c.Database.Password == "" || c.Database.Password == "deploysentry" {
		errors = append(errors, "DS_DATABASE_PASSWORD must be set to a strong password in production")
	}

	// Check Redis security
	if c.Redis.Password == "" {
		errors = append(errors, "DS_REDIS_PASSWORD should be set in production")
	}

	// Check server configuration
	if c.Server.Host == "localhost" || c.Server.Host == "127.0.0.1" {
		errors = append(errors, "DS_SERVER_HOST should be '0.0.0.0' or a specific interface in production")
	}

	// Check timeouts are reasonable
	if c.Server.ReadTimeout < 5*time.Second {
		errors = append(errors, "DS_SERVER_READ_TIMEOUT should be at least 5s in production")
	}
	if c.Server.WriteTimeout < 5*time.Second {
		errors = append(errors, "DS_SERVER_WRITE_TIMEOUT should be at least 5s in production")
	}

	if len(errors) > 0 {
		return fmt.Errorf("production configuration validation failed:\n- %s",
			strings.Join(errors, "\n- "))
	}

	return nil
}

// setDefaults configures sensible default values for all settings.
func setDefaults(v *viper.Viper) {
	// Server defaults.
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 15*time.Second)
	v.SetDefault("server.write_timeout", 15*time.Second)
	v.SetDefault("server.shutdown_timeout", 10*time.Second)

	// Database defaults.
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "deploysentry")
	v.SetDefault("database.password", "deploysentry")
	v.SetDefault("database.name", "deploysentry")
	v.SetDefault("database.schema", "deploy")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", 5*time.Minute)

	// Redis defaults.
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)

	// NATS defaults.
	v.SetDefault("nats.url", "nats://localhost:4222")
	v.SetDefault("nats.max_reconnects", 10)
	v.SetDefault("nats.reconnect_wait", 2*time.Second)
	v.SetDefault("nats.connect_timeout", 5*time.Second)

	// Auth defaults.
	v.SetDefault("auth.jwt_secret", "change-me-in-production")
	v.SetDefault("auth.jwt_expiration", 30*time.Minute)
	v.SetDefault("auth.session_ttl", 30*time.Minute)

	// Log defaults.
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	// GitHub integration defaults.
	v.SetDefault("github.auto_deploy", false)
	v.SetDefault("github.deploy_branches", []string{"main"})
	v.SetDefault("github.default_strategy", "rolling")

	// Security defaults.
	v.SetDefault("security.encryption_key", "")

	// Health monitor defaults.
	v.SetDefault("health.check_interval_seconds", 30)
	v.SetDefault("health.threshold", 0.95)
	v.SetDefault("health.prometheus.error_rate_query", `rate(http_requests_total{status=~"5.."}[1m])`)
	v.SetDefault("health.prometheus.latency_query", `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[1m]))`)
	v.SetDefault("health.prometheus.error_rate_threshold", 0.02)
	v.SetDefault("health.prometheus.latency_threshold_sec", 0.5)
	v.SetDefault("health.datadog.site", "datadoghq.com")
	v.SetDefault("health.datadog.error_rate_threshold", 0.02)
	v.SetDefault("health.datadog.latency_threshold_sec", 0.5)
	v.SetDefault("health.sentry.base_url", "https://sentry.io/api/0")
	v.SetDefault("health.sentry.error_threshold", 10)

	// Notification defaults (all disabled by default).
	v.SetDefault("notifications.slack.enabled", false)
	v.SetDefault("notifications.slack.username", "DeploySentry")
	v.SetDefault("notifications.email.enabled", false)
	v.SetDefault("notifications.email.smtp_port", 587)
	v.SetDefault("notifications.email.from_name", "DeploySentry")
	v.SetDefault("notifications.pagerduty.enabled", false)
}

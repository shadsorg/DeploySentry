package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DatabaseConfig.DSN
// ---------------------------------------------------------------------------

func TestDatabaseConfig_DSN_ReturnsCorrectFormat(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "db.example.com",
		Port:     5432,
		User:     "admin",
		Password: "secret",
		Name:     "mydb",
		SSLMode:  "require",
	}

	dsn := cfg.DSN()
	expected := "postgres://admin:secret@db.example.com:5432/mydb?sslmode=require"
	assert.Equal(t, expected, dsn)
}

func TestDatabaseConfig_DSN_WithDifferentValues(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5433,
		User:     "deploysentry",
		Password: "deploysentry",
		Name:     "deploysentry",
		SSLMode:  "disable",
	}

	dsn := cfg.DSN()
	expected := "postgres://deploysentry:deploysentry@localhost:5433/deploysentry?sslmode=disable"
	assert.Equal(t, expected, dsn)
}

// ---------------------------------------------------------------------------
// RedisConfig.Addr
// ---------------------------------------------------------------------------

func TestRedisConfig_Addr_ReturnsHostColonPort(t *testing.T) {
	cfg := RedisConfig{
		Host: "redis.example.com",
		Port: 6380,
	}

	addr := cfg.Addr()
	assert.Equal(t, "redis.example.com:6380", addr)
}

func TestRedisConfig_Addr_Localhost(t *testing.T) {
	cfg := RedisConfig{
		Host: "localhost",
		Port: 6379,
	}

	addr := cfg.Addr()
	assert.Equal(t, "localhost:6379", addr)
}

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

func TestLoad_ReturnsConfigWithDefaults(t *testing.T) {
	// Clear any DS_ environment variables that might interfere with defaults.
	envVars := []string{
		"DS_SERVER_PORT",
		"DS_SERVER_HOST",
		"DS_DATABASE_HOST",
		"DS_DATABASE_PORT",
		"DS_DATABASE_USER",
		"DS_DATABASE_PASSWORD",
		"DS_DATABASE_NAME",
		"DS_DATABASE_SSL_MODE",
		"DS_REDIS_HOST",
		"DS_REDIS_PORT",
		"DS_LOG_LEVEL",
		"DS_LOG_FORMAT",
	}
	for _, env := range envVars {
		original, wasSet := os.LookupEnv(env)
		if wasSet {
			os.Unsetenv(env)
			defer os.Setenv(env, original)
		}
	}

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify the config is populated (not zero-value everywhere).
	assert.NotEmpty(t, cfg.Server.Host)
	assert.NotZero(t, cfg.Server.Port)
	assert.NotEmpty(t, cfg.Database.Host)
	assert.NotZero(t, cfg.Database.Port)
	assert.NotEmpty(t, cfg.Redis.Host)
	assert.NotZero(t, cfg.Redis.Port)
}

func TestLoad_DefaultServerPortIs8080(t *testing.T) {
	original, wasSet := os.LookupEnv("DS_SERVER_PORT")
	os.Unsetenv("DS_SERVER_PORT")
	if wasSet {
		defer os.Setenv("DS_SERVER_PORT", original)
	}

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
}

func TestLoad_DefaultDatabaseHostIsLocalhost(t *testing.T) {
	original, wasSet := os.LookupEnv("DS_DATABASE_HOST")
	os.Unsetenv("DS_DATABASE_HOST")
	if wasSet {
		defer os.Setenv("DS_DATABASE_HOST", original)
	}

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "localhost", cfg.Database.Host)
}

func TestLoad_DefaultRedisPortIs6379(t *testing.T) {
	original, wasSet := os.LookupEnv("DS_REDIS_PORT")
	os.Unsetenv("DS_REDIS_PORT")
	if wasSet {
		defer os.Setenv("DS_REDIS_PORT", original)
	}

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 6379, cfg.Redis.Port)
}

func TestLoad_DefaultDatabasePort(t *testing.T) {
	original, wasSet := os.LookupEnv("DS_DATABASE_PORT")
	os.Unsetenv("DS_DATABASE_PORT")
	if wasSet {
		defer os.Setenv("DS_DATABASE_PORT", original)
	}

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 5432, cfg.Database.Port)
}

func TestLoad_DefaultLogLevel(t *testing.T) {
	original, wasSet := os.LookupEnv("DS_LOG_LEVEL")
	os.Unsetenv("DS_LOG_LEVEL")
	if wasSet {
		defer os.Setenv("DS_LOG_LEVEL", original)
	}

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "info", cfg.Log.Level)
}

func TestLoad_DefaultLogFormat(t *testing.T) {
	original, wasSet := os.LookupEnv("DS_LOG_FORMAT")
	os.Unsetenv("DS_LOG_FORMAT")
	if wasSet {
		defer os.Setenv("DS_LOG_FORMAT", original)
	}

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "json", cfg.Log.Format)
}

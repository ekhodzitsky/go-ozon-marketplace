package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 50053, cfg.GRPCPort)
	assert.Equal(t, 51053, cfg.MetricsPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "http://localhost:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "localhost:6379", cfg.RedisAddr)
	assert.Equal(t, 5*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 3*time.Second, cfg.DefaultQueryTimeout)
	assert.Equal(t, "", cfg.CertPath)
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("GRPC_PORT", "8080")
	t.Setenv("METRICS_PORT", "9080")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "console")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel:4318")
	t.Setenv("POSTGRES_DSN", "postgres://db/test")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("JWT_SECRET", "another-super-secret-at-least-32-bytes-long")
	t.Setenv("DEFAULT_CALL_TIMEOUT", "10s")
	t.Setenv("DEFAULT_QUERY_TIMEOUT", "7s")
	t.Setenv("CERT_PATH", "/certs")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.GRPCPort)
	assert.Equal(t, 9080, cfg.MetricsPort)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "console", cfg.LogFormat)
	assert.Equal(t, "http://otel:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "postgres://db/test", cfg.PostgresDSN)
	assert.Equal(t, "redis:6379", cfg.RedisAddr)
	assert.Equal(t, "another-super-secret-at-least-32-bytes-long", cfg.JWTSecret)
	assert.Equal(t, 10*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 7*time.Second, cfg.DefaultQueryTimeout)
	assert.Equal(t, "/certs", cfg.CertPath)
}

func TestLoad_MetricsPortDefaultBasedOnGRPCPort(t *testing.T) {
	t.Setenv("GRPC_PORT", "9090")
	t.Setenv("POSTGRES_DSN", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.GRPCPort)
	assert.Equal(t, 10090, cfg.MetricsPort)
}

func TestLoad_MissingPostgresDSN(t *testing.T) {
	_ = os.Unsetenv("POSTGRES_DSN")
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")

	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://localhost/test")
	_ = os.Unsetenv("JWT_SECRET")

	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_InvalidGRPCPortFallsBack(t *testing.T) {
	t.Setenv("GRPC_PORT", "not-a-number")
	t.Setenv("POSTGRES_DSN", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 50053, cfg.GRPCPort)
}

func TestLoad_InvalidDurationFallsBack(t *testing.T) {
	t.Setenv("DEFAULT_CALL_TIMEOUT", "not-a-duration")
	t.Setenv("DEFAULT_QUERY_TIMEOUT", "not-a-duration")
	t.Setenv("POSTGRES_DSN", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 5*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 3*time.Second, cfg.DefaultQueryTimeout)
}

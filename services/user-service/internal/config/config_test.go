package config_test

import (
	"errors"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "this-is-a-very-long-secret-key-for-tests")
	t.Setenv("POSTGRES_DSN", "postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable")
}

func unsetRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "")
	t.Setenv("POSTGRES_DSN", "")
}

func TestLoad_Defaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 50051, cfg.GRPCPort)
	assert.Equal(t, 8080, cfg.HTTPPort)
	assert.Equal(t, 51051, cfg.MetricsPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "http://localhost:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "", cfg.CertPath)
	assert.Equal(t, 5*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 3*time.Second, cfg.DefaultQueryTimeout)
}

func TestLoad_CustomValues(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GRPC_PORT", "50055")
	t.Setenv("HTTP_PORT", "8085")
	t.Setenv("METRICS_PORT", "70055")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "console")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel:4318")
	t.Setenv("CERT_PATH", "/certs")
	t.Setenv("DEFAULT_CALL_TIMEOUT", "10s")
	t.Setenv("DEFAULT_QUERY_TIMEOUT", "5s")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 50055, cfg.GRPCPort)
	assert.Equal(t, 8085, cfg.HTTPPort)
	assert.Equal(t, 70055, cfg.MetricsPort)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "console", cfg.LogFormat)
	assert.Equal(t, "http://otel:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "/certs", cfg.CertPath)
	assert.Equal(t, 10*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 5*time.Second, cfg.DefaultQueryTimeout)
}

func TestLoad_MetricsPortDefaultsToGRPCPlus1000(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GRPC_PORT", "50060")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 51060, cfg.MetricsPort)
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	unsetRequiredEnv(t)
	t.Setenv("POSTGRES_DSN", "postgres://u:p@localhost/db?sslmode=disable")

	_, err := config.Load()
	require.Error(t, err)
	assert.True(t, errors.Is(err, config.ErrMissingJWTSecret))
}

func TestLoad_ShortJWTSecret(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("JWT_SECRET", "short")

	_, err := config.Load()
	require.Error(t, err)
	assert.True(t, errors.Is(err, config.ErrJWTSecretTooShort))
}

func TestLoad_MissingPostgresDSN(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("POSTGRES_DSN", "")

	_, err := config.Load()
	require.Error(t, err)
	assert.True(t, errors.Is(err, config.ErrMissingPostgresDSN))
}

func TestLoad_InvalidPostgresDSN(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("POSTGRES_DSN", "not-a-valid-dsn")

	_, err := config.Load()
	require.Error(t, err)
	assert.True(t, errors.Is(err, config.ErrInvalidPostgresDSN))
}

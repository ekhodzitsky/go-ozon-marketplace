package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 50052, cfg.GRPCPort)
	assert.Equal(t, 51052, cfg.MetricsPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "http://localhost:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "postgres://user:pass@localhost:5432/db?sslmode=disable", cfg.PostgresDSN)
	assert.Equal(t, "http://localhost:9200", cfg.ESURL)
	assert.Equal(t, "01234567890123456789012345678901", cfg.JWTSecret)
	assert.Equal(t, 5*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 3*time.Second, cfg.DefaultQueryTimeout)
	assert.Empty(t, cfg.CertPath)
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://ozon:ozonpass@db:5432/marketplace?sslmode=disable")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("GRPC_PORT", "60052")
	t.Setenv("METRICS_PORT", "61052")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "console")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel:4318")
	t.Setenv("ES_URL", "http://es:9200")
	t.Setenv("DEFAULT_CALL_TIMEOUT", "10s")
	t.Setenv("DEFAULT_QUERY_TIMEOUT", "7s")
	t.Setenv("CERT_PATH", "/certs")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 60052, cfg.GRPCPort)
	assert.Equal(t, 61052, cfg.MetricsPort)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "console", cfg.LogFormat)
	assert.Equal(t, "http://otel:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "postgres://ozon:ozonpass@db:5432/marketplace?sslmode=disable", cfg.PostgresDSN)
	assert.Equal(t, "http://es:9200", cfg.ESURL)
	assert.Equal(t, 10*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 7*time.Second, cfg.DefaultQueryTimeout)
	assert.Equal(t, "/certs", cfg.CertPath)
}

func TestLoad_MetricsPortDefaultFromGRPC(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("GRPC_PORT", "7000")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 8000, cfg.MetricsPort)
}

func TestLoad_MissingPostgresDSN(t *testing.T) {
	_ = os.Unsetenv("POSTGRES_DSN")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")

	_, err := config.Load()
	require.Error(t, err)
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	_ = os.Unsetenv("JWT_SECRET")

	_, err := config.Load()
	require.Error(t, err)
}

func TestLoad_ShortJWTSecret(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "short")

	_, err := config.Load()
	require.Error(t, err)
}

func TestLoad_InvalidDSN(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "not-a-valid-dsn")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")

	_, err := config.Load()
	require.Error(t, err)
}

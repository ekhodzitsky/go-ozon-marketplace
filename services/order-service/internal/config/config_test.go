package config_test

import (
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost/db")
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")
}

func TestLoad_Defaults(t *testing.T) {
	setRequired(t)

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 50055, cfg.GRPCPort)
	assert.Equal(t, 51055, cfg.MetricsPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "http://localhost:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "localhost:50053", cfg.InventoryAddr)
	assert.Equal(t, "localhost:50054", cfg.PaymentAddr)
	assert.Equal(t, 5*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 3*time.Second, cfg.DefaultQueryTimeout)
	assert.Equal(t, "", cfg.CertPath)
	assert.Equal(t, []string{"localhost:9092"}, cfg.KafkaBrokers)
	assert.Equal(t, "order-events", cfg.KafkaTopic)
	assert.Equal(t, "localhost:6379", cfg.RedisAddr)
}

func TestLoad_Overrides(t *testing.T) {
	setRequired(t)
	t.Setenv("GRPC_PORT", "8080")
	t.Setenv("METRICS_PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "console")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
	t.Setenv("INVENTORY_ADDR", "inventory:50053")
	t.Setenv("PAYMENT_ADDR", "payment:50054")
	t.Setenv("DEFAULT_CALL_TIMEOUT", "10s")
	t.Setenv("DEFAULT_QUERY_TIMEOUT", "7s")
	t.Setenv("CERT_PATH", "/certs")
	t.Setenv("KAFKA_BROKERS", "kafka1:9092,kafka2:9092")
	t.Setenv("KAFKA_TOPIC", "events")
	t.Setenv("REDIS_ADDR", "redis:6379")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.GRPCPort)
	assert.Equal(t, 9090, cfg.MetricsPort)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "console", cfg.LogFormat)
	assert.Equal(t, "http://collector:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "inventory:50053", cfg.InventoryAddr)
	assert.Equal(t, "payment:50054", cfg.PaymentAddr)
	assert.Equal(t, 10*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 7*time.Second, cfg.DefaultQueryTimeout)
	assert.Equal(t, "/certs", cfg.CertPath)
	assert.Equal(t, []string{"kafka1:9092", "kafka2:9092"}, cfg.KafkaBrokers)
	assert.Equal(t, "events", cfg.KafkaTopic)
	assert.Equal(t, "redis:6379", cfg.RedisAddr)
}

func TestLoad_MetricsPortDerivedFromGRPCPort(t *testing.T) {
	setRequired(t)
	t.Setenv("GRPC_PORT", "60055")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 60055, cfg.GRPCPort)
	assert.Equal(t, 61055, cfg.MetricsPort)
}

func TestLoad_MissingPostgresDSN(t *testing.T) {
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")

	_, err := config.Load()
	require.Error(t, err)
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost/db")

	_, err := config.Load()
	require.Error(t, err)
}

func TestLoad_InvalidIntegerFallsBackToDefault(t *testing.T) {
	setRequired(t)
	t.Setenv("GRPC_PORT", "not-a-number")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 50055, cfg.GRPCPort)
	assert.Equal(t, 51055, cfg.MetricsPort)
}

func TestLoad_InvalidDurationFallsBackToDefault(t *testing.T) {
	setRequired(t)
	t.Setenv("DEFAULT_CALL_TIMEOUT", "not-a-duration")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 5*time.Second, cfg.DefaultCallTimeout)
}

func TestLoad_EmptySliceFallsBackToDefault(t *testing.T) {
	setRequired(t)

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, []string{"localhost:9092"}, cfg.KafkaBrokers)
}

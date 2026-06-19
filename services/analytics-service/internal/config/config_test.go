package config_test

import (
	"errors"
	"os"
	"testing"
	"time"

	sharedconfig "github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("JWT_SECRET", "valid-secret-key-32-bytes-long!!")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 50057, cfg.GRPCPort)
	assert.Equal(t, 51057, cfg.MetricsPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "http://localhost:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "localhost:9000", cfg.ClickHouseAddr)
	assert.Empty(t, cfg.ClickHouseUser)
	assert.Empty(t, cfg.ClickHousePassword)
	assert.Equal(t, "valid-secret-key-32-bytes-long!!", cfg.JWTSecret)
	assert.Equal(t, 5*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 3*time.Second, cfg.DefaultQueryTimeout)
	assert.Empty(t, cfg.CertPath)
	assert.Equal(t, []string{"localhost:9092"}, cfg.KafkaBrokers)
	assert.Equal(t, "analytics-service", cfg.KafkaConsumerGroup)
	assert.Equal(t, []string{"order-events"}, cfg.KafkaTopics)
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("GRPC_PORT", "60057")
	t.Setenv("METRICS_PORT", "61057")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "console")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
	t.Setenv("CLICKHOUSE_DSN", "clickhouse:9000")
	t.Setenv("CLICKHOUSE_USER", "default")
	t.Setenv("CLICKHOUSE_PASSWORD", "secret")
	t.Setenv("JWT_SECRET", "valid-secret-key-32-bytes-long!!")
	t.Setenv("DEFAULT_CALL_TIMEOUT", "10s")
	t.Setenv("DEFAULT_QUERY_TIMEOUT", "7s")
	t.Setenv("CERT_PATH", "/certs")
	t.Setenv("KAFKA_BROKERS", "kafka1:9092,kafka2:9092")
	t.Setenv("KAFKA_CONSUMER_GROUP", "analytics-test")
	t.Setenv("KAFKA_TOPICS", "order-events,payment-events")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 60057, cfg.GRPCPort)
	assert.Equal(t, 61057, cfg.MetricsPort)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "console", cfg.LogFormat)
	assert.Equal(t, "http://collector:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "clickhouse:9000", cfg.ClickHouseAddr)
	assert.Equal(t, "default", cfg.ClickHouseUser)
	assert.Equal(t, "secret", cfg.ClickHousePassword)
	assert.Equal(t, 10*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 7*time.Second, cfg.DefaultQueryTimeout)
	assert.Equal(t, "/certs", cfg.CertPath)
	assert.Equal(t, []string{"kafka1:9092", "kafka2:9092"}, cfg.KafkaBrokers)
	assert.Equal(t, "analytics-test", cfg.KafkaConsumerGroup)
	assert.Equal(t, []string{"order-events", "payment-events"}, cfg.KafkaTopics)
}

func TestLoad_MetricsPortDerivedFromGRPCPort(t *testing.T) {
	t.Setenv("GRPC_PORT", "8080")
	t.Setenv("JWT_SECRET", "valid-secret-key-32-bytes-long!!")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.GRPCPort)
	assert.Equal(t, 9080, cfg.MetricsPort)
}

func TestLoad_ValidationFailures(t *testing.T) {
	// Preserve and restore the original JWT_SECRET so parallel tests in other
	// packages are not affected, then clear it for this test.
	origJWT := os.Getenv("JWT_SECRET")
	defer func() { _ = os.Setenv("JWT_SECRET", origJWT) }()
	require.NoError(t, os.Unsetenv("JWT_SECRET"))

	tests := []struct {
		name    string
		setup   func()
		wantErr error
	}{
		{
			name:    "missing jwt secret",
			setup:   func() {},
			wantErr: config.ErrMissingJWTSecret,
		},
		{
			name: "short jwt secret",
			setup: func() {
				t.Setenv("JWT_SECRET", "short")
			},
			wantErr: config.ErrJWTSecretTooShort,
		},
		{
			name: "invalid clickhouse dsn",
			setup: func() {
				t.Setenv("JWT_SECRET", "valid-secret-key-32-bytes-long!!")
				t.Setenv("CLICKHOUSE_DSN", "not-a-dsn")
			},
			wantErr: config.ErrInvalidClickHouseDSN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.Unsetenv("JWT_SECRET"))
			require.NoError(t, os.Unsetenv("CLICKHOUSE_DSN"))
			tt.setup()

			_, err := config.Load()
			require.Error(t, err)
			assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	valid := &config.Config{
		Base: sharedconfig.Base{
			JWTSecret: "valid-secret-key-32-bytes-long!!",
		},
		ClickHouseAddr: "localhost:9000",
		ClickHouseUser: "default",
		KafkaBrokers:   []string{"localhost:9092"},
		KafkaTopics:    []string{"order-events"},
	}
	require.NoError(t, valid.Validate())

	short := *valid
	short.JWTSecret = "short"
	assert.ErrorIs(t, short.Validate(), config.ErrJWTSecretTooShort)

	badDSN := *valid
	badDSN.ClickHouseAddr = "nocolon"
	assert.ErrorIs(t, badDSN.Validate(), config.ErrInvalidClickHouseDSN)
}

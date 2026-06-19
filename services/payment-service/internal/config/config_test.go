package config_test

import (
	"os"
	"testing"
	"time"

	pkgconfig "github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unsetRequiredEnv(t *testing.T) {
	t.Helper()
	_ = os.Unsetenv("POSTGRES_DSN")
	_ = os.Unsetenv("JWT_SECRET")
	t.Cleanup(func() {
		_ = os.Unsetenv("POSTGRES_DSN")
		_ = os.Unsetenv("JWT_SECRET")
	})
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "long-enough-secret")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 50054, cfg.GRPCPort)
	assert.Equal(t, 51054, cfg.MetricsPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "http://localhost:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, []string{"localhost:9092"}, cfg.KafkaBrokers)
	assert.Equal(t, "payment-dlq", cfg.DLQTopic)
	assert.Empty(t, cfg.CertPath)
	assert.Equal(t, 5*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 3*time.Second, cfg.DefaultQueryTimeout)
}

func TestLoad_OverrideDefaults(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "long-enough-secret")
	t.Setenv("GRPC_PORT", "60054")
	t.Setenv("METRICS_PORT", "61054")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "console")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://jaeger:4318")
	t.Setenv("DEFAULT_CALL_TIMEOUT", "10s")
	t.Setenv("DEFAULT_QUERY_TIMEOUT", "7s")
	t.Setenv("CERT_PATH", "/certs")
	t.Setenv("KAFKA_BROKERS", "kafka-1:9092,kafka-2:9092")
	t.Setenv("DLQ_TOPIC", "custom-dlq")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 60054, cfg.GRPCPort)
	assert.Equal(t, 61054, cfg.MetricsPort)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "console", cfg.LogFormat)
	assert.Equal(t, "http://jaeger:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, 10*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 7*time.Second, cfg.DefaultQueryTimeout)
	assert.Equal(t, "/certs", cfg.CertPath)
	assert.Equal(t, []string{"kafka-1:9092", "kafka-2:9092"}, cfg.KafkaBrokers)
	assert.Equal(t, "custom-dlq", cfg.DLQTopic)
}

func TestLoad_MissingRequiredEnv(t *testing.T) {
	unsetRequiredEnv(t)
	_, err := config.Load()
	require.Error(t, err)
}

func TestConfig_Validate(t *testing.T) {
	valid := &config.Config{
		Base: pkgconfig.Base{
			JWTSecret:           "long-enough-secret",
			DefaultCallTimeout:  5 * time.Second,
			DefaultQueryTimeout: 3 * time.Second,
		},
		ServerBase: pkgconfig.ServerBase{
			GRPCPort:    50054,
			MetricsPort: 51054,
		},
		PostgresDSN:  "postgres://user:pass@localhost:5432/db?sslmode=disable",
		KafkaBrokers: []string{"localhost:9092"},
		DLQTopic:     "payment-dlq",
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name    string
		mutate  func(*config.Config)
		wantErr string
	}{
		{
			name: "short_jwt_secret",
			mutate: func(c *config.Config) {
				c.JWTSecret = "short"
			},
			wantErr: "jwt secret must be at least",
		},
		{
			name: "empty_postgres_dsn",
			mutate: func(c *config.Config) {
				c.PostgresDSN = ""
			},
			wantErr: "postgres dsn is required",
		},
		{
			name: "invalid_postgres_dsn_prefix",
			mutate: func(c *config.Config) {
				c.PostgresDSN = "mysql://localhost/db"
			},
			wantErr: "postgres dsn must start with",
		},
		{
			name: "zero_grpc_port",
			mutate: func(c *config.Config) {
				c.GRPCPort = 0
			},
			wantErr: "grpc port must be greater than 0",
		},
		{
			name: "zero_metrics_port",
			mutate: func(c *config.Config) {
				c.MetricsPort = 0
			},
			wantErr: "metrics port must be greater than 0",
		},
		{
			name: "zero_call_timeout",
			mutate: func(c *config.Config) {
				c.DefaultCallTimeout = 0
			},
			wantErr: "default call timeout must be greater than 0",
		},
		{
			name: "zero_query_timeout",
			mutate: func(c *config.Config) {
				c.DefaultQueryTimeout = 0
			},
			wantErr: "default query timeout must be greater than 0",
		},
		{
			name: "empty_kafka_brokers",
			mutate: func(c *config.Config) {
				c.KafkaBrokers = nil
			},
			wantErr: "at least one kafka broker is required",
		},
		{
			name: "empty_dlq_topic",
			mutate: func(c *config.Config) {
				c.DLQTopic = "   "
			},
			wantErr: "dlq topic is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := *valid
			tt.mutate(&cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

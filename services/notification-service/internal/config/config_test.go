package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("JWT_SECRET", "very-secret-key")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 50056, cfg.GRPCPort)
	assert.Equal(t, 51056, cfg.MetricsPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "http://localhost:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "very-secret-key", cfg.JWTSecret)
	assert.Equal(t, 5*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 3*time.Second, cfg.DefaultQueryTimeout)
	assert.Equal(t, []string{"localhost:9092"}, cfg.KafkaBrokers)
	assert.Equal(t, "notification-service", cfg.KafkaConsumerGroup)
	assert.Equal(t, []string{"order-events"}, cfg.KafkaTopics)
	assert.Equal(t, "notification-dlq", cfg.KafkaDLQTopic)
	assert.Empty(t, cfg.SMTPHost)
	assert.Equal(t, 587, cfg.SMTPPort)
	assert.Equal(t, "notifications@example.com", cfg.SMTPFrom)
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("JWT_SECRET", "another-secret")
	t.Setenv("GRPC_PORT", "60056")
	t.Setenv("METRICS_PORT", "61056")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "console")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel:4318")
	t.Setenv("DEFAULT_CALL_TIMEOUT", "10s")
	t.Setenv("DEFAULT_QUERY_TIMEOUT", "7s")
	t.Setenv("KAFKA_BROKERS", "kafka1:9092,kafka2:9092")
	t.Setenv("KAFKA_TOPICS", "order-events,payment-events")
	t.Setenv("KAFKA_CONSUMER_GROUP", "notification-test")
	t.Setenv("KAFKA_DLQ_TOPIC", "test-dlq")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_FROM", "test@example.com")
	t.Setenv("SMTP_USER", "user")
	t.Setenv("SMTP_PASSWORD", "pass")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 60056, cfg.GRPCPort)
	assert.Equal(t, 61056, cfg.MetricsPort)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "console", cfg.LogFormat)
	assert.Equal(t, "http://otel:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "another-secret", cfg.JWTSecret)
	assert.Equal(t, 10*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 7*time.Second, cfg.DefaultQueryTimeout)
	assert.Equal(t, []string{"kafka1:9092", "kafka2:9092"}, cfg.KafkaBrokers)
	assert.Equal(t, []string{"order-events", "payment-events"}, cfg.KafkaTopics)
	assert.Equal(t, "notification-test", cfg.KafkaConsumerGroup)
	assert.Equal(t, "test-dlq", cfg.KafkaDLQTopic)
	assert.Equal(t, "smtp.example.com", cfg.SMTPHost)
	assert.Equal(t, 465, cfg.SMTPPort)
	assert.Equal(t, "test@example.com", cfg.SMTPFrom)
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	require.NoError(t, os.Unsetenv("JWT_SECRET"))

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoad_ShortJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoad_InvalidCallTimeout(t *testing.T) {
	t.Setenv("JWT_SECRET", "very-secret-key")
	t.Setenv("DEFAULT_CALL_TIMEOUT", "0s")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DEFAULT_CALL_TIMEOUT")
}

func TestLoad_InvalidQueryTimeout(t *testing.T) {
	t.Setenv("JWT_SECRET", "very-secret-key")
	t.Setenv("DEFAULT_QUERY_TIMEOUT", "-1s")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DEFAULT_QUERY_TIMEOUT")
}

func TestLoad_InvalidSMTPPort(t *testing.T) {
	t.Setenv("JWT_SECRET", "very-secret-key")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "70000")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP_PORT")
}

func TestLoad_InvalidSMTPFrom(t *testing.T) {
	t.Setenv("JWT_SECRET", "very-secret-key")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM", "not-an-email")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP_FROM")
}

func TestConfig_Validate(t *testing.T) {
	cfg := &Config{
		JWTSecret:           "secret-key",
		KafkaBrokers:        []string{"localhost:9092"},
		KafkaTopics:         []string{"order-events"},
		DefaultCallTimeout:  5 * time.Second,
		DefaultQueryTimeout: 3 * time.Second,
		SMTPHost:            "",
		SMTPPort:            587,
		SMTPFrom:            "notifications@example.com",
	}
	require.NoError(t, cfg.Validate())
}

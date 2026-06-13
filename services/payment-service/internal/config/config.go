package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort                 int
	MetricsPort              int
	LogLevel                 string
	LogFormat                string
	OTELExporterOTLPEndpoint string
	PostgresDSN              string
	JWTSecret                string
	DefaultCallTimeout       time.Duration
	DefaultQueryTimeout      time.Duration
	CertPath                 string
	KafkaBrokers             []string
	DLQTopic                 string
}

// minJWTSecretLen is the minimum acceptable length for the JWT signing secret.
const minJWTSecretLen = 8

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50054)
	cfg := &Config{
		GRPCPort:                 grpcPort,
		MetricsPort:              config.GetEnvInt("METRICS_PORT", grpcPort+1000),
		LogLevel:                 config.GetEnv("LOG_LEVEL", "info"),
		LogFormat:                config.GetEnv("LOG_FORMAT", "json"),
		OTELExporterOTLPEndpoint: config.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		PostgresDSN:              config.GetEnv("POSTGRES_DSN", ""),
		JWTSecret:                config.GetEnv("JWT_SECRET", ""),
		DefaultCallTimeout:       config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout:      config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
		CertPath:                 config.GetEnv("CERT_PATH", ""),
		KafkaBrokers:             config.GetEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		DLQTopic:                 config.GetEnv("DLQ_TOPIC", "payment-dlq"),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks that all required configuration values are present and well-formed.
func (c *Config) Validate() error {
	if c.GRPCPort <= 0 {
		return errors.New("grpc port must be greater than 0")
	}
	if c.MetricsPort <= 0 {
		return errors.New("metrics port must be greater than 0")
	}
	if strings.TrimSpace(c.PostgresDSN) == "" {
		return errors.New("postgres dsn is required")
	}
	if !strings.HasPrefix(c.PostgresDSN, "postgres://") && !strings.HasPrefix(c.PostgresDSN, "postgresql://") {
		return fmt.Errorf("postgres dsn must start with postgres:// or postgresql://, got %q", c.PostgresDSN)
	}
	if len(c.JWTSecret) < minJWTSecretLen {
		return fmt.Errorf("jwt secret must be at least %d characters long", minJWTSecretLen)
	}
	if c.DefaultCallTimeout <= 0 {
		return errors.New("default call timeout must be greater than 0")
	}
	if c.DefaultQueryTimeout <= 0 {
		return errors.New("default query timeout must be greater than 0")
	}
	if len(c.KafkaBrokers) == 0 {
		return errors.New("at least one kafka broker is required")
	}
	if strings.TrimSpace(c.DLQTopic) == "" {
		return errors.New("dlq topic is required")
	}
	return nil
}

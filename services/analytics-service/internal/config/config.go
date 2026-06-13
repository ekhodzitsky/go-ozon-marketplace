package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

var (
	ErrMissingJWTSecret     = errors.New("JWT_SECRET is required")
	ErrJWTSecretTooShort    = errors.New("JWT_SECRET must be at least 32 characters")
	ErrMissingClickHouseDSN = errors.New("CLICKHOUSE_DSN is required")
	ErrInvalidClickHouseDSN = errors.New("CLICKHOUSE_DSN must be in host:port form")
	ErrMissingKafkaBrokers  = errors.New("KAFKA_BROKERS is required")
	ErrMissingKafkaTopics   = errors.New("KAFKA_TOPICS is required")
)

type Config struct {
	GRPCPort                 int
	MetricsPort              int
	LogLevel                 string
	LogFormat                string
	OTELExporterOTLPEndpoint string
	ClickHouseAddr           string
	ClickHouseUser           string
	ClickHousePassword       string
	JWTSecret                string
	DefaultCallTimeout       time.Duration
	DefaultQueryTimeout      time.Duration
	CertPath                 string
	KafkaBrokers             []string
	KafkaConsumerGroup       string
	KafkaTopics              []string
}

func Load() (*Config, error) {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50057)
	cfg := &Config{
		GRPCPort:                 grpcPort,
		MetricsPort:              config.GetEnvInt("METRICS_PORT", grpcPort+1000),
		LogLevel:                 config.GetEnv("LOG_LEVEL", "info"),
		LogFormat:                config.GetEnv("LOG_FORMAT", "json"),
		OTELExporterOTLPEndpoint: config.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		ClickHouseAddr:           config.GetEnv("CLICKHOUSE_DSN", "localhost:9000"),
		ClickHouseUser:           config.GetEnv("CLICKHOUSE_USER", ""),
		ClickHousePassword:       config.GetEnv("CLICKHOUSE_PASSWORD", ""),
		JWTSecret:                config.GetEnv("JWT_SECRET", ""),
		DefaultCallTimeout:       config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout:      config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
		CertPath:                 config.GetEnv("CERT_PATH", ""),
		KafkaBrokers:             config.GetEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		KafkaConsumerGroup:       config.GetEnv("KAFKA_CONSUMER_GROUP", "analytics-service"),
		KafkaTopics:              config.GetEnvSlice("KAFKA_TOPICS", []string{"order-events"}),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return ErrMissingJWTSecret
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("%w: got %d bytes", ErrJWTSecretTooShort, len(c.JWTSecret))
	}
	if c.ClickHouseAddr == "" {
		return ErrMissingClickHouseDSN
	}
	if !strings.Contains(c.ClickHouseAddr, ":") {
		return ErrInvalidClickHouseDSN
	}
	if len(c.KafkaBrokers) == 0 {
		return ErrMissingKafkaBrokers
	}
	if len(c.KafkaTopics) == 0 {
		return ErrMissingKafkaTopics
	}
	return nil
}

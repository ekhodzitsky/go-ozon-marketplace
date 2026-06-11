package config

import (
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

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50054)
	return &Config{
		GRPCPort:                 grpcPort,
		MetricsPort:              config.GetEnvInt("METRICS_PORT", grpcPort+1000),
		LogLevel:                 config.GetEnv("LOG_LEVEL", "info"),
		LogFormat:                config.GetEnv("LOG_FORMAT", "json"),
		OTELExporterOTLPEndpoint: config.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		PostgresDSN:              config.MustGetEnv("POSTGRES_DSN"),
		JWTSecret:                config.MustGetEnv("JWT_SECRET"),
		DefaultCallTimeout:       config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout:      config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
		CertPath:                 config.GetEnv("CERT_PATH", ""),
		KafkaBrokers:             config.GetEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		DLQTopic:                 config.GetEnv("DLQ_TOPIC", "payment-dlq"),
	}
}

package config

import (
	"fmt"
	"net/mail"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort                 int
	MetricsPort              int
	LogLevel                 string
	LogFormat                string
	OTELExporterOTLPEndpoint string
	JWTSecret                string
	DefaultCallTimeout       time.Duration
	DefaultQueryTimeout      time.Duration
	CertPath                 string
	KafkaBrokers             []string
	KafkaConsumerGroup       string
	KafkaTopics              []string
	KafkaDLQTopic            string
	SMTPHost                 string
	SMTPPort                 int
	SMTPFrom                 string
	SMTPUser                 string
	SMTPPassword             string
}

func Load() (*Config, error) {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50056)
	cfg := &Config{
		GRPCPort:                 grpcPort,
		MetricsPort:              config.GetEnvInt("METRICS_PORT", grpcPort+1000),
		LogLevel:                 config.GetEnv("LOG_LEVEL", "info"),
		LogFormat:                config.GetEnv("LOG_FORMAT", "json"),
		OTELExporterOTLPEndpoint: config.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		JWTSecret:                config.GetEnv("JWT_SECRET", ""),
		DefaultCallTimeout:       config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout:      config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
		CertPath:                 config.GetEnv("CERT_PATH", ""),
		KafkaBrokers:             config.GetEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		KafkaConsumerGroup:       config.GetEnv("KAFKA_CONSUMER_GROUP", "notification-service"),
		KafkaTopics:              config.GetEnvSlice("KAFKA_TOPICS", []string{"order-events"}),
		KafkaDLQTopic:            config.GetEnv("KAFKA_DLQ_TOPIC", "notification-dlq"),
		SMTPHost:                 config.GetEnv("SMTP_HOST", ""),
		SMTPPort:                 config.GetEnvInt("SMTP_PORT", 587),
		SMTPFrom:                 config.GetEnv("SMTP_FROM", "notifications@example.com"),
		SMTPUser:                 config.GetEnv("SMTP_USER", ""),
		SMTPPassword:             config.GetEnv("SMTP_PASSWORD", ""),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if len(c.JWTSecret) < 8 {
		return fmt.Errorf("JWT_SECRET must be at least 8 characters")
	}
	if len(c.KafkaBrokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS must not be empty")
	}
	if len(c.KafkaTopics) == 0 {
		return fmt.Errorf("KAFKA_TOPICS must not be empty")
	}
	if c.DefaultCallTimeout <= 0 {
		return fmt.Errorf("DEFAULT_CALL_TIMEOUT must be positive")
	}
	if c.DefaultQueryTimeout <= 0 {
		return fmt.Errorf("DEFAULT_QUERY_TIMEOUT must be positive")
	}
	if c.SMTPHost != "" {
		if c.SMTPPort <= 0 || c.SMTPPort > 65535 {
			return fmt.Errorf("SMTP_PORT must be a valid port number")
		}
		if _, err := mail.ParseAddress(c.SMTPFrom); err != nil {
			return fmt.Errorf("SMTP_FROM must be a valid email address")
		}
	}
	return nil
}

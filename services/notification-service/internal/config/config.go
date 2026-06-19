package config

import (
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	config.Base
	config.ServerBase

	KafkaBrokers       []string
	KafkaConsumerGroup string
	KafkaTopics        []string
	KafkaDLQTopic      string
	SMTPHost           string
	SMTPPort           int
	SMTPFrom           string
	SMTPUser           string
	SMTPPassword       string
}

func Load() (*Config, error) {
	base := config.LoadBase()
	serverBase := config.LoadServerBase(50056)

	cfg := &Config{
		Base:       base,
		ServerBase: serverBase,

		KafkaBrokers:       config.GetEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		KafkaConsumerGroup: config.GetEnv("KAFKA_CONSUMER_GROUP", "notification-service"),
		KafkaTopics:        config.GetEnvSlice("KAFKA_TOPICS", []string{"order-events"}),
		KafkaDLQTopic:      config.GetEnv("KAFKA_DLQ_TOPIC", "notification-dlq"),
		SMTPHost:           config.GetEnv("SMTP_HOST", ""),
		SMTPPort:           config.GetEnvInt("SMTP_PORT", 587),
		SMTPFrom:           config.GetEnv("SMTP_FROM", "notifications@example.com"),
		SMTPUser:           config.GetEnv("SMTP_USER", ""),
		SMTPPassword:       config.GetEnv("SMTP_PASSWORD", ""),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if err := config.ValidateJWTSecret(c.JWTSecret, 8); err != nil {
		return err
	}
	if err := config.ValidateKafkaBrokers(c.KafkaBrokers); err != nil {
		return err
	}
	if err := config.ValidateKafkaTopics(c.KafkaTopics); err != nil {
		return err
	}
	if c.DefaultCallTimeout <= 0 {
		return fmt.Errorf("DEFAULT_CALL_TIMEOUT must be positive")
	}
	if c.DefaultQueryTimeout <= 0 {
		return fmt.Errorf("DEFAULT_QUERY_TIMEOUT must be positive")
	}
	if err := config.ValidateSMTP(c.SMTPHost, c.SMTPPort, c.SMTPFrom); err != nil {
		return err
	}
	return nil
}

func (c *Config) GetKafkaBrokers() []string             { return c.KafkaBrokers }
func (c *Config) GetKafkaConsumerGroup() string         { return c.KafkaConsumerGroup }
func (c *Config) GetKafkaTopics() []string              { return c.KafkaTopics }
func (c *Config) GetKafkaDLQTopic() string              { return c.KafkaDLQTopic }
func (c *Config) GetKafkaMaxRetries() int               { return 2 }
func (c *Config) GetKafkaInitialBackoff() time.Duration { return 50 * time.Millisecond }
func (c *Config) GetKafkaProcessTimeout() time.Duration { return 10 * time.Second }

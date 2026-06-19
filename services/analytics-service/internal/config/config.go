package config

import (
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

var (
	ErrMissingJWTSecret     = config.ErrMissingJWTSecret
	ErrJWTSecretTooShort    = config.ErrJWTSecretTooShort
	ErrMissingClickHouseDSN = config.ErrMissingClickHouseDSN
	ErrInvalidClickHouseDSN = config.ErrInvalidClickHouseDSN
	ErrMissingKafkaBrokers  = config.ErrMissingKafkaBrokers
	ErrMissingKafkaTopics   = config.ErrMissingKafkaTopics
)

type Config struct {
	config.Base
	config.ServerBase

	ClickHouseAddr     string
	ClickHouseUser     string
	ClickHousePassword string
	KafkaBrokers       []string
	KafkaConsumerGroup string
	KafkaTopics        []string
}

func Load() (*Config, error) {
	base := config.LoadBase()
	serverBase := config.LoadServerBase(50057)

	cfg := &Config{
		Base:               base,
		ServerBase:         serverBase,
		ClickHouseAddr:     config.GetEnv("CLICKHOUSE_DSN", "localhost:9000"),
		ClickHouseUser:     config.GetEnv("CLICKHOUSE_USER", ""),
		ClickHousePassword: config.GetEnv("CLICKHOUSE_PASSWORD", ""),
		KafkaBrokers:       config.GetEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		KafkaConsumerGroup: config.GetEnv("KAFKA_CONSUMER_GROUP", "analytics-service"),
		KafkaTopics:        config.GetEnvSlice("KAFKA_TOPICS", []string{"order-events"}),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if err := config.ValidateJWTSecret(c.JWTSecret, 32); err != nil {
		return err
	}
	if err := config.ValidateClickHouseAddr(c.ClickHouseAddr); err != nil {
		return err
	}
	if err := config.ValidateKafkaBrokers(c.KafkaBrokers); err != nil {
		return err
	}
	if err := config.ValidateKafkaTopics(c.KafkaTopics); err != nil {
		return err
	}
	return nil
}

func (c *Config) GetKafkaBrokers() []string             { return c.KafkaBrokers }
func (c *Config) GetKafkaConsumerGroup() string         { return c.KafkaConsumerGroup }
func (c *Config) GetKafkaTopics() []string              { return c.KafkaTopics }
func (c *Config) GetKafkaDLQTopic() string              { return "" }
func (c *Config) GetKafkaMaxRetries() int               { return 3 }
func (c *Config) GetKafkaInitialBackoff() time.Duration { return 100 * time.Millisecond }
func (c *Config) GetKafkaProcessTimeout() time.Duration { return 10 * time.Second }

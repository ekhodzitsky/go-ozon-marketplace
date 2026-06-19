package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	config.Base
	config.ServerBase

	PostgresDSN  string
	KafkaBrokers []string
	DLQTopic     string
}

// minJWTSecretLen is the minimum acceptable length for the JWT signing secret.
const minJWTSecretLen = 8

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	base := config.LoadBase()
	serverBase := config.LoadServerBase(50054)
	cfg := &Config{
		Base:       base,
		ServerBase: serverBase,

		PostgresDSN:  config.GetEnv("POSTGRES_DSN", ""),
		KafkaBrokers: config.GetEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		DLQTopic:     config.GetEnv("DLQ_TOPIC", "payment-dlq"),
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
	if err := config.ValidatePostgresDSN(c.PostgresDSN); err != nil {
		if errors.Is(err, config.ErrMissingPostgresDSN) {
			return fmt.Errorf("postgres dsn is required: %w", config.ErrMissingPostgresDSN)
		}
		return fmt.Errorf("postgres dsn must start with postgres:// or postgresql://, got %q: %w", c.PostgresDSN, config.ErrInvalidPostgresDSN)
	}
	if err := config.ValidateJWTSecret(c.JWTSecret, minJWTSecretLen); err != nil {
		return fmt.Errorf("jwt secret must be at least %d characters long: %w", minJWTSecretLen, err)
	}
	if c.DefaultCallTimeout <= 0 {
		return errors.New("default call timeout must be greater than 0")
	}
	if c.DefaultQueryTimeout <= 0 {
		return errors.New("default query timeout must be greater than 0")
	}
	if err := config.ValidateKafkaBrokers(c.KafkaBrokers); err != nil {
		return fmt.Errorf("at least one kafka broker is required: %w", config.ErrMissingKafkaBrokers)
	}
	if strings.TrimSpace(c.DLQTopic) == "" {
		return errors.New("dlq topic is required")
	}
	return nil
}

func (c *Config) GetPostgresDSN() string    { return c.PostgresDSN }
func (c *Config) GetKafkaBrokers() []string { return c.KafkaBrokers }

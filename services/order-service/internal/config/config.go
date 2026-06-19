package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	config.Base
	config.ServerBase

	PostgresDSN   string
	InventoryAddr string
	PaymentAddr   string
	CatalogAddr   string
	KafkaBrokers  []string
	KafkaTopic    string
	RedisAddr     string
}

func Load() (*Config, error) {
	base := config.LoadBase()
	if err := config.ValidateJWTSecret(base.JWTSecret, 32); err != nil {
		return nil, err
	}

	serverBase := config.LoadServerBase(50055)

	postgresDSN := config.GetEnv("POSTGRES_DSN", "")
	if err := config.ValidatePostgresDSN(postgresDSN); err != nil {
		return nil, err
	}

	kafkaBrokers := config.GetEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"})
	if err := config.ValidateKafkaBrokers(kafkaBrokers); err != nil {
		return nil, err
	}

	kafkaTopic := config.GetEnv("KAFKA_TOPIC", "order-events")
	if err := config.ValidateKafkaTopics([]string{kafkaTopic}); err != nil {
		return nil, err
	}

	return &Config{
		Base:       base,
		ServerBase: serverBase,

		PostgresDSN:   postgresDSN,
		InventoryAddr: config.GetEnv("INVENTORY_ADDR", "localhost:50053"),
		PaymentAddr:   config.GetEnv("PAYMENT_ADDR", "localhost:50054"),
		CatalogAddr:   config.GetEnv("CATALOG_ADDR", "localhost:50052"),
		KafkaBrokers:  kafkaBrokers,
		KafkaTopic:    kafkaTopic,
		RedisAddr:     config.GetEnv("REDIS_ADDR", "localhost:6379"),
	}, nil
}

func (c *Config) GetPostgresDSN() string    { return c.PostgresDSN }
func (c *Config) GetRedisAddr() string      { return c.RedisAddr }
func (c *Config) GetKafkaBrokers() []string { return c.KafkaBrokers }
func (c *Config) GetInsecureSkipTLS() bool  { return false }

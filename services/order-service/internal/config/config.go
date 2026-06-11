package config

import (
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort            int
	MetricsPort         int
	PostgresDSN         string
	InventoryAddr       string
	PaymentAddr         string
	JWTSecret           string
	DefaultCallTimeout  time.Duration
	DefaultQueryTimeout time.Duration
	CertPath            string
	KafkaBrokers        []string
	KafkaTopic          string
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50055)
	return &Config{
		GRPCPort:            grpcPort,
		MetricsPort:         grpcPort + 1000,
		PostgresDSN:         config.MustGetEnv("POSTGRES_DSN"),
		InventoryAddr:       config.GetEnv("INVENTORY_ADDR", "localhost:50053"),
		PaymentAddr:         config.GetEnv("PAYMENT_ADDR", "localhost:50054"),
		JWTSecret:           config.MustGetEnv("JWT_SECRET"),
		DefaultCallTimeout:  config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout: config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
		CertPath:            config.GetEnv("CERT_PATH", ""),
		KafkaBrokers:        config.GetEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		KafkaTopic:          config.GetEnv("KAFKA_TOPIC", "order-events"),
	}
}

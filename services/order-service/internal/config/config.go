package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort      int
	MetricsPort   int
	PostgresDSN   string
	InventoryAddr string
	PaymentAddr   string
	JWTSecret     string
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50055)
	return &Config{
		GRPCPort:      grpcPort,
		MetricsPort:   grpcPort + 1000,
		PostgresDSN:   config.MustGetEnv("POSTGRES_DSN"),
		InventoryAddr: config.GetEnv("INVENTORY_ADDR", "localhost:50053"),
		PaymentAddr:   config.GetEnv("PAYMENT_ADDR", "localhost:50054"),
		JWTSecret:     config.MustGetEnv("JWT_SECRET"),
	}
}

package config

import (
	"os"
	"strconv"
)

type Config struct {
	GRPCPort      int
	MetricsPort   int
	PostgresDSN   string
	InventoryAddr string
	PaymentAddr   string
}

func Load() *Config {
	grpcPort, _ := strconv.Atoi(getEnv("GRPC_PORT", "50055"))
	return &Config{
		GRPCPort:      grpcPort,
		MetricsPort:   grpcPort + 1000,
		PostgresDSN:   getEnv("POSTGRES_DSN", "postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable"),
		InventoryAddr: getEnv("INVENTORY_ADDR", "localhost:50053"),
		PaymentAddr:   getEnv("PAYMENT_ADDR", "localhost:50054"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

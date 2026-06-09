package config

import (
	"os"
	"strconv"
)

type Config struct {
	GRPCPort    int
	PostgresDSN string
}

func Load() *Config {
	grpcPort, _ := strconv.Atoi(getEnv("GRPC_PORT", "50054"))
	return &Config{
		GRPCPort:    grpcPort,
		PostgresDSN: getEnv("POSTGRES_DSN", "postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

package config

import (
	"os"
	"strconv"
)

type Config struct {
	GRPCPort    int
	HTTPPort    int
	PostgresDSN string
	JWTSecret   string
}

func Load() *Config {
	grpcPort, _ := strconv.Atoi(getEnv("GRPC_PORT", "50051"))
	httpPort, _ := strconv.Atoi(getEnv("HTTP_PORT", "8080"))
	return &Config{
		GRPCPort:    grpcPort,
		HTTPPort:    httpPort,
		PostgresDSN: getEnv("POSTGRES_DSN", "postgres://ozon:ozonpass@localhost:5432/marketplace?sslmode=disable"),
		JWTSecret:   getEnv("JWT_SECRET", "super-secret-key"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

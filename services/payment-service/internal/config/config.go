package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort    int
	MetricsPort int
	PostgresDSN string
	JWTSecret   string
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50054)
	return &Config{
		GRPCPort:    grpcPort,
		MetricsPort: grpcPort + 1000,
		PostgresDSN: config.MustGetEnv("POSTGRES_DSN"),
		JWTSecret:   config.MustGetEnv("JWT_SECRET"),
	}
}

package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort    int
	MetricsPort int
	PostgresDSN string
	RedisAddr   string
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50053)
	return &Config{
		GRPCPort:    grpcPort,
		MetricsPort: grpcPort + 1000,
		PostgresDSN: config.MustGetEnv("POSTGRES_DSN"),
		RedisAddr:   config.GetEnv("REDIS_ADDR", "localhost:6379"),
	}
}

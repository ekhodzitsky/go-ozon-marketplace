package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort    int
	MetricsPort int
	JWTSecret   string
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50056)
	return &Config{
		GRPCPort:    grpcPort,
		MetricsPort: grpcPort + 1000,
		JWTSecret:   config.MustGetEnv("JWT_SECRET"),
	}
}

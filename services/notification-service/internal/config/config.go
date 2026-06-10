package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort    int
	MetricsPort int
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50056)
	return &Config{
		GRPCPort:    grpcPort,
		MetricsPort: grpcPort + 1000,
	}
}

package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort    int
	MetricsPort int
	PostgresDSN string
	ESURL       string
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50052)
	return &Config{
		GRPCPort:    grpcPort,
		MetricsPort: grpcPort + 1000,
		PostgresDSN: config.MustGetEnv("POSTGRES_DSN"),
		ESURL:       config.GetEnv("ES_URL", "http://localhost:9200"),
	}
}

package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort       int
	MetricsPort    int
	ClickHouseAddr string
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50057)
	return &Config{
		GRPCPort:       grpcPort,
		MetricsPort:    grpcPort + 1000,
		ClickHouseAddr: config.GetEnv("CLICKHOUSE_DSN", "localhost:9000"),
	}
}

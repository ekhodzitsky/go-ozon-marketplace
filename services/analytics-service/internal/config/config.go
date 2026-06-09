package config

import (
	"os"
	"strconv"
)

type Config struct {
	GRPCPort       int
	MetricsPort    int
	ClickHouseAddr string
}

func Load() *Config {
	grpcPort, _ := strconv.Atoi(getEnv("GRPC_PORT", "50057"))
	return &Config{
		GRPCPort:       grpcPort,
		MetricsPort:    grpcPort + 1000,
		ClickHouseAddr: getEnv("CLICKHOUSE_DSN", "localhost:9000"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

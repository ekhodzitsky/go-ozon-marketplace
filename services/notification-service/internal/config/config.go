package config

import (
	"os"
	"strconv"
)

type Config struct {
	GRPCPort int
}

func Load() *Config {
	grpcPort, _ := strconv.Atoi(getEnv("GRPC_PORT", "50056"))
	return &Config{
		GRPCPort: grpcPort,
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

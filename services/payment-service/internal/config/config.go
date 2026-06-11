package config

import (
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort            int
	MetricsPort         int
	PostgresDSN         string
	JWTSecret           string
	DefaultCallTimeout  time.Duration
	DefaultQueryTimeout time.Duration
	CertPath            string
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50054)
	return &Config{
		GRPCPort:            grpcPort,
		MetricsPort:         grpcPort + 1000,
		PostgresDSN:         config.MustGetEnv("POSTGRES_DSN"),
		JWTSecret:           config.MustGetEnv("JWT_SECRET"),
		DefaultCallTimeout:  config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout: config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
		CertPath:            config.GetEnv("CERT_PATH", ""),
	}
}

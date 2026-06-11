package config

import (
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort            int
	HTTPPort            int
	MetricsPort         int
	PostgresDSN         string
	JWTSecret           string
	CertPath            string
	DefaultCallTimeout  time.Duration
	DefaultQueryTimeout time.Duration
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50051)
	httpPort := config.GetEnvInt("HTTP_PORT", 8080)
	return &Config{
		GRPCPort:            grpcPort,
		HTTPPort:            httpPort,
		MetricsPort:         grpcPort + 1000,
		PostgresDSN:         config.MustGetEnv("POSTGRES_DSN"),
		JWTSecret:           config.MustGetEnv("JWT_SECRET"),
		CertPath:            config.GetEnv("CERT_PATH", ""),
		DefaultCallTimeout:  config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout: config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
	}
}

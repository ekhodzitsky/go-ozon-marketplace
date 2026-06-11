package config

import (
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	GRPCPort                 int
	HTTPPort                 int
	MetricsPort              int
	LogLevel                 string
	LogFormat                string
	OTELExporterOTLPEndpoint string
	PostgresDSN              string
	JWTSecret                string
	CertPath                 string
	DefaultCallTimeout       time.Duration
	DefaultQueryTimeout      time.Duration
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50051)
	httpPort := config.GetEnvInt("HTTP_PORT", 8080)
	jwtSecret := config.MustGetEnv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		panic("JWT_SECRET must be at least 32 characters long")
	}
	return &Config{
		GRPCPort:                 grpcPort,
		HTTPPort:                 httpPort,
		MetricsPort:              config.GetEnvInt("METRICS_PORT", grpcPort+1000),
		LogLevel:                 config.GetEnv("LOG_LEVEL", "info"),
		LogFormat:                config.GetEnv("LOG_FORMAT", "json"),
		OTELExporterOTLPEndpoint: config.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		PostgresDSN:              config.MustGetEnv("POSTGRES_DSN"),
		JWTSecret:                jwtSecret,
		CertPath:                 config.GetEnv("CERT_PATH", ""),
		DefaultCallTimeout:       config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout:      config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
	}
}

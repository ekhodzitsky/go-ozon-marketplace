package config

import (
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	GRPCPort                 int
	MetricsPort              int
	LogLevel                 string
	LogFormat                string
	OTELExporterOTLPEndpoint string
	PostgresDSN              string
	ESURL                    string
	JWTSecret                string
	DefaultCallTimeout       time.Duration
	DefaultQueryTimeout      time.Duration
	CertPath                 string
}

func Load() *Config {
	grpcPort := config.GetEnvInt("GRPC_PORT", 50052)
	postgresDSN := config.MustGetEnv("POSTGRES_DSN")
	if _, err := pgxpool.ParseConfig(postgresDSN); err != nil {
		panic(fmt.Sprintf("invalid POSTGRES_DSN: %v", err))
	}

	jwtSecret := config.MustGetEnv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		panic("JWT_SECRET must be at least 32 characters long")
	}

	return &Config{
		GRPCPort:                 grpcPort,
		MetricsPort:              config.GetEnvInt("METRICS_PORT", grpcPort+1000),
		LogLevel:                 config.GetEnv("LOG_LEVEL", "info"),
		LogFormat:                config.GetEnv("LOG_FORMAT", "json"),
		OTELExporterOTLPEndpoint: config.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		PostgresDSN:              postgresDSN,
		ESURL:                    config.GetEnv("ES_URL", "http://localhost:9200"),
		JWTSecret:                jwtSecret,
		DefaultCallTimeout:       config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout:      config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
		CertPath:                 config.GetEnv("CERT_PATH", ""),
	}
}

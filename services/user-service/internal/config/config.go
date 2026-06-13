package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrMissingJWTSecret   = errors.New("JWT_SECRET is required")
	ErrJWTSecretTooShort  = errors.New("JWT_SECRET must be at least 32 characters long")
	ErrMissingPostgresDSN = errors.New("POSTGRES_DSN is required")
	ErrInvalidPostgresDSN = errors.New("POSTGRES_DSN is invalid")
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

func Load() (*Config, error) {
	jwtSecret := config.GetEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		return nil, ErrMissingJWTSecret
	}
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("%w: got %d characters", ErrJWTSecretTooShort, len(jwtSecret))
	}

	postgresDSN := config.GetEnv("POSTGRES_DSN", "")
	if postgresDSN == "" {
		return nil, ErrMissingPostgresDSN
	}
	if _, err := pgxpool.ParseConfig(postgresDSN); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPostgresDSN, err)
	}

	grpcPort := config.GetEnvInt("GRPC_PORT", 50051)
	httpPort := config.GetEnvInt("HTTP_PORT", 8080)

	return &Config{
		GRPCPort:                 grpcPort,
		HTTPPort:                 httpPort,
		MetricsPort:              config.GetEnvInt("METRICS_PORT", grpcPort+1000),
		LogLevel:                 config.GetEnv("LOG_LEVEL", "info"),
		LogFormat:                config.GetEnv("LOG_FORMAT", "json"),
		OTELExporterOTLPEndpoint: config.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		PostgresDSN:              postgresDSN,
		JWTSecret:                jwtSecret,
		CertPath:                 config.GetEnv("CERT_PATH", ""),
		DefaultCallTimeout:       config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout:      config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
	}, nil
}

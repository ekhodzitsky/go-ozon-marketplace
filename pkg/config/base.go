package config

import "time"

// Base contains settings shared by every service: logging, tracing, auth and timeouts.
type Base struct {
	LogLevel                 string
	LogFormat                string
	OTELExporterOTLPEndpoint string
	JWTSecret                string
	CertPath                 string
	DefaultCallTimeout       time.Duration
	DefaultQueryTimeout      time.Duration
}

// LoadBase reads the common environment variables used by all services.
func LoadBase() Base {
	return Base{
		LogLevel:                 GetEnv("LOG_LEVEL", "info"),
		LogFormat:                GetEnv("LOG_FORMAT", "json"),
		OTELExporterOTLPEndpoint: GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		JWTSecret:                GetEnv("JWT_SECRET", ""),
		CertPath:                 GetEnv("CERT_PATH", ""),
		DefaultCallTimeout:       GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout:      GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
	}
}

func (b *Base) GetLogLevel() string                 { return b.LogLevel }
func (b *Base) GetLogFormat() string                { return b.LogFormat }
func (b *Base) GetOTELExporterOTLPEndpoint() string { return b.OTELExporterOTLPEndpoint }
func (b *Base) GetJWTSecret() string                { return b.JWTSecret }
func (b *Base) GetCertPath() string                 { return b.CertPath }
func (b *Base) GetDefaultCallTimeout() time.Duration { return b.DefaultCallTimeout }
func (b *Base) GetDefaultQueryTimeout() time.Duration { return b.DefaultQueryTimeout }

// ServerBase contains the network settings for standard gRPC services.
type ServerBase struct {
	GRPCPort    int
	MetricsPort int
}

// LoadServerBase reads gRPC and metrics ports. Metrics default to grpcPort+1000.
func LoadServerBase(defaultGRPCPort int) ServerBase {
	grpcPort := GetEnvInt("GRPC_PORT", defaultGRPCPort)
	return ServerBase{
		GRPCPort:    grpcPort,
		MetricsPort: GetEnvInt("METRICS_PORT", grpcPort+1000),
	}
}

func (s *ServerBase) GetGRPCPort() int    { return s.GRPCPort }
func (s *ServerBase) GetMetricsPort() int { return s.MetricsPort }

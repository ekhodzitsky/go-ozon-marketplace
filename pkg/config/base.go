package config

import "time"

// Base — общие настройки всех сервисов: логи, трассировка, auth и таймауты.
type Base struct {
	LogLevel                 string
	LogFormat                string
	OTELExporterOTLPEndpoint string
	JWTSecret                string
	CertPath                 string
	DefaultCallTimeout       time.Duration
	DefaultQueryTimeout      time.Duration
}

// LoadBase читает общие переменные окружения, которые используют все сервисы.
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

// ServerBase — сетевые настройки обычного gRPC-сервиса.
type ServerBase struct {
	GRPCPort    int
	MetricsPort int
}

// LoadServerBase читает порты gRPC и метрик. Метрики по умолчанию grpcPort+1000.
func LoadServerBase(defaultGRPCPort int) ServerBase {
	grpcPort := GetEnvInt("GRPC_PORT", defaultGRPCPort)
	return ServerBase{
		GRPCPort:    grpcPort,
		MetricsPort: GetEnvInt("METRICS_PORT", grpcPort+1000),
	}
}

func (s *ServerBase) GetGRPCPort() int    { return s.GRPCPort }
func (s *ServerBase) GetMetricsPort() int { return s.MetricsPort }

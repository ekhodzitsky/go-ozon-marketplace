package config

import (
	"strings"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	UserServiceAddr          string
	CatalogServiceAddr       string
	OrderServiceAddr         string
	InventoryServiceAddr     string
	PaymentServiceAddr       string
	HTTPPort                 string
	MetricsPort              int
	LogLevel                 string
	LogFormat                string
	OTELExporterOTLPEndpoint string
	RedisAddr                string
	RateLimitRPS             int
	RateLimitWindow          time.Duration
	RateLimitUserRPS         int
	RateLimitAdminRPS        int
	TrustedProxies           []string
	MaxBodySizeBytes         int64
	DefaultCallTimeout       time.Duration
	DefaultQueryTimeout      time.Duration
	CertPath                 string
	JWTSecret                string
	CORSAllowedOrigins       []string
}

func Load() *Config {
	trusted := []string{}
	if v := config.GetEnv("TRUSTED_PROXIES", ""); v != "" {
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				trusted = append(trusted, s)
			}
		}
	}
	corsOrigins := []string{"*"}
	if v := config.GetEnv("CORS_ALLOWED_ORIGINS", ""); v != "" {
		corsOrigins = []string{}
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				corsOrigins = append(corsOrigins, s)
			}
		}
	}
	httpPort := config.GetEnvInt("PORT", 8080)
	return &Config{
		UserServiceAddr:          config.GetEnv("USER_SERVICE_ADDR", "localhost:50051"),
		CatalogServiceAddr:       config.GetEnv("CATALOG_SERVICE_ADDR", "localhost:50052"),
		OrderServiceAddr:         config.GetEnv("ORDER_SERVICE_ADDR", "localhost:50055"),
		InventoryServiceAddr:     config.GetEnv("INVENTORY_SERVICE_ADDR", "localhost:50053"),
		PaymentServiceAddr:       config.GetEnv("PAYMENT_SERVICE_ADDR", "localhost:50054"),
		HTTPPort:                 config.GetEnv("PORT", "8080"),
		MetricsPort:              config.GetEnvInt("METRICS_PORT", httpPort+1000),
		LogLevel:                 config.GetEnv("LOG_LEVEL", "info"),
		LogFormat:                config.GetEnv("LOG_FORMAT", "json"),
		OTELExporterOTLPEndpoint: config.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		RedisAddr:                config.GetEnv("REDIS_ADDR", "localhost:6379"),
		RateLimitRPS:             config.GetEnvInt("RATE_LIMIT_RPS", 10),
		RateLimitWindow:          config.GetEnvDuration("RATE_LIMIT_WINDOW", time.Second),
		RateLimitUserRPS:         config.GetEnvInt("RATE_LIMIT_USER_RPS", 100),
		RateLimitAdminRPS:        config.GetEnvInt("RATE_LIMIT_ADMIN_RPS", 1000),
		TrustedProxies:           trusted,
		MaxBodySizeBytes:         config.GetEnvInt64("MAX_BODY_SIZE_BYTES", 1<<20),
		DefaultCallTimeout:       config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout:      config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
		CertPath:                 config.GetEnv("CERT_PATH", ""),
		JWTSecret:                config.GetEnv("JWT_SECRET", ""),
		CORSAllowedOrigins:       corsOrigins,
	}
}

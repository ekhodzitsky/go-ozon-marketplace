package config

import (
	"strings"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	UserServiceAddr     string
	CatalogServiceAddr  string
	HTTPPort            string
	RedisAddr           string
	RateLimitRPS        int
	RateLimitWindow     time.Duration
	TrustedProxies      []string
	MaxBodySizeBytes    int64
	DefaultCallTimeout  time.Duration
	DefaultQueryTimeout time.Duration
	CertPath            string
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
	return &Config{
		UserServiceAddr:     config.GetEnv("USER_SERVICE_ADDR", "localhost:50051"),
		CatalogServiceAddr:  config.GetEnv("CATALOG_SERVICE_ADDR", "localhost:50052"),
		RedisAddr:           config.GetEnv("REDIS_ADDR", "localhost:6379"),
		HTTPPort:            config.GetEnv("PORT", "8080"),
		RateLimitRPS:        config.GetEnvInt("RATE_LIMIT_RPS", 10),
		RateLimitWindow:     config.GetEnvDuration("RATE_LIMIT_WINDOW", time.Second),
		TrustedProxies:      trusted,
		MaxBodySizeBytes:    config.GetEnvInt64("MAX_BODY_SIZE_BYTES", 1<<20),
		DefaultCallTimeout:  config.GetEnvDuration("DEFAULT_CALL_TIMEOUT", 5*time.Second),
		DefaultQueryTimeout: config.GetEnvDuration("DEFAULT_QUERY_TIMEOUT", 3*time.Second),
		CertPath:            config.GetEnv("CERT_PATH", ""),
	}
}

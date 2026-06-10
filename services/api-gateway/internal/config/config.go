package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	UserServiceAddr    string
	CatalogServiceAddr string
	HTTPPort           string
	RateLimitRPS       int
}

func Load() *Config {
	return &Config{
		UserServiceAddr:    config.GetEnv("USER_SERVICE_ADDR", "localhost:50051"),
		CatalogServiceAddr: config.GetEnv("CATALOG_SERVICE_ADDR", "localhost:50052"),
		HTTPPort:           config.GetEnv("PORT", "8080"),
		RateLimitRPS:       config.GetEnvInt("RATE_LIMIT_RPS", 10),
	}
}

package config

import "os"

// Config holds API-gateway configuration.
type Config struct {
	UserServiceAddr    string
	CatalogServiceAddr string
	HTTPPort           string
}

// NewDefaultConfig returns a Config with local defaults.
func NewDefaultConfig() *Config {
	return &Config{
		UserServiceAddr:    getEnv("USER_SERVICE_ADDR", "localhost:50051"),
		CatalogServiceAddr: getEnv("CATALOG_SERVICE_ADDR", "localhost:50052"),
		HTTPPort:           getEnv("PORT", "8080"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

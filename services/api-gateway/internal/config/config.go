package config

// Config holds API-gateway configuration.
type Config struct {
	UserServiceAddr    string
	CatalogServiceAddr string
	HTTPPort           string
}

// NewDefaultConfig returns a Config with local defaults.
func NewDefaultConfig() *Config {
	return &Config{
		UserServiceAddr:    "localhost:50051",
		CatalogServiceAddr: "localhost:50052",
		HTTPPort:           "8080",
	}
}

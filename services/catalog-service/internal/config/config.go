package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	config.Base
	config.ServerBase

	PostgresDSN string
	ESURL       string
}

func Load() (*Config, error) {
	base := config.LoadBase()
	serverBase := config.LoadServerBase(50052)

	postgresDSN := config.GetEnv("POSTGRES_DSN", "")
	if err := config.ValidatePostgresDSN(postgresDSN); err != nil {
		return nil, err
	}

	if err := config.ValidateJWTSecret(base.JWTSecret, 32); err != nil {
		return nil, err
	}

	return &Config{
		Base:        base,
		ServerBase:  serverBase,
		PostgresDSN: postgresDSN,
		ESURL:       config.GetEnv("ES_URL", "http://localhost:9200"),
	}, nil
}

func (c *Config) GetPostgresDSN() string { return c.PostgresDSN }

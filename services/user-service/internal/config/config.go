package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

var (
	ErrMissingJWTSecret   = config.ErrMissingJWTSecret
	ErrJWTSecretTooShort  = config.ErrJWTSecretTooShort
	ErrMissingPostgresDSN = config.ErrMissingPostgresDSN
	ErrInvalidPostgresDSN = config.ErrInvalidPostgresDSN
)

type Config struct {
	config.Base
	config.ServerBase

	HTTPPort    int
	PostgresDSN string
}

func Load() (*Config, error) {
	base := config.LoadBase()
	if err := config.ValidateJWTSecret(base.JWTSecret, 32); err != nil {
		return nil, err
	}

	postgresDSN := config.GetEnv("POSTGRES_DSN", "")
	if err := config.ValidatePostgresDSN(postgresDSN); err != nil {
		return nil, err
	}

	serverBase := config.LoadServerBase(50051)
	httpPort := config.GetEnvInt("HTTP_PORT", 8080)

	return &Config{
		Base:        base,
		ServerBase:  serverBase,
		HTTPPort:    httpPort,
		PostgresDSN: postgresDSN,
	}, nil
}

func (c *Config) GetPostgresDSN() string { return c.PostgresDSN }
func (c *Config) GetHTTPPort() int       { return c.HTTPPort }

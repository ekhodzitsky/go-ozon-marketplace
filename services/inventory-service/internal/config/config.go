package config

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	config.Base
	config.ServerBase

	PostgresDSN string
	RedisAddr   string
}

func Load() (*Config, error) {
	base := config.LoadBase()
	if err := config.ValidateJWTSecret(base.JWTSecret, 32); err != nil {
		return nil, err
	}
	serverBase := config.LoadServerBase(50053)
	postgresDSN := config.GetEnv("POSTGRES_DSN", "")
	if err := config.ValidatePostgresDSN(postgresDSN); err != nil {
		return nil, err
	}
	return &Config{
		Base:        base,
		ServerBase:  serverBase,
		PostgresDSN: postgresDSN,
		RedisAddr:   config.GetEnv("REDIS_ADDR", "localhost:6379"),
	}, nil
}

func (c *Config) GetPostgresDSN() string { return c.PostgresDSN }
func (c *Config) GetRedisAddr() string   { return c.RedisAddr }

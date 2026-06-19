package fxmodules

import (
	"context"
	"time"

	pkgredis "github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// RedisConfig — настройки подключения к Redis.
type RedisConfig interface {
	GetRedisAddr() string
	GetDefaultQueryTimeout() time.Duration
}

// Redis отдаёт Redis-клиент как fx-модуль и закрывает его при остановке.
// Настройки берутся из RedisConfig через DI, чтобы тесты могли их переопределять.
func Redis(cfg RedisConfig) fx.Option {
	return fx.Options(
		fx.Provide(func() RedisConfig { return cfg }),
		fx.Provide(func(lc fx.Lifecycle, cfg RedisConfig) (*redis.Client, error) {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.GetDefaultQueryTimeout())
			defer cancel()
			client, err := pkgredis.NewClient(ctx, cfg.GetRedisAddr())
			if err != nil {
				return nil, err
			}
			lc.Append(fx.Hook{OnStop: func(ctx context.Context) error { return client.Close() }})
			return client, nil
		}),
	)
}

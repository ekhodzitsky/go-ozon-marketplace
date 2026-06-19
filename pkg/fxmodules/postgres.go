package fxmodules

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

// PostgresConfig — настройки подключения к Postgres.
type PostgresConfig interface {
	GetPostgresDSN() string
	GetDefaultQueryTimeout() time.Duration
}

// Postgres отдаёт пул соединений pgx как fx-модуль.
// Настройки берутся из PostgresConfig через DI, чтобы тесты могли их переопределять.
func Postgres(cfg PostgresConfig) fx.Option {
	return fx.Options(
		fx.Provide(func() PostgresConfig { return cfg }),
		fx.Provide(func(cfg PostgresConfig) (*pgxpool.Pool, error) {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.GetDefaultQueryTimeout())
			defer cancel()
			return postgres.NewPool(ctx, cfg.GetPostgresDSN())
		}),
	)
}

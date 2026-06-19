package fxmodules

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

// PostgresConfig exposes Postgres connection settings.
type PostgresConfig interface {
	GetPostgresDSN() string
	GetDefaultQueryTimeout() time.Duration
}

// Postgres provides a pgx connection pool as an fx module.
// Settings are resolved from PostgresConfig via DI, so tests can override them.
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

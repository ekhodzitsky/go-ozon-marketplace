package app

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	pkgredis "github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func New() *fx.App {
	return fx.New(
		fx.Provide(
			config.Load,
			func(cfg *config.Config) (*zap.Logger, error) {
				return logger.New(cfg.LogLevel, cfg.LogFormat)
			},
			func(cfg *config.Config) (*pgxpool.Pool, error) {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.DefaultQueryTimeout)
				defer cancel()
				return pkgpostgres.NewPool(ctx, cfg.PostgresDSN)
			},
			func(cfg *config.Config, lc fx.Lifecycle) (*redis.Client, error) {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.DefaultQueryTimeout)
				defer cancel()
				client, err := pkgredis.NewClient(ctx, cfg.RedisAddr)
				if err != nil {
					return nil, err
				}
				lc.Append(fx.Hook{OnStop: func(ctx context.Context) error { return client.Close() }})
				return client, nil
			},
			func(db *pgxpool.Pool) postgres.Querier {
				return db
			},
			postgres.NewInventoryPostgres,
			postgres.NewInventoryTxManager,
			func(repo repository.InventoryRepository, txm repository.TxManager, redisClient *redis.Client, cfg *config.Config) usecase.InventoryUsecase {
				return usecase.NewInventoryUsecase(repo, txm, redisClient, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
			},
			grpcdelivery.NewInventoryHandler,
		),
		fx.Invoke(registerServers),
	)
}

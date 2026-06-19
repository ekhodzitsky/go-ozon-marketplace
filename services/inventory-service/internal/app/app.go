package app

import (
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/fxmodules"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

func New(cfg *config.Config) *fx.App {
	return fxmodules.GRPCService(
		"inventory-service",
		cfg,
		inventoryv1.RegisterInventoryServiceServer,
		grpcdelivery.NewInventoryHandler,
		fxmodules.Postgres(cfg),
		fxmodules.Redis(cfg),
		fx.Provide(
			func(db *pgxpool.Pool) postgres.Querier {
				return db
			},
			postgres.NewInventoryPostgres,
			postgres.NewInventoryTxManager,
			func(repo repository.InventoryRepository, txm repository.TxManager, redisClient *redis.Client, cfg *config.Config) usecase.InventoryUsecase {
				return usecase.NewInventoryUsecase(repo, txm, redisClient, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
			},
		),
	)
}

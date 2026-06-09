package app

import (
	"context"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	pkgredis "github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func New() *fx.App {
	return fx.New(
		fx.Provide(
			config.Load,
			logger.New,
			func(cfg *config.Config) (*pgxpool.Pool, error) {
				return pkgpostgres.NewPool(context.Background(), cfg.PostgresDSN)
			},
			func(cfg *config.Config) (*redis.Client, error) {
				return pkgredis.NewClient(context.Background(), cfg.RedisAddr)
			},
			postgres.NewInventoryPostgres,
			usecase.NewInventoryUsecase,
			grpcdelivery.NewInventoryHandler,
		),
		fx.Invoke(func(lc fx.Lifecycle, handler *grpcdelivery.InventoryHandler, cfg *config.Config, log *zap.Logger) {
			grpcServer := server.NewGRPC(cfg.GRPCPort, grpc.UnaryInterceptor(middleware.LoggingUnaryInterceptor))
			inventoryv1.RegisterInventoryServiceServer(grpcServer.Server, handler)

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go func() {
						if err := grpcServer.Start(); err != nil {
							log.Error("grpc server error", zap.Error(err))
						}
					}()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					grpcServer.GracefulStop()
					return nil
				},
			})
		}),
	)
}

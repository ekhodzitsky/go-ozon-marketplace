package app

import (
	"context"

	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/usecase"
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
			postgres.NewUserPostgres,
			func(repo repository.UserRepository, cfg *config.Config) *usecase.UserUsecase {
				return usecase.NewUserUsecase(repo, cfg.JWTSecret)
			},
			grpcdelivery.NewUserHandler,
		),
		fx.Invoke(func(lc fx.Lifecycle, handler *grpcdelivery.UserHandler, cfg *config.Config, log *zap.Logger) {
			grpcServer := server.NewGRPC(cfg.GRPCPort, grpc.UnaryInterceptor(middleware.LoggingUnaryInterceptor))
			userv1.RegisterUserServiceServer(grpcServer.Server, handler)

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

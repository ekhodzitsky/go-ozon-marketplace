package app

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
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
			postgres.NewUserPostgres,
			func(repo repository.UserRepository, cfg *config.Config) usecase.UserUsecase {
				// In-memory rate limiter: 5 requests per minute per email for auth endpoints.
				rl := usecase.NewMemoryRateLimiter(5, time.Minute)
				return usecase.NewUserUsecase(repo, cfg.JWTSecret, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout, rl)
			},
			grpcdelivery.NewUserHandler,
		),
		fx.Invoke(registerServers),
	)
}

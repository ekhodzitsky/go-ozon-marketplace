package app

import (
	"time"

	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/fxmodules"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/ratelimit"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/usecase"
	"go.uber.org/fx"
)

func New(cfg *config.Config) *fx.App {
	return fxmodules.GRPCService(
		"user-service",
		cfg,
		userv1.RegisterUserServiceServer,
		grpcdelivery.NewUserHandler,
		fxmodules.Postgres(cfg),
		fx.Provide(
			postgres.NewUserPostgres,
			func(repo repository.UserRepository, cfg *config.Config) usecase.UserUsecase {
				// Лимит на регистрацию/логин: 5 попыток в минуту с одного email.
				rl := ratelimit.NewMemoryRateLimiter(5, time.Minute)
				return usecase.NewUserUsecase(repo, cfg.JWTSecret, cfg.DefaultCallTimeout, rl)
			},
		),
	)
}

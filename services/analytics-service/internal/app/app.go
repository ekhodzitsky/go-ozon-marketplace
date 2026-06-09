package app

import (
	"context"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/repository/clickhouse"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func New() *fx.App {
	return fx.New(
		fx.Provide(
			config.Load,
			logger.New,
			func(cfg *config.Config) (*clickhouse.EventRepo, error) {
				return clickhouse.NewEventRepo(cfg.ClickHouseAddr)
			},
			usecase.NewAnalyticsUsecase,
			grpcdelivery.NewAnalyticsHandler,
		),
		fx.Invoke(func(lc fx.Lifecycle, handler *grpcdelivery.AnalyticsHandler, cfg *config.Config, log *zap.Logger) {
			grpcServer := server.NewGRPC(cfg.GRPCPort, grpc.UnaryInterceptor(middleware.LoggingUnaryInterceptor))
			analyticsv1.RegisterAnalyticsServiceServer(grpcServer.Server, handler)

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

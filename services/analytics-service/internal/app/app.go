package app

import (
	"context"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/fxmodules"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/consumer"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/repository/clickhouse"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/migrations"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// New builds the fx application container. Optional fx overrides can be supplied for tests.
func New(cfg *config.Config, overrides ...fx.Option) *fx.App {
	return fxmodules.GRPCService("analytics-service", cfg,
		analyticsv1.RegisterAnalyticsServiceServer,
		grpcdelivery.NewAnalyticsHandler,
		fx.Provide(
			func(cfg *config.Config) (*clickhouse.EventRepo, error) {
				return clickhouse.NewEventRepo(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePassword, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout, migrations.FS)
			},
			func(cfg *config.Config, repo *clickhouse.EventRepo, log *zap.Logger) usecase.AnalyticsUsecase {
				return usecase.NewAnalyticsUsecase(repo, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout, log)
			},
			consumer.NewProcessor,
		),
		fxmodules.KafkaConsumer(cfg),
		fx.Invoke(func(lc fx.Lifecycle, uc usecase.AnalyticsUsecase, log *zap.Logger) {
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					if err := uc.Flush(ctx); err != nil {
						log.Error("flush analytics buffer error", zap.Error(err))
						return err
					}
					return nil
				},
			})
		}),
		fx.Options(overrides...),
	)
}

package app

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/consumer"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/repository/clickhouse"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/migrations"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// New builds the fx application container. It returns the constructed app
// together with any error produced during dependency resolution (e.g. invalid
// configuration or unreachable infrastructure).
// Optional fx overrides can be supplied for tests.
func New(overrides ...fx.Option) (*fx.App, error) {
	application := fx.New(
		fx.Provide(
			config.Load,
			func(cfg *config.Config) (*zap.Logger, error) {
				return logger.New(cfg.LogLevel, cfg.LogFormat)
			},
			func(cfg *config.Config) (*clickhouse.EventRepo, error) {
				return clickhouse.NewEventRepo(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePassword, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout, migrations.FS)
			},
			func(cfg *config.Config, repo *clickhouse.EventRepo, log *zap.Logger) usecase.AnalyticsUsecase {
				return usecase.NewAnalyticsUsecase(repo, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout, log)
			},
			func(cfg *config.Config, uc usecase.AnalyticsUsecase, log *zap.Logger) (*consumer.Consumer, error) {
				return consumer.NewConsumer(cfg.KafkaBrokers, cfg.KafkaConsumerGroup, cfg.KafkaTopics, uc, log)
			},
			grpcdelivery.NewAnalyticsHandler,
		),
		fx.Invoke(registerConsumer),
		fx.Invoke(registerServers),
		fx.Options(overrides...),
	)

	return application, application.Err()
}

func registerConsumer(lc fx.Lifecycle, c *consumer.Consumer, log *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			c.Start(ctx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := c.Close(); err != nil {
				log.Error("consumer close error", zap.Error(err))
			}
			return nil
		},
	})
}

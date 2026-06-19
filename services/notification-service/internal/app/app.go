package app

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/consumer"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/email"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/usecase"
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
			func(cfg *config.Config, log *zap.Logger) email.Provider {
				if cfg.SMTPHost != "" {
					return email.NewSMTPProvider(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom, cfg.SMTPUser, cfg.SMTPPassword)
				}
				return email.NewLogProvider(log)
			},
			func(log *zap.Logger, cfg *config.Config, provider email.Provider) usecase.NotificationUsecase {
				return usecase.NewNotificationUsecase(log, provider, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
			},
			func(cfg *config.Config, uc usecase.NotificationUsecase, log *zap.Logger) (*consumer.Consumer, error) {
				return consumer.NewConsumer(cfg.KafkaBrokers, cfg.KafkaConsumerGroup, cfg.KafkaTopics, cfg.KafkaDLQTopic, uc, log)
			},
			grpcdelivery.NewNotificationHandler,
		),
		fx.Invoke(func(lc fx.Lifecycle, c *consumer.Consumer, log *zap.Logger) {
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
		}),
		fx.Invoke(registerServers),
	)
}

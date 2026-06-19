package app

import (
	"errors"

	notificationv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/notification/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/fxmodules"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/kafka"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/consumer"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/email"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func New(cfg *config.Config) *fx.App {
	return fxmodules.GRPCService(
		"notification-service",
		cfg,
		notificationv1.RegisterNotificationServiceServer,
		grpcdelivery.NewNotificationHandler,
		fxmodules.KafkaConsumer(cfg),
		fx.Provide(
			func(cfg *config.Config, log *zap.Logger) email.Provider {
				if cfg.SMTPHost != "" {
					return email.NewSMTPProvider(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom, cfg.SMTPUser, cfg.SMTPPassword)
				}
				return email.NewLogProvider(log)
			},
			func(log *zap.Logger, cfg *config.Config, provider email.Provider) usecase.NotificationUsecase {
				return usecase.NewNotificationUsecase(log, provider, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
			},
			consumer.NewProcessor,
			func() kafka.IsPermanentError {
				return func(err error) bool { return errors.Is(err, apperrors.ErrInvalidArgument) }
			},
		),
	)
}

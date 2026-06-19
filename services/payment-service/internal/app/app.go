package app

import (
	"context"

	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/fxmodules"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/dlq"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func New(cfg *config.Config) *fx.App {
	return fxmodules.GRPCService(
		"payment-service",
		cfg,
		paymentv1.RegisterPaymentServiceServer,
		grpcdelivery.NewPaymentHandler,
		fxmodules.Postgres(cfg),
		fxmodules.KafkaProducer(cfg),
		fx.Provide(
			func(db *pgxpool.Pool) postgres.Querier {
				return db
			},
			postgres.NewPaymentPostgres,
			postgres.NewPaymentTxManager,
			func(repo repository.PaymentRepository, txm repository.TxManager, log *zap.Logger, cfg *config.Config) usecase.PaymentUsecase {
				return usecase.NewPaymentUsecase(repo, txm, log, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
			},
			func(cfg *config.Config, log *zap.Logger) (*dlq.Producer, error) {
				return dlq.NewProducer(cfg.KafkaBrokers, cfg.DLQTopic, log)
			},
		),
		fx.Invoke(func(lc fx.Lifecycle, p *dlq.Producer, log *zap.Logger) {
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					if err := p.Close(); err != nil {
						log.Error("dlq producer close error", zap.Error(err))
					}
					return nil
				},
			})
		}),
	)
}

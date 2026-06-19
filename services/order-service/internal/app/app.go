package app

import (
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/fxmodules"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/kafka"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/txmanager"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/outbox"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork"
	postgresuow "github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func New(cfg *config.Config) *fx.App {
	return fxmodules.GRPCService(
		"order-service",
		cfg,
		orderv1.RegisterOrderServiceServer,
		grpcdelivery.NewOrderHandler,
		fx.Options(
			fxmodules.Postgres(cfg),
			fxmodules.Redis(cfg),
			fxmodules.KafkaProducer(cfg),
			fxmodules.Runner[*outbox.Relay](),
			fxmodules.Runner[*saga.RecoveryWorker](),
			fxmodules.CircuitBreaker("order-service-downstream"),
			fxmodules.GRPCClientFactory(cfg, "order-service", false),
			fxmodules.GRPCClient(cfg.InventoryAddr, inventoryv1.NewInventoryServiceClient),
			fxmodules.GRPCClient(cfg.PaymentAddr, paymentv1.NewPaymentServiceClient),
			fxmodules.GRPCClient(cfg.CatalogAddr, catalogv1.NewCatalogServiceClient),
			fx.Provide(
				func(pool *pgxpool.Pool) *txmanager.Manager[unitofwork.UnitOfWork] {
					return txmanager.New(pool, func(tx pgx.Tx) unitofwork.UnitOfWork {
						return postgresuow.NewUnitOfWork(tx)
					})
				},
				postgres.NewOrderPostgres,
				postgres.NewOutboxPostgres,
				postgres.NewSagaPostgres,
				func(
					orderRepo repository.OrderRepository,
					sagaRepo repository.SagaRepository,
					invClient inventoryv1.InventoryServiceClient,
					payClient paymentv1.PaymentServiceClient,
					log *zap.Logger,
					cfg *config.Config,
				) *saga.Orchestrator {
					return saga.NewOrchestrator(orderRepo, sagaRepo, invClient, payClient, log, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
				},
				func(
					txm *txmanager.Manager[unitofwork.UnitOfWork],
					orderRepo repository.OrderRepository,
					outboxRepo repository.OutboxRepository,
					sagaRepo repository.SagaRepository,
					orchestrator *saga.Orchestrator,
					invClient inventoryv1.InventoryServiceClient,
					payClient paymentv1.PaymentServiceClient,
					catalogClient catalogv1.CatalogServiceClient,
					redisClient *redis.Client,
					cfg *config.Config,
				) usecase.OrderUsecase {
					return usecase.NewOrderUsecase(txm, orderRepo, outboxRepo, sagaRepo, orchestrator, invClient, payClient, catalogClient, redisClient, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
				},
				func(repo repository.OutboxRepository, producer kafka.Producer, log *zap.Logger, cfg *config.Config) *outbox.Relay {
					return outbox.NewRelay(repo, producer, log, cfg.DefaultQueryTimeout, cfg.KafkaTopic)
				},
				func(orchestrator *saga.Orchestrator, pool *pgxpool.Pool, log *zap.Logger) *saga.RecoveryWorker {
					return saga.NewRecoveryWorker(orchestrator, log, saga.WithLocker(saga.NewPostgresAdvisoryLock(pool)))
				},
			),
		),
	)
}

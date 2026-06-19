package app

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	pkgredis "github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/txmanager"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/infrastructure/grpcclient"
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
			func(cfg *config.Config, lc fx.Lifecycle) (*redis.Client, error) {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.DefaultQueryTimeout)
				defer cancel()
				client, err := pkgredis.NewClient(ctx, cfg.RedisAddr)
				if err != nil {
					return nil, err
				}
				lc.Append(fx.Hook{OnStop: func(ctx context.Context) error { return client.Close() }})
				return client, nil
			},
			func(pool *pgxpool.Pool) *txmanager.Manager[unitofwork.UnitOfWork] {
				return txmanager.New(pool, func(tx pgx.Tx) unitofwork.UnitOfWork {
					return postgresuow.NewUnitOfWork(tx)
				})
			},
			func(pool *pgxpool.Pool) *postgres.OrderPostgres { return postgres.NewOrderPostgres(pool) },
			func(r *postgres.OrderPostgres) repository.OrderRepository { return r },
			func(pool *pgxpool.Pool) *postgres.OutboxPostgres { return postgres.NewOutboxPostgres(pool) },
			func(r *postgres.OutboxPostgres) repository.OutboxRepository { return r },
			func(pool *pgxpool.Pool) *postgres.SagaPostgres { return postgres.NewSagaPostgres(pool) },
			func(r *postgres.SagaPostgres) repository.SagaRepository { return r },
			provideCircuitBreaker,
			provideClientFactory,
			provideInventoryClient,
			providePaymentClient,
			provideCatalogClient,
			func(
				orderRepo repository.OrderRepository,
				sagaRepo repository.SagaRepository,
				invClient saga.InventoryClient,
				payClient saga.PaymentClient,
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
				invClient saga.InventoryClient,
				payClient saga.PaymentClient,
				catalogClient grpcclient.CatalogClient,
				redisClient *redis.Client,
				cfg *config.Config,
			) usecase.OrderUsecase {
				return usecase.NewOrderUsecase(txm, orderRepo, outboxRepo, sagaRepo, orchestrator, invClient, payClient, catalogClient, redisClient, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
			},
			grpcdelivery.NewOrderHandler,
			func(cfg *config.Config, lc fx.Lifecycle) (outbox.Producer, error) {
				p, err := outbox.NewSaramaProducer(cfg.KafkaBrokers)
				if err != nil {
					return nil, err
				}
				lc.Append(fx.Hook{OnStop: func(ctx context.Context) error { return p.Close() }})
				return p, nil
			},
			func(repo repository.OutboxRepository, producer outbox.Producer, log *zap.Logger, cfg *config.Config) *outbox.Relay {
				return outbox.NewRelay(repo, producer, log, cfg.DefaultQueryTimeout, cfg.KafkaTopic)
			},
			func(orchestrator *saga.Orchestrator, pool *pgxpool.Pool, log *zap.Logger) *saga.RecoveryWorker {
				return saga.NewRecoveryWorker(orchestrator, log, saga.WithLocker(saga.NewPostgresAdvisoryLock(pool)))
			},
		),
		fx.Invoke(registerServers),
		fx.Invoke(func(lc fx.Lifecycle, relay *outbox.Relay) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error { relay.Start(context.Background()); return nil },
				OnStop:  func(ctx context.Context) error { relay.Stop(); return nil },
			})
		}),
		fx.Invoke(func(lc fx.Lifecycle, recovery *saga.RecoveryWorker) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error { recovery.Start(context.Background()); return nil },
				OnStop:  func(ctx context.Context) error { recovery.Stop(); return nil },
			})
		}),
	)
}

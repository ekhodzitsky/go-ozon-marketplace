package app

import (
	"context"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/outbox"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository/elasticsearch"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/unitofwork"
	postgresuow "github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/unitofwork/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olivere/elastic/v7"
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
			func(cfg *config.Config) (*elastic.Client, error) {
				client, err := elastic.NewClient(elastic.SetURL(cfg.ESURL), elastic.SetSniff(false))
				if err != nil {
					return nil, err
				}
				ctx, cancel := context.WithTimeout(context.Background(), cfg.DefaultQueryTimeout)
				defer cancel()
				if _, _, err := client.Ping(cfg.ESURL).Do(ctx); err != nil {
					return nil, fmt.Errorf("elasticsearch ping: %w", err)
				}
				return client, nil
			},
			func(pool *pgxpool.Pool) *postgres.ProductPostgres {
				return postgres.NewProductPostgres(pool)
			},
			func(r *postgres.ProductPostgres) repository.ProductRepository { return r },
			func(pool *pgxpool.Pool) *postgres.OutboxPostgres {
				return postgres.NewOutboxPostgres(pool)
			},
			func(r *postgres.OutboxPostgres) repository.OutboxRepository { return r },
			func(pool *pgxpool.Pool) func() unitofwork.UnitOfWork {
				return func() unitofwork.UnitOfWork {
					return postgresuow.NewUnitOfWork(pool)
				}
			},
			elasticsearch.NewProductES,
			func(
				uowFactory func() unitofwork.UnitOfWork,
				productRepo repository.ProductRepository,
				searchRepo repository.ProductSearchRepository,
				cfg *config.Config,
			) usecase.CatalogUsecase {
				return usecase.NewCatalogUsecase(uowFactory, productRepo, searchRepo, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
			},
			grpcdelivery.NewCatalogHandler,
			func(outboxRepo repository.OutboxRepository, searchRepo repository.ProductSearchRepository, log *zap.Logger, cfg *config.Config) *outbox.Relay {
				return outbox.NewRelay(outboxRepo, searchRepo, log, cfg.DefaultQueryTimeout)
			},
		),
		fx.Invoke(registerServers),
		fx.Invoke(func(lc fx.Lifecycle, searchRepo repository.ProductSearchRepository, log *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					if err := searchRepo.EnsureIndex(ctx); err != nil {
						log.Error("failed to ensure elasticsearch index", zap.Error(err))
						return err
					}
					return nil
				},
			})
		}),
		fx.Invoke(func(lc fx.Lifecycle, relay *outbox.Relay) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					relay.Start(ctx)
					return nil
				},
				OnStop: func(ctx context.Context) error {
					relay.Stop()
					return nil
				},
			})
		}),
	)
}

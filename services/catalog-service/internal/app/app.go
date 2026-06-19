package app

import (
	"context"
	"fmt"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/fxmodules"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/txmanager"
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

func New(cfg *config.Config) *fx.App {
	return fxmodules.GRPCService(
		"catalog-service",
		cfg,
		catalogv1.RegisterCatalogServiceServer,
		grpcdelivery.NewCatalogHandler,
		fxmodules.Postgres(cfg),
		fxmodules.Runner[*outbox.Relay](),
		fx.Provide(
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
			postgres.NewProductPostgres,
			postgres.NewOutboxPostgres,
			func(pool *pgxpool.Pool) *txmanager.Manager[unitofwork.UnitOfWork] {
				return txmanager.New(pool, postgresuow.NewUnitOfWork)
			},
			elasticsearch.NewProductES,
			func(
				txm *txmanager.Manager[unitofwork.UnitOfWork],
				productRepo repository.ProductRepository,
				searchRepo repository.ProductSearchRepository,
				cfg *config.Config,
			) usecase.CatalogUsecase {
				return usecase.NewCatalogUsecase(txm, productRepo, searchRepo, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
			},
			func(outboxRepo repository.OutboxRepository, searchRepo repository.ProductSearchRepository, log *zap.Logger, cfg *config.Config) *outbox.Relay {
				return outbox.NewRelay(outboxRepo, outbox.NewESHandler(searchRepo), log, cfg.DefaultQueryTimeout)
			},
		),
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
	)
}

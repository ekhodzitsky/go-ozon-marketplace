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
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/usecase"
	"github.com/jackc/pgx/v5"
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
			elasticsearch.NewProductES,
			func(
				pool *pgxpool.Pool,
				productRepo repository.ProductRepository,
				outboxRepo repository.OutboxRepository,
				searchRepo repository.ProductSearchRepository,
				cfg *config.Config,
			) usecase.CatalogUsecase {
				return usecase.NewCatalogUsecase(
					func(ctx context.Context, fn func(pgx.Tx) error) error { return txmanager.RunTx(ctx, pool, fn) },
					productRepo,
					outboxRepo,
					searchRepo,
					cfg.DefaultCallTimeout,
					cfg.DefaultQueryTimeout,
				)
			},
			func(
				pool *pgxpool.Pool,
				outboxRepo repository.OutboxRepository,
				searchRepo repository.ProductSearchRepository,
				log *zap.Logger,
				cfg *config.Config,
			) *outbox.Relay {
				return outbox.NewRelay(
					func(ctx context.Context, fn func(pgx.Tx) error) error { return txmanager.RunTx(ctx, pool, fn) },
					outboxRepo,
					outbox.NewESHandler(searchRepo),
					log,
					cfg.DefaultQueryTimeout,
				)
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

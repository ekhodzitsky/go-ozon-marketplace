package app

import (
	"context"
	"fmt"
	"net/http"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository/elasticsearch"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olivere/elastic/v7"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func New() *fx.App {
	return fx.New(
		fx.Provide(
			config.Load,
			logger.New,
			func(cfg *config.Config) (*pgxpool.Pool, error) {
				return pkgpostgres.NewPool(context.Background(), cfg.PostgresDSN)
			},
			func(cfg *config.Config) (*elastic.Client, error) {
				return elastic.NewClient(elastic.SetURL(cfg.ESURL), elastic.SetSniff(false))
			},
			postgres.NewProductPostgres,
			elasticsearch.NewProductES,
			usecase.NewCatalogUsecase,
			grpcdelivery.NewCatalogHandler,
		),
		fx.Invoke(func(lc fx.Lifecycle, handler *grpcdelivery.CatalogHandler, cfg *config.Config, log *zap.Logger) {
			grpcServer := server.NewGRPC(cfg.GRPCPort, grpc.ChainUnaryInterceptor(middleware.LoggingUnaryInterceptor, middleware.MetricsUnaryInterceptor))

			http.Handle("/metrics", promhttp.Handler())
			go http.ListenAndServe(fmt.Sprintf(":%d", cfg.MetricsPort), nil)
			catalogv1.RegisterCatalogServiceServer(grpcServer.Server, handler)

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go func() {
						if err := grpcServer.Start(); err != nil {
							log.Error("grpc server error", zap.Error(err))
						}
					}()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					grpcServer.GracefulStop()
					return nil
				},
			})
		}),
	)
}

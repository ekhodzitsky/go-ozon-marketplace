package app

import (
	"context"
	"fmt"
	"net/http"

	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/outbox"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
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
			postgres.NewOrderPostgres,
			func(r *postgres.OrderPostgres) repository.OrderRepository { return r },
			postgres.NewOutboxPostgres,
			func(r *postgres.OutboxPostgres) repository.OutboxRepository { return r },
			func(
				orderRepo repository.OrderRepository,
				cfg *config.Config,
				log *zap.Logger,
			) *saga.Orchestrator {
				return saga.NewOrchestrator(orderRepo, cfg.InventoryAddr, cfg.PaymentAddr, log)
			},
			usecase.NewOrderUsecase,
			grpcdelivery.NewOrderHandler,
			outbox.NewRelay,
		),
		fx.Invoke(func(lc fx.Lifecycle, handler *grpcdelivery.OrderHandler, cfg *config.Config, log *zap.Logger) {
			grpcServer := server.NewGRPC(cfg.GRPCPort, grpc.ChainUnaryInterceptor(middleware.LoggingUnaryInterceptor, middleware.MetricsUnaryInterceptor, middleware.AuthUnaryInterceptor(cfg.JWTSecret)))

			http.Handle("/metrics", promhttp.Handler())
			go http.ListenAndServe(fmt.Sprintf(":%d", cfg.MetricsPort), nil)
			orderv1.RegisterOrderServiceServer(grpcServer.Server, handler)

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

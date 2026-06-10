package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
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
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork"
	postgresuow "github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func New() *fx.App {
	return fx.New(
		fx.Provide(
			config.Load,
			logger.New,
			func(cfg *config.Config) (*pgxpool.Pool, error) {
				return pkgpostgres.NewPool(context.Background(), cfg.PostgresDSN)
			},
			func(pool *pgxpool.Pool) unitofwork.UnitOfWork {
				return postgresuow.NewUnitOfWork(pool)
			},
			func(pool *pgxpool.Pool) *postgres.OrderPostgres {
				return postgres.NewOrderPostgres(pool)
			},
			func(r *postgres.OrderPostgres) repository.OrderRepository { return r },
			func(pool *pgxpool.Pool) *postgres.OutboxPostgres {
				return postgres.NewOutboxPostgres(pool)
			},
			func(r *postgres.OutboxPostgres) repository.OutboxRepository { return r },
			func(cfg *config.Config, lc fx.Lifecycle) (saga.InventoryClient, error) {
				conn, err := grpc.NewClient(
					cfg.InventoryAddr,
					grpc.WithTransportCredentials(insecure.NewCredentials()),
					grpc.WithKeepaliveParams(keepalive.ClientParameters{
						Time:                10 * time.Second,
						Timeout:             20 * time.Second,
						PermitWithoutStream: true,
					}),
				)
				if err != nil {
					return nil, err
				}
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						return conn.Close()
					},
				})
				return inventoryv1.NewInventoryServiceClient(conn), nil
			},
			func(cfg *config.Config, lc fx.Lifecycle) (saga.PaymentClient, error) {
				conn, err := grpc.NewClient(
					cfg.PaymentAddr,
					grpc.WithTransportCredentials(insecure.NewCredentials()),
					grpc.WithKeepaliveParams(keepalive.ClientParameters{
						Time:                10 * time.Second,
						Timeout:             20 * time.Second,
						PermitWithoutStream: true,
					}),
				)
				if err != nil {
					return nil, err
				}
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						return conn.Close()
					},
				})
				return paymentv1.NewPaymentServiceClient(conn), nil
			},
			func(
				orderRepo repository.OrderRepository,
				invClient saga.InventoryClient,
				payClient saga.PaymentClient,
				log *zap.Logger,
			) *saga.Orchestrator {
				return saga.NewOrchestrator(orderRepo, invClient, payClient, log)
			},
			usecase.NewOrderUsecase,
			grpcdelivery.NewOrderHandler,
			outbox.NewRelay,
		),
		fx.Invoke(func(lc fx.Lifecycle, handler *grpcdelivery.OrderHandler, cfg *config.Config, log *zap.Logger) {
			grpcServer := server.NewGRPC(cfg.GRPCPort, grpc.ChainUnaryInterceptor(middleware.LoggingUnaryInterceptor, middleware.MetricsUnaryInterceptor, middleware.AuthUnaryInterceptor(cfg.JWTSecret)))

			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			metricsServer := &http.Server{
				Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
				Handler: mux,
			}
			orderv1.RegisterOrderServiceServer(grpcServer.Server, handler)

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go func() {
						if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
							log.Error("metrics server error", zap.Error(err))
						}
					}()
					go func() {
						if err := grpcServer.Start(); err != nil {
							log.Error("grpc server error", zap.Error(err))
						}
					}()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					if err := metricsServer.Shutdown(ctx); err != nil {
						log.Error("metrics server shutdown error", zap.Error(err))
					}
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

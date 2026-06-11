package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func initCtx(cfg *config.Config) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), cfg.DefaultQueryTimeout)
}

func clientCreds(cfg *config.Config) (credentials.TransportCredentials, error) {
	if cfg.CertPath != "" {
		return server.LoadClientMTLSCredentials(
			filepath.Join(cfg.CertPath, "server-cert.pem"),
			filepath.Join(cfg.CertPath, "server-key.pem"),
			filepath.Join(cfg.CertPath, "ca-cert.pem"),
			"",
		)
	}
	return insecure.NewCredentials(), nil
}

func New() *fx.App {
	return fx.New(
		fx.Provide(
			config.Load,
			logger.New,
			func(cfg *config.Config) (*pgxpool.Pool, error) {
				ctx, cancel := initCtx(cfg)
				defer cancel()
				return pkgpostgres.NewPool(ctx, cfg.PostgresDSN)
			},
			func(pool *pgxpool.Pool) func() unitofwork.UnitOfWork {
				return func() unitofwork.UnitOfWork {
					return postgresuow.NewUnitOfWork(pool)
				}
			},
			func(pool *pgxpool.Pool) *postgres.OrderPostgres {
				return postgres.NewOrderPostgres(pool)
			},
			func(r *postgres.OrderPostgres) repository.OrderRepository { return r },
			func(pool *pgxpool.Pool) *postgres.OutboxPostgres {
				return postgres.NewOutboxPostgres(pool)
			},
			func(r *postgres.OutboxPostgres) repository.OutboxRepository { return r },
			func(pool *pgxpool.Pool) *postgres.SagaPostgres {
				return postgres.NewSagaPostgres(pool)
			},
			func(r *postgres.SagaPostgres) repository.SagaRepository { return r },
			func(cfg *config.Config, lc fx.Lifecycle) (saga.InventoryClient, error) {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.DefaultCallTimeout)
				defer cancel()
				creds, err := clientCreds(cfg)
				if err != nil {
					return nil, err
				}
				conn, err := grpc.DialContext(
					ctx,
					cfg.InventoryAddr,
					grpc.WithTransportCredentials(creds),
					grpc.WithKeepaliveParams(keepalive.ClientParameters{
						Time:                10 * time.Second,
						Timeout:             20 * time.Second,
						PermitWithoutStream: true,
					}),
					grpc.WithBlock(),
				)
				if err != nil {
					return nil, err
				}
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						return conn.Close()
					},
				})
				return grpcclient.NewInventoryClient(conn, cfg.DefaultCallTimeout), nil
			},
			func(cfg *config.Config, lc fx.Lifecycle) (saga.PaymentClient, error) {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.DefaultCallTimeout)
				defer cancel()
				creds, err := clientCreds(cfg)
				if err != nil {
					return nil, err
				}
				conn, err := grpc.DialContext(
					ctx,
					cfg.PaymentAddr,
					grpc.WithTransportCredentials(creds),
					grpc.WithKeepaliveParams(keepalive.ClientParameters{
						Time:                10 * time.Second,
						Timeout:             20 * time.Second,
						PermitWithoutStream: true,
					}),
					grpc.WithBlock(),
				)
				if err != nil {
					return nil, err
				}
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						return conn.Close()
					},
				})
				return grpcclient.NewPaymentClient(conn, cfg.DefaultCallTimeout), nil
			},
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
				uowFactory func() unitofwork.UnitOfWork,
				orderRepo repository.OrderRepository,
				outboxRepo repository.OutboxRepository,
				orchestrator *saga.Orchestrator,
				cfg *config.Config,
			) usecase.OrderUsecase {
				return usecase.NewOrderUsecase(uowFactory, orderRepo, outboxRepo, orchestrator, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
			},
			grpcdelivery.NewOrderHandler,
			func(cfg *config.Config, lc fx.Lifecycle) (outbox.Producer, error) {
				p, err := outbox.NewSaramaProducer(cfg.KafkaBrokers)
				if err != nil {
					return nil, err
				}
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						return p.Close()
					},
				})
				return p, nil
			},
			func(repo repository.OutboxRepository, producer outbox.Producer, log *zap.Logger, cfg *config.Config) *outbox.Relay {
				return outbox.NewRelay(repo, producer, log, cfg.DefaultQueryTimeout, cfg.KafkaTopic)
			},
		),
		fx.Invoke(func(lc fx.Lifecycle, handler *grpcdelivery.OrderHandler, cfg *config.Config, log *zap.Logger) {
			opts := []grpc.ServerOption{
				grpc.ChainUnaryInterceptor(middleware.LoggingUnaryInterceptor, middleware.MetricsUnaryInterceptor, middleware.AuthUnaryInterceptor(cfg.JWTSecret)),
			}
			if cfg.CertPath != "" {
				tlsOpt, err := server.LoadServerMTLSCredentials(
					filepath.Join(cfg.CertPath, "server-cert.pem"),
					filepath.Join(cfg.CertPath, "server-key.pem"),
					filepath.Join(cfg.CertPath, "ca-cert.pem"),
				)
				if err != nil {
					log.Fatal("load tls credentials", zap.Error(err))
				}
				opts = append(opts, tlsOpt)
				log.Info("tls enabled for gRPC server", zap.String("cert_path", cfg.CertPath))
			}
			grpcServer := server.NewGRPC(cfg.GRPCPort, opts...)

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
							log.Fatal("metrics server error", zap.Error(err))
						}
					}()
					go func() {
						if err := grpcServer.Start(); err != nil {
							log.Fatal("grpc server error", zap.Error(err))
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

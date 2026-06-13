package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"

	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	pkgredis "github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
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
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

func initCtx(cfg *config.Config) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), cfg.DefaultQueryTimeout)
}

func serverNameFromAddr(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func clientCreds(cfg *config.Config, addr string) (credentials.TransportCredentials, error) {
	if cfg.CertPath != "" {
		return server.LoadClientMTLSCredentials(
			filepath.Join(cfg.CertPath, "server-cert.pem"),
			filepath.Join(cfg.CertPath, "server-key.pem"),
			filepath.Join(cfg.CertPath, "ca-cert.pem"),
			serverNameFromAddr(addr),
		)
	}
	return insecure.NewCredentials(), nil
}

func serviceAuthInterceptor(jwtSecret string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, middleware.CustomClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "order-service",
				Issuer:    "go-ozon-marketplace",
				Audience:  jwt.ClaimStrings{"api-gateway"},
				ID:        uuid.New().String(),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			Role: string(middleware.RoleService),
		})
		tokenStr, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			return fmt.Errorf("sign service token: %w", err)
		}
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tokenStr)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func New() *fx.App {
	return fx.New(
		fx.Provide(
			config.Load,
			func(cfg *config.Config) (*zap.Logger, error) {
				return logger.New(cfg.LogLevel, cfg.LogFormat)
			},
			func(cfg *config.Config) (*pgxpool.Pool, error) {
				ctx, cancel := initCtx(cfg)
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
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						return client.Close()
					},
				})
				return client, nil
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
				creds, err := clientCreds(cfg, cfg.InventoryAddr)
				if err != nil {
					return nil, err
				}
				conn, err := grpc.NewClient(
					cfg.InventoryAddr,
					grpc.WithTransportCredentials(creds),
					grpc.WithChainUnaryInterceptor(serviceAuthInterceptor(cfg.JWTSecret)),
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
				return grpcclient.NewInventoryClient(conn, cfg.DefaultCallTimeout), nil
			},
			func(cfg *config.Config, lc fx.Lifecycle) (saga.PaymentClient, error) {
				creds, err := clientCreds(cfg, cfg.PaymentAddr)
				if err != nil {
					return nil, err
				}
				conn, err := grpc.NewClient(
					cfg.PaymentAddr,
					grpc.WithTransportCredentials(creds),
					grpc.WithChainUnaryInterceptor(serviceAuthInterceptor(cfg.JWTSecret)),
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
				return grpcclient.NewPaymentClient(conn, cfg.DefaultCallTimeout), nil
			},
			func(cfg *config.Config, lc fx.Lifecycle) (grpcclient.CatalogClient, error) {
				creds, err := clientCreds(cfg, cfg.CatalogAddr)
				if err != nil {
					return nil, err
				}
				conn, err := grpc.NewClient(
					cfg.CatalogAddr,
					grpc.WithTransportCredentials(creds),
					grpc.WithChainUnaryInterceptor(serviceAuthInterceptor(cfg.JWTSecret)),
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
				return grpcclient.NewCatalogClient(conn, cfg.DefaultCallTimeout), nil
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
				sagaRepo repository.SagaRepository,
				orchestrator *saga.Orchestrator,
				invClient saga.InventoryClient,
				payClient saga.PaymentClient,
				catalogClient grpcclient.CatalogClient,
				redisClient *redis.Client,
				cfg *config.Config,
			) usecase.OrderUsecase {
				return usecase.NewOrderUsecase(uowFactory, orderRepo, outboxRepo, sagaRepo, orchestrator, invClient, payClient, catalogClient, redisClient, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
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
			func(
				orchestrator *saga.Orchestrator,
				pool *pgxpool.Pool,
				log *zap.Logger,
			) *saga.RecoveryWorker {
				return saga.NewRecoveryWorker(orchestrator, log, saga.WithLocker(saga.NewPostgresAdvisoryLock(pool)))
			},
		),
		fx.Invoke(func(lc fx.Lifecycle, handler *grpcdelivery.OrderHandler, cfg *config.Config, log *zap.Logger) {
			opts := []grpc.ServerOption{
				grpc.ChainUnaryInterceptor(middleware.LoggingUnaryInterceptor, middleware.MetricsUnaryInterceptor, tracing.UnaryServerInterceptor(), middleware.AuthUnaryInterceptor(cfg.JWTSecret)),
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
				Addr:         fmt.Sprintf(":%d", cfg.MetricsPort),
				Handler:      mux,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 10 * time.Second,
				IdleTimeout:  120 * time.Second,
			}
			orderv1.RegisterOrderServiceServer(grpcServer.Server, handler)
			grpc_health_v1.RegisterHealthServer(grpcServer.Server, health.NewServer())

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
					// Use a background context so the relay keeps running after fx startup.
					relay.Start(context.Background())
					return nil
				},
				OnStop: func(ctx context.Context) error {
					relay.Stop()
					return nil
				},
			})
		}),
		fx.Invoke(func(lc fx.Lifecycle, recovery *saga.RecoveryWorker) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					// Use a background context so the worker keeps running after fx startup.
					recovery.Start(context.Background())
					return nil
				},
				OnStop: func(ctx context.Context) error {
					recovery.Stop()
					return nil
				},
			})
		}),
	)
}

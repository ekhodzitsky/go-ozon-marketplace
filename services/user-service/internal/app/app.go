package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
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
			postgres.NewUserPostgres,
			func(repo repository.UserRepository, cfg *config.Config) usecase.UserUsecase {
				// In-memory rate limiter: 5 requests per minute per email for auth endpoints.
				rl := usecase.NewMemoryRateLimiter(5, time.Minute)
				return usecase.NewUserUsecase(repo, cfg.JWTSecret, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout, rl)
			},
			grpcdelivery.NewUserHandler,
		),
		fx.Invoke(func(lc fx.Lifecycle, handler *grpcdelivery.UserHandler, cfg *config.Config, log *zap.Logger) error {
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
					return fmt.Errorf("load tls credentials: %w", err)
				}
				opts = append(opts, tlsOpt)
				log.Info("tls enabled for gRPC server", zap.String("cert_path", cfg.CertPath))
			}

			grpcServer := server.NewGRPC(cfg.GRPCPort, opts...)
			healthServer := health.NewServer()

			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			metricsServer := &http.Server{
				Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
				Handler: mux,
			}
			userv1.RegisterUserServiceServer(grpcServer.Server, handler)
			healthpb.RegisterHealthServer(grpcServer.Server, healthServer)

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
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
			return nil
		}),
	)
}

package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/consumer"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/repository/clickhouse"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/migrations"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// New builds the fx application container. It returns the constructed app
// together with any error produced during dependency resolution (e.g. invalid
// configuration or unreachable infrastructure).
// Optional fx overrides can be supplied for tests.
func New(overrides ...fx.Option) (*fx.App, error) {
	application := fx.New(
		fx.Provide(
			config.Load,
			func(cfg *config.Config) (*zap.Logger, error) {
				return logger.New(cfg.LogLevel, cfg.LogFormat)
			},
			func(cfg *config.Config) (*clickhouse.EventRepo, error) {
				return clickhouse.NewEventRepo(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePassword, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout, migrations.FS)
			},
			func(cfg *config.Config, repo *clickhouse.EventRepo, log *zap.Logger) usecase.AnalyticsUsecase {
				return usecase.NewAnalyticsUsecase(repo, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout, log)
			},
			func(cfg *config.Config, uc usecase.AnalyticsUsecase, log *zap.Logger) (*consumer.Consumer, error) {
				return consumer.NewConsumer(cfg.KafkaBrokers, cfg.KafkaConsumerGroup, cfg.KafkaTopics, uc, log)
			},
			grpcdelivery.NewAnalyticsHandler,
		),
		fx.Invoke(func(lc fx.Lifecycle, c *consumer.Consumer, log *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					c.Start(ctx)
					return nil
				},
				OnStop: func(ctx context.Context) error {
					if err := c.Close(); err != nil {
						log.Error("consumer close error", zap.Error(err))
					}
					return nil
				},
			})
		}),
		fx.Invoke(func(lc fx.Lifecycle, handler *grpcdelivery.AnalyticsHandler, cfg *config.Config, log *zap.Logger, uc usecase.AnalyticsUsecase) {
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
				Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
				Handler: mux,
			}
			analyticsv1.RegisterAnalyticsServiceServer(grpcServer.Server, handler)
			healthSrv := health.NewServer()
			healthpb.RegisterHealthServer(grpcServer.Server, healthSrv)

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					if cfg.GRPCPort == 0 && cfg.MetricsPort == 0 {
						log.Info("server startup disabled (ports set to 0)")
						return nil
					}
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
					if err := uc.Flush(ctx); err != nil {
						log.Error("flush analytics buffer error", zap.Error(err))
					}
					if cfg.GRPCPort == 0 && cfg.MetricsPort == 0 {
						return nil
					}
					if err := metricsServer.Shutdown(ctx); err != nil {
						log.Error("metrics server shutdown error", zap.Error(err))
					}
					grpcServer.GracefulStop()
					return nil
				},
			})
		}),
		fx.Options(overrides...),
	)

	return application, application.Err()
}

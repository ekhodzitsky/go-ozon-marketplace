package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	notificationv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/notification/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/consumer"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/email"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/usecase"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpchealthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func New() *fx.App {
	return fx.New(
		fx.Provide(
			config.Load,
			func(cfg *config.Config) (*zap.Logger, error) {
				return logger.New(cfg.LogLevel, cfg.LogFormat)
			},
			func(cfg *config.Config, log *zap.Logger) email.Provider {
				if cfg.SMTPHost != "" {
					return email.NewSMTPProvider(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom, cfg.SMTPUser, cfg.SMTPPassword)
				}
				return email.NewLogProvider(log)
			},
			func(log *zap.Logger, cfg *config.Config, provider email.Provider) usecase.NotificationUsecase {
				return usecase.NewNotificationUsecase(log, provider, cfg.DefaultCallTimeout, cfg.DefaultQueryTimeout)
			},
			func(cfg *config.Config, uc usecase.NotificationUsecase, log *zap.Logger) (*consumer.Consumer, error) {
				return consumer.NewConsumer(cfg.KafkaBrokers, cfg.KafkaConsumerGroup, cfg.KafkaTopics, cfg.KafkaDLQTopic, uc, log)
			},
			grpcdelivery.NewNotificationHandler,
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
		fx.Invoke(func(lc fx.Lifecycle, handler *grpcdelivery.NotificationHandler, cfg *config.Config, log *zap.Logger) {
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

			healthSrv := health.NewServer()
			grpchealthv1.RegisterHealthServer(grpcServer.Server, healthSrv)
			healthSrv.SetServingStatus("", grpchealthv1.HealthCheckResponse_SERVING)

			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			metricsServer := &http.Server{
				Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
				Handler: mux,
			}
			notificationv1.RegisterNotificationServiceServer(grpcServer.Server, handler)

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
	)
}

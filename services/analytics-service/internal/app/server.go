package app

import (
	"context"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func registerServers(lc fx.Lifecycle, handler *grpcdelivery.AnalyticsHandler, cfg *config.Config, log *zap.Logger, uc usecase.AnalyticsUsecase) {
	if cfg.GRPCPort == 0 && cfg.MetricsPort == 0 {
		log.Info("server startup disabled (ports set to 0)")
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return flushAnalytics(ctx, uc, log)
			},
		})
		return
	}

	interceptors := []grpc.UnaryServerInterceptor{
		middleware.LoggingUnaryInterceptor,
		middleware.MetricsUnaryInterceptor,
		tracing.UnaryServerInterceptor(),
		middleware.AuthUnaryInterceptor(cfg.JWTSecret),
	}

	grpcServer, metricsServer, err := server.StartService(server.ServiceConfig{
		GRPCPort:    cfg.GRPCPort,
		MetricsPort: cfg.MetricsPort,
		CertPath:    cfg.CertPath,
	}, func(s *grpc.Server) {
		analyticsv1.RegisterAnalyticsServiceServer(s, handler)
	}, interceptors, log)
	if err != nil {
		log.Fatal("start service", zap.Error(err))
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := metricsServer.Start(); err != nil {
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
			_ = flushAnalytics(ctx, uc, log)
			if err := metricsServer.Shutdown(ctx); err != nil {
				log.Error("metrics server shutdown error", zap.Error(err))
			}
			grpcServer.GracefulStop()
			return nil
		},
	})
}

func flushAnalytics(ctx context.Context, uc usecase.AnalyticsUsecase, log *zap.Logger) error {
	if err := uc.Flush(ctx); err != nil {
		log.Error("flush analytics buffer error", zap.Error(err))
		return err
	}
	return nil
}

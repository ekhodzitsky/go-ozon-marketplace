package app

import (
	"context"
	"errors"
	"net/http"

	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/config"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/delivery/grpc"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func registerServers(lc fx.Lifecycle, handler *grpcdelivery.OrderHandler, cfg *config.Config, log *zap.Logger) {
	protoValidateInterceptor, err := middleware.ProtoValidateInterceptor()
	if err != nil {
		log.Fatal("create protovalidate interceptor", zap.Error(err))
	}

	verifier := auth.NewJWTVerifier(cfg.JWTSecret)
	interceptors := []grpc.UnaryServerInterceptor{
		middleware.LoggingUnaryInterceptor,
		middleware.MetricsUnaryInterceptor,
		protoValidateInterceptor,
		tracing.UnaryServerInterceptor(),
		middleware.AuthUnaryInterceptor(verifier),
	}

	grpcServer, metricsServer, err := server.StartService(server.ServiceConfig{
		GRPCPort:    cfg.GRPCPort,
		MetricsPort: cfg.MetricsPort,
		CertPath:    cfg.CertPath,
	}, func(s *grpc.Server) {
		orderv1.RegisterOrderServiceServer(s, handler)
	}, interceptors, log)
	if err != nil {
		log.Fatal("start service", zap.Error(err))
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := metricsServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
}

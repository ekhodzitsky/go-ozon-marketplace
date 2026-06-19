package server

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// RegisterGRPCService подключает стандартный gRPC-сервис к fx-приложению.
// Собирает интерцепторы логирования, метрик, protovalidate, трассировки и JWT-auth,
// стартует gRPC-сервер и sidecar метрик на lifecycle start и аккуратно останавливает
// их на lifecycle stop.
func RegisterGRPCService(lc fx.Lifecycle, cfg ServiceConfig, register RegisterFn, jwtSecret string, log *zap.Logger) {
	protoValidateInterceptor, err := middleware.ProtoValidateInterceptor()
	if err != nil {
		log.Fatal("create protovalidate interceptor", zap.Error(err))
	}

	interceptors := []grpc.UnaryServerInterceptor{
		middleware.LoggingUnaryInterceptor,
		middleware.MetricsUnaryInterceptor,
		protoValidateInterceptor,
		tracing.UnaryServerInterceptor(),
		middleware.AuthUnaryInterceptor(auth.NewJWTVerifier(jwtSecret)),
	}

	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err := RunService(ctx, cfg, register, interceptors, log); err != nil {
					log.Fatal("run service", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

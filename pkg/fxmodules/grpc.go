package fxmodules

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// GRPCServerConfig — настройки для запуска обычного gRPC-сервиса.
type GRPCServerConfig interface {
	GetGRPCPort() int
	GetMetricsPort() int
	GetCertPath() string
	GetJWTSecret() string
}

// GRPCServer подключает стандартный gRPC-сервис к fx-приложению.
// Вызывающий должен предоставить server.RegisterFn, который регистрирует
// реализации сервисов на *grpc.Server.
// Если оба порта равны нулю, сервер не стартует (удобно для воркеров).
// Настройки берутся из GRPCServerConfig через DI, чтобы тесты могли их переопределять.
func GRPCServer(cfg GRPCServerConfig) fx.Option {
	return fx.Options(
		fx.Provide(func() GRPCServerConfig { return cfg }),
		fx.Invoke(func(lc fx.Lifecycle, cfg GRPCServerConfig, register server.RegisterFn, log *zap.Logger) {
			if cfg.GetGRPCPort() == 0 && cfg.GetMetricsPort() == 0 {
				log.Info("gRPC server startup disabled (ports set to 0)")
				return
			}
			server.RegisterGRPCService(lc, server.ServiceConfig{
				GRPCPort:    cfg.GetGRPCPort(),
				MetricsPort: cfg.GetMetricsPort(),
				CertPath:    cfg.GetCertPath(),
			}, register, cfg.GetJWTSecret(), log)
		}),
	)
}

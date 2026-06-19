package fxmodules

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// GRPCServerConfig exposes the settings needed to start a standard gRPC service.
type GRPCServerConfig interface {
	GetGRPCPort() int
	GetMetricsPort() int
	GetCertPath() string
	GetJWTSecret() string
}

// GRPCServer wires a standard gRPC service into an fx application.
// The caller must provide a server.RegisterFn that registers service
// implementations on the *grpc.Server.
// If both ports are zero the server is not started (useful for worker-only modes).
// Settings are resolved from GRPCServerConfig via DI, so tests can override them.
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

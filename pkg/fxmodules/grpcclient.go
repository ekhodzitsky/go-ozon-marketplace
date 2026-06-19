package fxmodules

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/grpcclient"
	"github.com/sony/gobreaker"
	"go.uber.org/fx"
)

// GRPCClientFactoryConfig exposes TLS/auth settings needed by the gRPC client factory.
type GRPCClientFactoryConfig interface {
	GetCertPath() string
	GetJWTSecret() string
	GetInsecureSkipTLS() bool
}

// GRPCClientFactory provides a shared pkg/grpcclient.Factory as an fx module.
// userAuth selects user-token auth; otherwise service-to-service auth is used
// when JWTSecret and ServiceName are configured.
// Settings are resolved from GRPCClientFactoryConfig via DI, so tests can override them.
func GRPCClientFactory(cfg GRPCClientFactoryConfig, serviceName string, userAuth bool) fx.Option {
	return fx.Options(
		fx.Provide(func() GRPCClientFactoryConfig { return cfg }),
		fx.Provide(func(cb *gobreaker.CircuitBreaker, cfg GRPCClientFactoryConfig) *grpcclient.Factory {
			return grpcclient.NewFactory(grpcclient.Config{
				CertPath:        cfg.GetCertPath(),
				JWTSecret:       cfg.GetJWTSecret(),
				ServiceName:     serviceName,
				UserAuth:        userAuth,
				InsecureSkipTLS: cfg.GetInsecureSkipTLS(),
			}, cb)
		}),
	)
}

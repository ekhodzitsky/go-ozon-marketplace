package fxmodules

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/grpcclient"
	"github.com/sony/gobreaker"
	"go.uber.org/fx"
)

// GRPCClientFactoryConfig — TLS/Auth-настройки фабрики gRPC-клиентов.
type GRPCClientFactoryConfig interface {
	GetCertPath() string
	GetJWTSecret() string
	GetInsecureSkipTLS() bool
}

// GRPCClientFactory отдаёт общую pkg/grpcclient.Factory как fx-модуль.
// userAuth включает авторизацию по пользовательскому токену; иначе используется
// сервисная авторизация, если заданы JWTSecret и ServiceName.
// Настройки берутся из GRPCClientFactoryConfig через DI, чтобы тесты могли их переопределять.
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

//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
)

// InitializeApp constructs App from config.
func InitializeApp(cfg *config.Config) (*App, func(), error) {
	wire.Build(
		provideContext,
		provideLogger,
		provideCircuitBreaker,
		provideClientFactory,
		provideRedis,
		provideFeatureFlags,
		provideRateLimiter,
		provideHub,
		provideAdminHandler,
		provideResolver,
		provideHTTPServer,
		provideMetricsServer,
		wire.Struct(new(App), "*"),
	)
	return nil, nil, nil
}

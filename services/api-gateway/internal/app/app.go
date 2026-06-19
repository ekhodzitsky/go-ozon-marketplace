package app

import (
	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/fxmodules"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/server"
	"go.uber.org/fx"
)

// New builds the API gateway fx application. Configuration is supplied by the caller.
func New(cfg *config.Config) *fx.App {
	return fx.New(
		fxmodules.Logger(cfg),
		fxmodules.Config(cfg),
		fxmodules.Redis(cfg),
		fxmodules.CircuitBreaker("grpc-client"),
		fxmodules.GRPCClientFactory(cfg, "api-gateway", true),
		fxmodules.GRPCClient(cfg.UserServiceAddr, userv1.NewUserServiceClient),
		fxmodules.GRPCClient(cfg.CatalogServiceAddr, catalogv1.NewCatalogServiceClient),
		fxmodules.GRPCClient(cfg.OrderServiceAddr, orderv1.NewOrderServiceClient),
		fxmodules.GRPCClient(cfg.InventoryServiceAddr, inventoryv1.NewInventoryServiceClient),
		fxmodules.GRPCClient(cfg.PaymentServiceAddr, paymentv1.NewPaymentServiceClient),
		fxmodules.GRPCClient(cfg.AnalyticsServiceAddr, analyticsv1.NewAnalyticsServiceClient),
		fxmodules.HTTPServer[*server.HTTP]("gateway"),
		fxmodules.HTTPServer[*server.Metrics]("metrics"),
		fx.Provide(
			provideFeatureFlags,
			provideRateLimiter,
			provideHub,
			provideAdminHandler,
			provideResolver,
			server.NewHTTP,
			func(cfg *config.Config) *server.Metrics { return server.NewMetrics(cfg.MetricsPort) },
		),
	)
}

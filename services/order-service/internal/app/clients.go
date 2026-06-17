package app

import (
	"context"
	"time"

	pkggrpcclient "github.com/ekhodzitsky/go-ozon-marketplace/pkg/grpcclient"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/infrastructure/grpcclient"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/sony/gobreaker"
)

func provideCircuitBreaker() *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "order-service-downstream",
		MaxRequests: 2,
		Interval:    0,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})
}

func provideClientFactory(cfg *config.Config, cb *gobreaker.CircuitBreaker) *pkggrpcclient.Factory {
	return pkggrpcclient.NewFactory(pkggrpcclient.Config{
		CertPath:    cfg.CertPath,
		JWTSecret:   cfg.JWTSecret,
		ServiceName: "order-service",
	}, cb)
}

func provideInventoryClient(ctx context.Context, factory *pkggrpcclient.Factory, cfg *config.Config) (saga.InventoryClient, error) {
	conn, err := factory.NewClient(ctx, cfg.InventoryAddr)
	if err != nil {
		return nil, err
	}
	return grpcclient.NewInventoryClient(conn, cfg.DefaultCallTimeout), nil
}

func providePaymentClient(ctx context.Context, factory *pkggrpcclient.Factory, cfg *config.Config) (saga.PaymentClient, error) {
	conn, err := factory.NewClient(ctx, cfg.PaymentAddr)
	if err != nil {
		return nil, err
	}
	return grpcclient.NewPaymentClient(conn, cfg.DefaultCallTimeout), nil
}

func provideCatalogClient(ctx context.Context, factory *pkggrpcclient.Factory, cfg *config.Config) (grpcclient.CatalogClient, error) {
	conn, err := factory.NewClient(ctx, cfg.CatalogAddr)
	if err != nil {
		return nil, err
	}
	return grpcclient.NewCatalogClient(conn, cfg.DefaultCallTimeout), nil
}

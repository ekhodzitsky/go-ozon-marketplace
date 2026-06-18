package app

import (
	"context"
	"fmt"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/abtesting"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/grpcclient"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type serviceConnections struct {
	user      *grpc.ClientConn
	catalog   *grpc.ClientConn
	order     *grpc.ClientConn
	inventory *grpc.ClientConn
	payment   *grpc.ClientConn
	analytics *grpc.ClientConn
}

func (c *serviceConnections) close() {
	closeConn(c.user)
	closeConn(c.catalog)
	closeConn(c.order)
	closeConn(c.inventory)
	closeConn(c.payment)
	closeConn(c.analytics)
}

func closeConn(conn *grpc.ClientConn) {
	if conn != nil {
		_ = conn.Close()
	}
}

func provideResolver(
	ctx context.Context,
	factory *grpcclient.Factory,
	ffEngine *featureflags.Engine,
	hub *ws.Hub,
	redisClient *redis.Client,
	cfg *config.Config,
) (*graph.Resolver, func(), error) {
	conns := &serviceConnections{}
	cleanup := func() {
		conns.close()
	}

	addrs := []struct {
		name string
		addr string
		set  func(*grpc.ClientConn)
	}{
		{"user", cfg.UserServiceAddr, func(c *grpc.ClientConn) { conns.user = c }},
		{"catalog", cfg.CatalogServiceAddr, func(c *grpc.ClientConn) { conns.catalog = c }},
		{"order", cfg.OrderServiceAddr, func(c *grpc.ClientConn) { conns.order = c }},
		{"inventory", cfg.InventoryServiceAddr, func(c *grpc.ClientConn) { conns.inventory = c }},
		{"payment", cfg.PaymentServiceAddr, func(c *grpc.ClientConn) { conns.payment = c }},
		{"analytics", cfg.AnalyticsServiceAddr, func(c *grpc.ClientConn) { conns.analytics = c }},
	}

	for _, svc := range addrs {
		conn, err := factory.NewClient(ctx, svc.addr)
		if err != nil {
			return nil, cleanup, fmt.Errorf("%s client: %w", svc.name, err)
		}
		svc.set(conn)
	}

	resolver := &graph.Resolver{
		UserService:        userv1.NewUserServiceClient(conns.user),
		CatalogService:     catalogv1.NewCatalogServiceClient(conns.catalog),
		OrderService:       orderv1.NewOrderServiceClient(conns.order),
		InventoryService:   inventoryv1.NewInventoryServiceClient(conns.inventory),
		PaymentService:     paymentv1.NewPaymentServiceClient(conns.payment),
		AnalyticsService:   analyticsv1.NewAnalyticsServiceClient(conns.analytics),
		FeatureFlagsEngine: ffEngine,
		ABExperiments:      defaultExperiments(),
		Hub:                hub,
		Redis:              redisClient,
		CallTimeout:        cfg.DefaultCallTimeout,
		QueryTimeout:       cfg.DefaultQueryTimeout,
	}
	return resolver, cleanup, nil
}

func defaultExperiments() []*abtesting.Experiment {
	return []*abtesting.Experiment{
		{
			Name: "checkout-button-color",
			Variations: []abtesting.Variation{
				{Name: "control", Weight: 50},
				{Name: "green", Weight: 50},
			},
		},
		{
			Name: "search-algorithm",
			Variations: []abtesting.Variation{
				{Name: "v1", Weight: 70},
				{Name: "v2", Weight: 30},
			},
		},
	}
}

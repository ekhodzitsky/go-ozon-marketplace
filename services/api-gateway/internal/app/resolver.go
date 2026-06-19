package app

import (
	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/abtesting"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/redis/go-redis/v9"
)

func provideResolver(
	user userv1.UserServiceClient,
	catalog catalogv1.CatalogServiceClient,
	order orderv1.OrderServiceClient,
	inventory inventoryv1.InventoryServiceClient,
	payment paymentv1.PaymentServiceClient,
	analytics analyticsv1.AnalyticsServiceClient,
	flags *featureflags.FeatureFlags,
	redisClient *redis.Client,
	cfg *config.Config,
) *graph.Resolver {
	return &graph.Resolver{
		UserService:      user,
		CatalogService:   catalog,
		OrderService:     order,
		InventoryService: inventory,
		PaymentService:   payment,
		AnalyticsService: analytics,
		FeatureFlags:     flags,
		ABExperiments:    defaultExperiments(),
		Redis:            redisClient,
		CallTimeout:      cfg.DefaultCallTimeout,
		QueryTimeout:     cfg.DefaultQueryTimeout,
	}
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

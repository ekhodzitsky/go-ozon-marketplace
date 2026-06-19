package graph

import (
	"time"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/abtesting"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/redis/go-redis/v9"
)

// Resolver serves as dependency injection for the app.
type Resolver struct {
	UserService        userv1.UserServiceClient
	CatalogService     catalogv1.CatalogServiceClient
	OrderService       orderv1.OrderServiceClient
	InventoryService   inventoryv1.InventoryServiceClient
	PaymentService     paymentv1.PaymentServiceClient
	AnalyticsService   analyticsv1.AnalyticsServiceClient
	FeatureFlags  *featureflags.FeatureFlags
	ABExperiments []*abtesting.Experiment
	Redis         *redis.Client
	CallTimeout        time.Duration
	QueryTimeout       time.Duration
}

package graph

import (
	"context"
	"fmt"
	"math"
	"time"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/abtesting"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph/model"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
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
	FeatureFlagsEngine *featureflags.Engine
	ABExperiments      []*abtesting.Experiment
	Hub                *ws.Hub
	Redis              *redis.Client
	CallTimeout        time.Duration
	QueryTimeout       time.Duration
}

func (r *Resolver) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, r.CallTimeout)
}

func (r *Resolver) withQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, r.QueryTimeout)
}

func dollarsToCents(dollars float64) int64 {
	return int64(math.Round(dollars * 100))
}

func centsToDollars(cents int64) float64 {
	return float64(cents) / 100.0
}

func protoProductToModel(p *catalogv1.Product) *model.Product {
	if p == nil {
		return nil
	}
	return &model.Product{
		ID:          p.ProductId,
		Name:        p.Name,
		Description: p.Description,
		Price:       centsToDollars(p.PriceCents),
		Categories:  p.Categories,
		CreatedAt:   p.CreatedAt,
	}
}

func protoOrderItemToModel(item *orderv1.OrderItem) *model.OrderItem {
	if item == nil {
		return nil
	}
	return &model.OrderItem{
		ProductID: item.ProductId,
		Quantity:  item.Quantity,
		Price:     centsToDollars(item.PriceCents),
	}
}

func protoOrderToModel(o *orderv1.Order) *model.Order {
	if o == nil {
		return nil
	}
	items := make([]*model.OrderItem, len(o.Items))
	for i, item := range o.Items {
		items[i] = protoOrderItemToModel(item)
	}
	return &model.Order{
		ID:          o.OrderId,
		UserID:      o.UserId,
		Items:       items,
		TotalAmount: centsToDollars(o.TotalAmountCents),
		Status:      orderStatusString(o.Status),
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
	}
}

func orderStatusString(s orderv1.OrderStatus) string {
	switch s {
	case orderv1.OrderStatus_ORDER_STATUS_PENDING:
		return "pending"
	case orderv1.OrderStatus_ORDER_STATUS_AWAITING_PAYMENT:
		return "awaiting_payment"
	case orderv1.OrderStatus_ORDER_STATUS_PAID:
		return "paid"
	case orderv1.OrderStatus_ORDER_STATUS_PROCESSING:
		return "processing"
	case orderv1.OrderStatus_ORDER_STATUS_SHIPPED:
		return "shipped"
	case orderv1.OrderStatus_ORDER_STATUS_DELIVERED:
		return "delivered"
	case orderv1.OrderStatus_ORDER_STATUS_CANCELLED:
		return "cancelled"
	case orderv1.OrderStatus_ORDER_STATUS_REFUNDED:
		return "refunded"
	default:
		return ""
	}
}

func requireAuth(ctx context.Context) (string, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok || userID == "" {
		return "", fmt.Errorf("unauthenticated")
	}
	return userID, nil
}

func isAdmin(ctx context.Context) bool {
	role, _ := middleware.GetRole(ctx)
	return role == auth.RoleAdmin
}

func requireOwnerOrAdmin(ctx context.Context, ownerID string) error {
	userID, err := requireAuth(ctx)
	if err != nil {
		return err
	}
	if userID == ownerID || isAdmin(ctx) {
		return nil
	}
	return fmt.Errorf("forbidden")
}

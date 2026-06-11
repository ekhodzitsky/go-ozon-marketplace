package graph

import (
	"context"
	"time"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph/model"
)

// Resolver serves as dependency injection for the app.
type Resolver struct {
	UserService      userv1.UserServiceClient
	CatalogService   catalogv1.CatalogServiceClient
	OrderService     orderv1.OrderServiceClient
	InventoryService inventoryv1.InventoryServiceClient
	PaymentService   paymentv1.PaymentServiceClient
	CallTimeout      time.Duration
	QueryTimeout     time.Duration
}

func (r *Resolver) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, r.CallTimeout)
}

func (r *Resolver) withQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, r.QueryTimeout)
}

func protoProductToModel(p *catalogv1.Product) *model.Product {
	if p == nil {
		return nil
	}
	return &model.Product{
		ID:          p.ProductId,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
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
		Price:     item.Price,
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
		TotalAmount: o.TotalAmount,
		Status:      o.Status,
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
	}
}

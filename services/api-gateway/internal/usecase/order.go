package usecase

import (
	"context"
	"fmt"
	"time"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/abtesting"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph/model"
	"github.com/google/uuid"
)

func CreateOrder(ctx context.Context, orderClient orderv1.OrderServiceClient, catalogClient catalogv1.CatalogServiceClient, analyticsClient analyticsv1.AnalyticsServiceClient, experiments []*abtesting.Experiment, items []*model.OrderItemInput, callTimeout, queryTimeout time.Duration) (string, error) {
	callerID, err := requireAuth(ctx)
	if err != nil {
		return "", err
	}

	protoItems := make([]*orderv1.OrderItem, len(items))
	for i, item := range items {
		protoItems[i] = &orderv1.OrderItem{
			ProductId:  item.ProductID,
			Quantity:   item.Quantity,
			PriceCents: dollarsToCents(item.Price),
		}
	}

	priceCtx, priceCancel := context.WithTimeout(ctx, queryTimeout)
	defer priceCancel()
	for _, item := range protoItems {
		prodResp, err := catalogClient.GetProduct(priceCtx, &catalogv1.GetProductRequest{ProductId: item.ProductId})
		if err != nil {
			return "", fmt.Errorf("catalog lookup: %w", err)
		}
		if prodResp.Product == nil {
			return "", fmt.Errorf("%w: product %s not found", apperrors.ErrInvalidArgument, item.ProductId)
		}
		if item.PriceCents != prodResp.Product.PriceCents {
			return "", fmt.Errorf("%w: price mismatch for product %s: requested %d, catalog %d", apperrors.ErrInvalidArgument, item.ProductId, item.PriceCents, prodResp.Product.PriceCents)
		}
	}

	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := orderClient.CreateOrder(callCtx, &orderv1.CreateOrderRequest{
		Items:          protoItems,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		return "", err
	}

	if analyticsClient != nil {
		trackCheckoutConversion(context.WithoutCancel(ctx), analyticsClient, experiments, callerID)
	}

	return resp.OrderId, nil
}

func CancelOrder(ctx context.Context, client orderv1.OrderServiceClient, orderID string, callTimeout, queryTimeout time.Duration) (bool, error) {
	if _, err := requireAuth(ctx); err != nil {
		return false, err
	}

	getCtx, getCancel := context.WithTimeout(ctx, queryTimeout)
	defer getCancel()
	getResp, err := client.GetOrder(getCtx, &orderv1.GetOrderRequest{OrderId: orderID})
	if err != nil {
		return false, err
	}
	if getResp.Order == nil {
		return false, fmt.Errorf("order not found")
	}
	if err := requireOwnerOrAdmin(ctx, getResp.Order.UserId); err != nil {
		return false, err
	}

	cancelCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	_, err = client.CancelOrder(cancelCtx, &orderv1.CancelOrderRequest{OrderId: orderID})
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetOrder(ctx context.Context, client orderv1.OrderServiceClient, id string, timeout time.Duration) (*model.Order, error) {
	if _, err := requireAuth(ctx); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.GetOrder(ctx, &orderv1.GetOrderRequest{OrderId: id})
	if err != nil {
		return nil, err
	}
	if resp.Order == nil {
		return nil, fmt.Errorf("order not found")
	}
	if err := requireOwnerOrAdmin(ctx, resp.Order.UserId); err != nil {
		return nil, err
	}
	return protoOrderToModel(resp.Order), nil
}

func ListOrders(ctx context.Context, client orderv1.OrderServiceClient, userID string, page, pageSize *int32, timeout time.Duration) (*model.OrderConnection, error) {
	if err := requireOwnerOrAdmin(ctx, userID); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req := &orderv1.ListOrdersRequest{Page: 1, PageSize: 10}
	if page != nil && *page > 0 {
		req.Page = *page
	}
	if pageSize != nil && *pageSize > 0 {
		req.PageSize = *pageSize
	}
	resp, err := client.ListOrders(ctx, req)
	if err != nil {
		return nil, err
	}
	orders := make([]*model.Order, len(resp.Orders))
	for i, o := range resp.Orders {
		orders[i] = protoOrderToModel(o)
	}
	return &model.OrderConnection{Orders: orders, Total: resp.Total}, nil
}

func protoOrderToModel(o *orderv1.Order) *model.Order {
	if o == nil {
		return nil
	}
	items := make([]*model.OrderItem, len(o.Items))
	for i, item := range o.Items {
		items[i] = &model.OrderItem{
			ProductID: item.ProductId,
			Quantity:  item.Quantity,
			Price:     centsToDollars(item.PriceCents),
		}
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

func trackCheckoutConversion(ctx context.Context, analyticsClient analyticsv1.AnalyticsServiceClient, experiments []*abtesting.Experiment, callerID string) {
	for _, exp := range experiments {
		if exp.Name == "checkout-button-color" {
			go func(experiment, variation string) {
				trackCtx, trackCancel := context.WithTimeout(ctx, 3*time.Second)
				defer trackCancel()
				_, _ = analyticsClient.TrackABTestEvent(trackCtx, &analyticsv1.TrackABTestEventRequest{
					Experiment: experiment,
					Variation:  variation,
					UserId:     callerID,
					Conversion: true,
				})
			}(exp.Name, exp.Assign(callerID))
			break
		}
	}
}

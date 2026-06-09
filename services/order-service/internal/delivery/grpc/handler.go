package grpc

import (
	"context"
	"time"

	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/usecase"
	"github.com/google/uuid"
)

type OrderHandler struct {
	orderv1.UnimplementedOrderServiceServer
	usecase *usecase.OrderUsecase
}

func NewOrderHandler(uc *usecase.OrderUsecase) *OrderHandler {
	return &OrderHandler{usecase: uc}
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, err
	}

	items := make([]domain.OrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		productID, err := uuid.Parse(item.ProductId)
		if err != nil {
			return nil, err
		}
		items = append(items, domain.OrderItem{
			ProductID: productID,
			Quantity:  int(item.Quantity),
			Price:     item.Price,
		})
	}

	orderID, err := h.usecase.CreateOrder(ctx, userID, items)
	if err != nil {
		return nil, err
	}

	return &orderv1.CreateOrderResponse{
		OrderId: orderID.String(),
		Status:  "pending",
	}, nil
}

func (h *OrderHandler) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, err
	}

	order, err := h.usecase.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	return &orderv1.GetOrderResponse{
		Order: mapOrderToProto(order),
	}, nil
}

func (h *OrderHandler) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, err
	}

	page := int(req.Page)
	if page < 1 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize < 1 {
		pageSize = 10
	}

	orders, total, err := h.usecase.ListOrders(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	protoOrders := make([]*orderv1.Order, 0, len(orders))
	for _, o := range orders {
		protoOrders = append(protoOrders, mapOrderToProto(&o))
	}

	return &orderv1.ListOrdersResponse{
		Orders: protoOrders,
		Total:  int32(total),
	}, nil
}

func mapOrderToProto(order *domain.Order) *orderv1.Order {
	items := make([]*orderv1.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, &orderv1.OrderItem{
			ProductId: item.ProductID.String(),
			Quantity:  int32(item.Quantity),
			Price:     item.Price,
		})
	}
	return &orderv1.Order{
		OrderId:     order.ID.String(),
		UserId:      order.UserID.String(),
		Items:       items,
		TotalAmount: order.TotalAmount,
		Status:      order.Status,
		CreatedAt:   order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   order.UpdatedAt.Format(time.RFC3339),
	}
}

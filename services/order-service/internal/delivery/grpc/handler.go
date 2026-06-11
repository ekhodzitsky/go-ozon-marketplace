package grpc

import (
	"context"
	"errors"
	"time"

	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/usecase"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
)

type OrderHandler struct {
	orderv1.UnimplementedOrderServiceServer
	usecase usecase.OrderUsecase
}

func NewOrderHandler(uc usecase.OrderUsecase) *OrderHandler {
	return &OrderHandler{usecase: uc}
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	authUserID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
	}
	if req.UserId != "" && req.UserId != authUserID {
		return nil, status.Error(codes.PermissionDenied, "user_id mismatch")
	}
	userID, err := uuid.Parse(authUserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user_id in token: %v", err)
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
			Price:     int64(item.Price * 100),
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
	authUserID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
	}

	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, err
	}

	order, err := h.usecase.GetOrder(ctx, orderID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "get order: %v", err)
	}

	if order.UserID.String() != authUserID {
		return nil, status.Error(codes.PermissionDenied, "order does not belong to user")
	}

	return &orderv1.GetOrderResponse{
		Order: mapOrderToProto(order),
	}, nil
}

func (h *OrderHandler) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	authUserID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
	}
	if req.UserId != "" && req.UserId != authUserID {
		return nil, status.Error(codes.PermissionDenied, "user_id mismatch")
	}
	userID, err := uuid.Parse(authUserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user_id in token: %v", err)
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
			Price:     float64(item.Price),
		})
	}
	return &orderv1.Order{
		OrderId:     order.ID.String(),
		UserId:      order.UserID.String(),
		Items:       items,
		TotalAmount: float64(order.TotalAmount),
		Status:      order.Status,
		CreatedAt:   order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   order.UpdatedAt.Format(time.RFC3339),
	}
}

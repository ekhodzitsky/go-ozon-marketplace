package grpc

import (
	"context"
	"errors"
	"time"

	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/validation"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/usecase"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	if req.IdempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "missing idempotency_key")
	}

	userID, err := uuid.Parse(authUserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user_id in token: %v", err)
	}

	items := make([]domain.OrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		if err := validation.ValidateQuantity(item.Quantity); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if item.PriceCents <= 0 {
			return nil, status.Error(codes.InvalidArgument, "price must be greater than 0")
		}
		productID, err := uuid.Parse(item.ProductId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid product_id")
		}
		items = append(items, domain.OrderItem{
			ProductID: productID,
			Quantity:  int(item.Quantity),
			Price:     item.PriceCents,
		})
	}

	orderID, err := h.usecase.CreateOrder(ctx, userID, items, req.IdempotencyKey)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidArgument) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, apperrors.ToStatus(err)
	}

	return &orderv1.CreateOrderResponse{
		OrderId: orderID.String(),
		Status:  orderv1.OrderStatus_ORDER_STATUS_PENDING,
	}, nil
}

func (h *OrderHandler) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	authUserID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
	}

	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}

	order, err := h.usecase.GetOrder(ctx, orderID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "get order: %v", err)
	}

	role, _ := middleware.GetRole(ctx)
	if order.UserID.String() != authUserID && role != middleware.RoleAdmin {
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
	userID, err := uuid.Parse(authUserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user_id in token: %v", err)
	}

	if err := validation.ValidatePageSize(req.PageSize); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
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
		return nil, apperrors.ToStatus(err)
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

func (h *OrderHandler) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	authUserID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
	}

	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}

	order, err := h.usecase.GetOrder(ctx, orderID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "get order: %v", err)
	}

	role, _ := middleware.GetRole(ctx)
	if order.UserID.String() != authUserID && role != middleware.RoleAdmin {
		return nil, status.Error(codes.PermissionDenied, "order does not belong to user")
	}

	if err := h.usecase.CancelOrder(ctx, orderID); err != nil {
		if errors.Is(err, apperrors.ErrInvalidArgument) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, apperrors.ErrFailedPrecondition) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "cancel order: %v", err)
	}

	return &orderv1.CancelOrderResponse{Success: true}, nil
}

func (h *OrderHandler) UpdateOrderStatus(ctx context.Context, req *orderv1.UpdateOrderStatusRequest) (*orderv1.UpdateOrderStatusResponse, error) {
	if _, ok := middleware.GetUserID(ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
	}

	role, _ := middleware.GetRole(ctx)
	if role != middleware.RoleAdmin && role != middleware.RoleService {
		return nil, status.Error(codes.PermissionDenied, "only admin or service can update order status")
	}

	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}

	if req.Status == orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "invalid order status")
	}

	if err := h.usecase.UpdateOrderStatus(ctx, orderID, protoStatusToDomain(req.Status)); err != nil {
		if errors.Is(err, apperrors.ErrInvalidArgument) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, apperrors.ErrFailedPrecondition) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "update order status: %v", err)
	}

	return &orderv1.UpdateOrderStatusResponse{OrderId: req.OrderId, Status: req.Status}, nil
}

func mapOrderToProto(order *domain.Order) *orderv1.Order {
	items := make([]*orderv1.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, &orderv1.OrderItem{
			ProductId:  item.ProductID.String(),
			Quantity:   int32(item.Quantity),
			PriceCents: item.Price,
		})
	}
	return &orderv1.Order{
		OrderId:          order.ID.String(),
		UserId:           order.UserID.String(),
		Items:            items,
		TotalAmountCents: order.TotalAmount,
		Status:           domainStatusToProto(order.Status),
		CreatedAt:        order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        order.UpdatedAt.Format(time.RFC3339),
	}
}

func domainStatusToProto(s domain.OrderStatus) orderv1.OrderStatus {
	switch s {
	case domain.OrderStatusPending:
		return orderv1.OrderStatus_ORDER_STATUS_PENDING
	case domain.OrderStatusAwaitingPayment:
		return orderv1.OrderStatus_ORDER_STATUS_AWAITING_PAYMENT
	case domain.OrderStatusPaid:
		return orderv1.OrderStatus_ORDER_STATUS_PAID
	case domain.OrderStatusProcessing:
		return orderv1.OrderStatus_ORDER_STATUS_PROCESSING
	case domain.OrderStatusShipped:
		return orderv1.OrderStatus_ORDER_STATUS_SHIPPED
	case domain.OrderStatusDelivered:
		return orderv1.OrderStatus_ORDER_STATUS_DELIVERED
	case domain.OrderStatusCancelled:
		return orderv1.OrderStatus_ORDER_STATUS_CANCELLED
	case domain.OrderStatusRefunded:
		return orderv1.OrderStatus_ORDER_STATUS_REFUNDED
	default:
		return orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func protoStatusToDomain(s orderv1.OrderStatus) domain.OrderStatus {
	switch s {
	case orderv1.OrderStatus_ORDER_STATUS_PENDING:
		return domain.OrderStatusPending
	case orderv1.OrderStatus_ORDER_STATUS_AWAITING_PAYMENT:
		return domain.OrderStatusAwaitingPayment
	case orderv1.OrderStatus_ORDER_STATUS_PAID:
		return domain.OrderStatusPaid
	case orderv1.OrderStatus_ORDER_STATUS_PROCESSING:
		return domain.OrderStatusProcessing
	case orderv1.OrderStatus_ORDER_STATUS_SHIPPED:
		return domain.OrderStatusShipped
	case orderv1.OrderStatus_ORDER_STATUS_DELIVERED:
		return domain.OrderStatusDelivered
	case orderv1.OrderStatus_ORDER_STATUS_CANCELLED:
		return domain.OrderStatusCancelled
	case orderv1.OrderStatus_ORDER_STATUS_REFUNDED:
		return domain.OrderStatusRefunded
	default:
		return domain.OrderStatusUnspecified
	}
}

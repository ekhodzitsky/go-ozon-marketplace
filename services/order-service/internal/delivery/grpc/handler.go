package grpc

import (
	"context"
	"errors"
	"time"

	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
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

func NewOrderHandler(uc usecase.OrderUsecase) orderv1.OrderServiceServer {
	return &OrderHandler{usecase: uc}
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	authUserID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
	}

	userID, err := uuid.Parse(authUserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user_id in token: %v", err)
	}

	items := make([]domain.OrderItem, 0, len(req.Items))
	for _, item := range req.Items {
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
	if order.UserID.String() != authUserID && role != auth.RoleAdmin {
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

	orders, total, err := h.usecase.ListOrders(ctx, userID, int(req.Page), int(req.PageSize))
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
	if order.UserID.String() != authUserID && role != auth.RoleAdmin {
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
	if role != auth.RoleAdmin && role != auth.RoleService {
		return nil, status.Error(codes.PermissionDenied, "only admin or service can update order status")
	}

	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
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

var domainStatusToProtoMap = map[domain.OrderStatus]orderv1.OrderStatus{
	domain.OrderStatusPending:         orderv1.OrderStatus_ORDER_STATUS_PENDING,
	domain.OrderStatusAwaitingPayment: orderv1.OrderStatus_ORDER_STATUS_AWAITING_PAYMENT,
	domain.OrderStatusPaid:            orderv1.OrderStatus_ORDER_STATUS_PAID,
	domain.OrderStatusProcessing:      orderv1.OrderStatus_ORDER_STATUS_PROCESSING,
	domain.OrderStatusShipped:         orderv1.OrderStatus_ORDER_STATUS_SHIPPED,
	domain.OrderStatusDelivered:       orderv1.OrderStatus_ORDER_STATUS_DELIVERED,
	domain.OrderStatusCancelled:       orderv1.OrderStatus_ORDER_STATUS_CANCELLED,
	domain.OrderStatusRefunded:        orderv1.OrderStatus_ORDER_STATUS_REFUNDED,
}

func domainStatusToProto(s domain.OrderStatus) orderv1.OrderStatus {
	if ps, ok := domainStatusToProtoMap[s]; ok {
		return ps
	}
	return orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
}

var protoStatusToDomainMap = map[orderv1.OrderStatus]domain.OrderStatus{
	orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED:      domain.OrderStatusUnspecified,
	orderv1.OrderStatus_ORDER_STATUS_PENDING:          domain.OrderStatusPending,
	orderv1.OrderStatus_ORDER_STATUS_AWAITING_PAYMENT: domain.OrderStatusAwaitingPayment,
	orderv1.OrderStatus_ORDER_STATUS_PAID:             domain.OrderStatusPaid,
	orderv1.OrderStatus_ORDER_STATUS_PROCESSING:       domain.OrderStatusProcessing,
	orderv1.OrderStatus_ORDER_STATUS_SHIPPED:          domain.OrderStatusShipped,
	orderv1.OrderStatus_ORDER_STATUS_DELIVERED:        domain.OrderStatusDelivered,
	orderv1.OrderStatus_ORDER_STATUS_CANCELLED:        domain.OrderStatusCancelled,
	orderv1.OrderStatus_ORDER_STATUS_REFUNDED:         domain.OrderStatusRefunded,
}

func protoStatusToDomain(s orderv1.OrderStatus) domain.OrderStatus {
	if ds, ok := protoStatusToDomainMap[s]; ok {
		return ds
	}
	return domain.OrderStatusUnspecified
}

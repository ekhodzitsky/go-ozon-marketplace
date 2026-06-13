package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/infrastructure/grpcclient"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	_ OrderUsecase = (*orderUsecase)(nil)
)

type OrderOrchestrator interface {
	ProcessOrder(ctx context.Context, order *domain.Order, idempotencyKey string) error
}

type orderUsecase struct {
	uowFactory    func() unitofwork.UnitOfWork
	orderRepo     repository.OrderRepository
	outboxRepo    repository.OutboxRepository
	sagaRepo      repository.SagaRepository
	orchestrator  OrderOrchestrator
	invClient     saga.InventoryClient
	payClient     saga.PaymentClient
	catalogClient grpcclient.CatalogClient
	redis         *redis.Client
	callTimeout   time.Duration
	queryTimeout  time.Duration
}

func NewOrderUsecase(
	uowFactory func() unitofwork.UnitOfWork,
	orderRepo repository.OrderRepository,
	outboxRepo repository.OutboxRepository,
	sagaRepo repository.SagaRepository,
	orchestrator OrderOrchestrator,
	invClient saga.InventoryClient,
	payClient saga.PaymentClient,
	catalogClient grpcclient.CatalogClient,
	redis *redis.Client,
	callTimeout time.Duration,
	queryTimeout time.Duration,
) OrderUsecase {
	if callTimeout == 0 {
		callTimeout = 5 * time.Second
	}
	if queryTimeout == 0 {
		queryTimeout = 3 * time.Second
	}
	return &orderUsecase{
		uowFactory:    uowFactory,
		orderRepo:     orderRepo,
		outboxRepo:    outboxRepo,
		sagaRepo:      sagaRepo,
		orchestrator:  orchestrator,
		invClient:     invClient,
		payClient:     payClient,
		catalogClient: catalogClient,
		redis:         redis,
		callTimeout:   callTimeout,
		queryTimeout:  queryTimeout,
	}
}

func (u *orderUsecase) publishOrderEvent(ctx context.Context, orderID, status, userID string) {
	if u.redis == nil {
		return
	}
	event := map[string]interface{}{
		"topic":   "orders",
		"user_id": userID,
		"payload": map[string]interface{}{
			"order_id": orderID,
			"status":   status,
			"user_id":  userID,
		},
	}
	data, _ := json.Marshal(event)
	pubCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	u.redis.Publish(pubCtx, "order-events", string(data))
}

func (u *orderUsecase) CreateOrder(ctx context.Context, userID uuid.UUID, items []domain.OrderItem, idempotencyKey string) (uuid.UUID, error) {
	if idempotencyKey == "" {
		return uuid.Nil, fmt.Errorf("%w: idempotency_key is required", apperrors.ErrInvalidArgument)
	}
	if len(items) == 0 {
		return uuid.Nil, fmt.Errorf("%w: order must contain at least one item", apperrors.ErrInvalidArgument)
	}

	var total int64
	for _, item := range items {
		if item.Quantity <= 0 {
			return uuid.Nil, fmt.Errorf("%w: quantity must be positive", apperrors.ErrInvalidArgument)
		}
		if item.Price <= 0 {
			return uuid.Nil, fmt.Errorf("%w: price must be greater than zero", apperrors.ErrInvalidArgument)
		}
		total += item.Price * int64(item.Quantity)
	}

	if total <= 0 {
		return uuid.Nil, fmt.Errorf("%w: total amount must be greater than zero", apperrors.ErrInvalidArgument)
	}

	for _, item := range items {
		product, err := u.catalogClient.GetProduct(ctx, item.ProductID.String())
		if err != nil {
			return uuid.Nil, fmt.Errorf("get product %s: %w", item.ProductID, err)
		}
		if product == nil || product.PriceCents != item.Price {
			return uuid.Nil, fmt.Errorf("%w: price mismatch for product %s", apperrors.ErrInvalidArgument, item.ProductID)
		}
	}

	order := &domain.Order{
		ID:          uuid.New(),
		UserID:      userID,
		Items:       items,
		TotalAmount: total,
		Status:      domain.OrderStatusPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	for i := range order.Items {
		order.Items[i].ID = uuid.New()
		order.Items[i].OrderID = order.ID
	}

	uow := u.uowFactory()
	txCtx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	if err := uow.Begin(txCtx); err != nil {
		return uuid.Nil, fmt.Errorf("begin uow: %w", err)
	}
	defer func() { _ = uow.Rollback(txCtx) }()

	if err := uow.OrderRepo().Create(txCtx, order); err != nil {
		return uuid.Nil, fmt.Errorf("create order: %w", err)
	}

	payload, err := json.Marshal(order)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal outbox payload: %w", err)
	}

	event := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "order",
		AggregateID:   order.ID.String(),
		EventType:     "OrderCreated",
		Payload:       payload,
		CreatedAt:     time.Now().UTC(),
	}

	if err := uow.OutboxRepo().Create(txCtx, event); err != nil {
		return uuid.Nil, fmt.Errorf("create outbox event: %w", err)
	}

	if err := uow.Commit(txCtx); err != nil {
		return uuid.Nil, fmt.Errorf("commit uow: %w", err)
	}

	u.publishOrderEvent(ctx, order.ID.String(), string(order.Status), order.UserID.String())

	sagaCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	if err := u.orchestrator.ProcessOrder(sagaCtx, order, idempotencyKey); err != nil {
		return order.ID, fmt.Errorf("saga process order: %w", err)
	}

	return order.ID, nil
}

func (u *orderUsecase) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.orderRepo.GetByID(ctx, id)
}

func (u *orderUsecase) ListOrders(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]domain.Order, int, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.orderRepo.ListByUser(ctx, userID, page, pageSize)
}

func (u *orderUsecase) CancelOrder(ctx context.Context, id uuid.UUID) error {
	qCtx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()

	order, err := u.orderRepo.GetByID(qCtx, id)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}

	if order.Status == domain.OrderStatusCancelled {
		return fmt.Errorf("%w: order already cancelled", apperrors.ErrInvalidArgument)
	}

	if order.Status.CancellableDirectly() {
		if err := u.orderRepo.UpdateStatus(qCtx, id, domain.OrderStatusCancelled); err != nil {
			return fmt.Errorf("update order status: %w", err)
		}
		u.publishOrderEvent(ctx, order.ID.String(), string(domain.OrderStatusCancelled), order.UserID.String())

		cCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
		defer cancel()
		for _, item := range order.Items {
			if err := u.invClient.Release(cCtx, item.ProductID.String(), int32(item.Quantity), order.ID.String(), releaseIdempotencyKey(order.ID.String(), item.ProductID.String())); err != nil {
				return fmt.Errorf("release inventory for product %s: %w", item.ProductID, err)
			}
		}
		return nil
	}

	// Paid or later orders require compensation/refund.
	if order.Status == domain.OrderStatusPaid || order.Status == domain.OrderStatusProcessing || order.Status == domain.OrderStatusShipped {
		saga, err := u.sagaRepo.GetByOrderID(qCtx, order.ID)
		if err != nil {
			return fmt.Errorf("%w: saga not found for refund: %v", apperrors.ErrFailedPrecondition, err)
		}

		cCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
		defer cancel()
		if saga.PaymentID != "" {
			if err := u.payClient.Refund(cCtx, saga.PaymentID, refundIdempotencyKey(order.ID.String(), saga.PaymentID)); err != nil {
				return fmt.Errorf("refund payment %s: %w", saga.PaymentID, err)
			}
		}
		for _, item := range order.Items {
			if err := u.invClient.Release(cCtx, item.ProductID.String(), int32(item.Quantity), order.ID.String(), releaseIdempotencyKey(order.ID.String(), item.ProductID.String())); err != nil {
				return fmt.Errorf("release inventory for product %s: %w", item.ProductID, err)
			}
		}

		if err := u.orderRepo.UpdateStatus(qCtx, id, domain.OrderStatusCancelled); err != nil {
			return fmt.Errorf("update order status: %w", err)
		}
		u.publishOrderEvent(ctx, order.ID.String(), string(domain.OrderStatusCancelled), order.UserID.String())
		return nil
	}

	return fmt.Errorf("%w: order with status %s cannot be cancelled", apperrors.ErrFailedPrecondition, order.Status)
}

func (u *orderUsecase) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error {
	qCtx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()

	order, err := u.orderRepo.GetByID(qCtx, id)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}

	if err := order.Status.ValidateTransition(status); err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrInvalidArgument, err)
	}

	if err := u.orderRepo.UpdateStatus(qCtx, id, status); err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	u.publishOrderEvent(ctx, order.ID.String(), string(status), order.UserID.String())
	return nil
}

func releaseIdempotencyKey(orderID, productID string) string {
	return fmt.Sprintf("release:%s:%s", orderID, productID)
}

func refundIdempotencyKey(orderID, paymentID string) string {
	return fmt.Sprintf("refund:%s:%s", orderID, paymentID)
}

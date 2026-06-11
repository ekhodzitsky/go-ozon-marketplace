package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork"
	"github.com/google/uuid"
)

var (
	_ OrderUsecase = (*orderUsecase)(nil)
)

type orderUsecase struct {
	uowFactory   func() unitofwork.UnitOfWork
	orderRepo    repository.OrderRepository
	outboxRepo   repository.OutboxRepository
	orchestrator *saga.Orchestrator
	invClient    saga.InventoryClient
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewOrderUsecase(
	uowFactory func() unitofwork.UnitOfWork,
	orderRepo repository.OrderRepository,
	outboxRepo repository.OutboxRepository,
	orchestrator *saga.Orchestrator,
	invClient saga.InventoryClient,
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
		uowFactory:   uowFactory,
		orderRepo:    orderRepo,
		outboxRepo:   outboxRepo,
		orchestrator: orchestrator,
		invClient:    invClient,
		callTimeout:  callTimeout,
		queryTimeout: queryTimeout,
	}
}

func (u *orderUsecase) CreateOrder(ctx context.Context, userID uuid.UUID, items []domain.OrderItem) (uuid.UUID, error) {
	if len(items) == 0 {
		return uuid.Nil, fmt.Errorf("%w: order must contain at least one item", apperrors.ErrInvalidArgument)
	}

	var total int64
	for _, item := range items {
		if item.Quantity <= 0 {
			return uuid.Nil, fmt.Errorf("%w: quantity must be positive", apperrors.ErrInvalidArgument)
		}
		if item.Price < 0 {
			return uuid.Nil, fmt.Errorf("%w: price cannot be negative", apperrors.ErrInvalidArgument)
		}
		total += item.Price * int64(item.Quantity)
	}

	if total <= 0 {
		return uuid.Nil, fmt.Errorf("%w: total amount must be greater than zero", apperrors.ErrInvalidArgument)
	}

	order := &domain.Order{
		ID:          uuid.New(),
		UserID:      userID,
		Items:       items,
		TotalAmount: total,
		Status:      "pending",
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
	defer uow.Rollback(txCtx)

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

	sagaCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	if err := u.orchestrator.ProcessOrder(sagaCtx, order); err != nil {
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

	if order.Status == "cancelled" {
		return fmt.Errorf("%w: order already cancelled", apperrors.ErrInvalidArgument)
	}

	if err := u.orderRepo.UpdateStatus(qCtx, id, "cancelled"); err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	cCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	for _, item := range order.Items {
		if err := u.invClient.Release(cCtx, item.ProductID.String(), int32(item.Quantity), order.ID.String()); err != nil {
			return fmt.Errorf("release inventory for product %s: %w", item.ProductID, err)
		}
	}

	return nil
}

func (u *orderUsecase) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) error {
	qCtx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.orderRepo.UpdateStatus(qCtx, id, status)
}

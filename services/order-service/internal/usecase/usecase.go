package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork"
	"github.com/google/uuid"
)

type OrderUsecase struct {
	uowFactory   func() unitofwork.UnitOfWork
	orderRepo    repository.OrderRepository
	outboxRepo   repository.OutboxRepository
	orchestrator *saga.Orchestrator
}

func NewOrderUsecase(
	uowFactory func() unitofwork.UnitOfWork,
	orderRepo repository.OrderRepository,
	outboxRepo repository.OutboxRepository,
	orchestrator *saga.Orchestrator,
) *OrderUsecase {
	return &OrderUsecase{
		uowFactory:   uowFactory,
		orderRepo:    orderRepo,
		outboxRepo:   outboxRepo,
		orchestrator: orchestrator,
	}
}

func (u *OrderUsecase) CreateOrder(ctx context.Context, userID uuid.UUID, items []domain.OrderItem) (uuid.UUID, error) {
	var total float64
	for _, item := range items {
		total += item.Price * float64(item.Quantity)
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
	if err := uow.Begin(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("begin uow: %w", err)
	}
	defer uow.Rollback(ctx)

	if err := uow.OrderRepo().Create(ctx, order); err != nil {
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

	if err := uow.OutboxRepo().Create(ctx, event); err != nil {
		return uuid.Nil, fmt.Errorf("create outbox event: %w", err)
	}

	if err := uow.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit uow: %w", err)
	}

	if err := u.orchestrator.ProcessOrder(ctx, order); err != nil {
		return order.ID, fmt.Errorf("saga process order: %w", err)
	}

	return order.ID, nil
}

func (u *OrderUsecase) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	return u.orderRepo.GetByID(ctx, id)
}

func (u *OrderUsecase) ListOrders(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]domain.Order, int, error) {
	return u.orderRepo.ListByUser(ctx, userID, page, pageSize)
}

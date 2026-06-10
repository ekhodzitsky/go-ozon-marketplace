package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderUsecase struct {
	pool         *pgxpool.Pool
	orderRepo    *postgres.OrderPostgres
	outboxRepo   *postgres.OutboxPostgres
	orchestrator *saga.Orchestrator
}

func NewOrderUsecase(
	pool *pgxpool.Pool,
	orderRepo *postgres.OrderPostgres,
	outboxRepo *postgres.OutboxPostgres,
	orchestrator *saga.Orchestrator,
) *OrderUsecase {
	return &OrderUsecase{
		pool:         pool,
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

	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := u.orderRepo.WithTx(tx).Create(ctx, order); err != nil {
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

	if err := u.outboxRepo.WithTx(tx).Create(ctx, event); err != nil {
		return uuid.Nil, fmt.Errorf("create outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit tx: %w", err)
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

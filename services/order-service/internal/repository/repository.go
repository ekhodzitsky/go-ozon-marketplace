package repository

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/google/uuid"
)

type OrderRepository interface {
	Create(ctx context.Context, order *domain.Order) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error
	ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]domain.Order, int, error)
}

type OutboxRepository interface {
	Create(ctx context.Context, event *domain.OutboxEvent) error
	GetUnprocessed(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkProcessed(ctx context.Context, id uuid.UUID) error
	BatchMarkProcessed(ctx context.Context, ids []uuid.UUID) error
	IncrementRetryAndSetError(ctx context.Context, id uuid.UUID, lastError string, nextRetryAt time.Time) error
	MoveToDLQ(ctx context.Context, event *domain.OutboxEvent, failedAt time.Time, lastError string) error
	Begin(ctx context.Context) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type SagaRepository interface {
	Create(ctx context.Context, saga *domain.Saga) error
	GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.Saga, error)
	UpdateStatus(ctx context.Context, orderID uuid.UUID, status domain.SagaStatus, step string, errMsg string) error
	Save(ctx context.Context, saga *domain.Saga) error
	ListIncomplete(ctx context.Context, limit int) ([]domain.Saga, error)
}

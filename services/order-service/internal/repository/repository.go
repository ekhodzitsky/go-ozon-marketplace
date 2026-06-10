package repository

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/google/uuid"
)

type OrderRepository interface {
	Create(ctx context.Context, order *domain.Order) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]domain.Order, int, error)
}

type OutboxRepository interface {
	Create(ctx context.Context, event *domain.OutboxEvent) error
	GetUnprocessed(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkProcessed(ctx context.Context, id uuid.UUID) error
	BatchMarkProcessed(ctx context.Context, ids []uuid.UUID) error
}

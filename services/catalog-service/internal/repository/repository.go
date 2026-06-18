package repository

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/google/uuid"
)

type ProductRepository interface {
	Create(ctx context.Context, product *domain.Product) (uuid.UUID, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	Update(ctx context.Context, product *domain.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error)
}

type ProductSearchRepository interface {
	Index(ctx context.Context, product *domain.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
	Search(ctx context.Context, query string, page, pageSize int) ([]*domain.Product, int, error)
	EnsureIndex(ctx context.Context) error
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

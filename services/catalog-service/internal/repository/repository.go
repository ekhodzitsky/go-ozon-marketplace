package repository

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ProductRepository работает с таблицей товаров.
// WithTx возвращает репозиторий, привязанный к конкретной транзакции.
type ProductRepository interface {
	WithTx(tx pgx.Tx) ProductRepository
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

// OutboxRepository работает с таблицей outbox.
// WithTx возвращает репозиторий, привязанный к конкретной транзакции;
// сам репозиторий не хранит состояние транзакции внутри себя.
type OutboxRepository interface {
	WithTx(tx pgx.Tx) OutboxRepository
	Create(ctx context.Context, event *domain.OutboxEvent) error
	GetUnprocessed(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkProcessed(ctx context.Context, id uuid.UUID) error
	BatchMarkProcessed(ctx context.Context, ids []uuid.UUID) error
	IncrementRetryAndSetError(ctx context.Context, id uuid.UUID, lastError string, nextRetryAt time.Time) error
	MoveToDLQ(ctx context.Context, event *domain.OutboxEvent, failedAt time.Time, lastError string) error
}

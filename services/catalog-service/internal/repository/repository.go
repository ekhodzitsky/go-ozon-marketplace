package repository

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/google/uuid"
)

type ProductRepository interface {
	Create(ctx context.Context, product *domain.Product) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	List(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error)
}

type ProductSearchRepository interface {
	Index(ctx context.Context, product *domain.Product) error
	Search(ctx context.Context, query string, page, pageSize int) ([]*domain.Product, int, error)
}

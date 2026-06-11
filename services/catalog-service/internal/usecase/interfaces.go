package usecase

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/google/uuid"
)

// CatalogUsecase defines the catalog use-case boundary.
type CatalogUsecase interface {
	CreateProduct(ctx context.Context, name, description string, price int64, categories []string) (uuid.UUID, error)
	GetProduct(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	UpdateProduct(ctx context.Context, id uuid.UUID, name, description string, price int64, categories []string) error
	DeleteProduct(ctx context.Context, id uuid.UUID) error
	ListProducts(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error)
	SearchProducts(ctx context.Context, query string, page, pageSize int) ([]*domain.Product, int, error)
}

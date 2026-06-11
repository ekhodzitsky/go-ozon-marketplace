package repository

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/domain"
	"github.com/google/uuid"
)

type InventoryRepository interface {
	GetStock(ctx context.Context, productID uuid.UUID) (*domain.Stock, error)
	Reserve(ctx context.Context, productID uuid.UUID, quantity int, orderID uuid.UUID) error
	Release(ctx context.Context, productID uuid.UUID, quantity int, orderID uuid.UUID) error
}

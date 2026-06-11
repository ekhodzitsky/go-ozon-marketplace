package usecase

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/domain"
	"github.com/google/uuid"
)

// InventoryUsecase defines the inventory use-case boundary.
type InventoryUsecase interface {
	GetStock(ctx context.Context, productID uuid.UUID) (*domain.Stock, error)
	Reserve(ctx context.Context, productID uuid.UUID, quantity int, orderID string) error
	Release(ctx context.Context, productID uuid.UUID, quantity int, orderID string) error
	GetLedger(ctx context.Context, productID uuid.UUID) ([]*domain.LedgerEntry, error)
}

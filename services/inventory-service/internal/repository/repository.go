package repository

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InventoryRepository defines the data access boundary for inventory.
// Transactional methods accept a repository instance bound to a transaction.
type InventoryRepository interface {
	GetStock(ctx context.Context, productID uuid.UUID) (*domain.Stock, error)
	GetLedger(ctx context.Context, productID uuid.UUID) ([]*domain.LedgerEntry, error)

	// Low-level transactional operations.
	InsertReservation(ctx context.Context, orderID, productID uuid.UUID, quantity int) (int64, error)
	SelectReservation(ctx context.Context, orderID, productID uuid.UUID) (*ReservationRow, error)
	SelectReservationForUpdate(ctx context.Context, orderID, productID uuid.UUID) (*ReservationRow, error)
	UpdateReservationStatus(ctx context.Context, orderID, productID uuid.UUID, status string) error
	UpdateStockForReserve(ctx context.Context, productID uuid.UUID, quantity int) (int64, error)
	UpdateStockForRelease(ctx context.Context, productID uuid.UUID, quantity int) (int64, error)
	InsertLedger(ctx context.Context, productID, orderID uuid.UUID, quantityChange int, operationType string) error

	WithTx(tx pgx.Tx) InventoryRepository
}

// ReservationRow is the persistent state of a reservation.
type ReservationRow struct {
	Quantity int
	Status   string
}

// TxManager runs a function inside a transaction with a repository bound to it.
type TxManager interface {
	Run(ctx context.Context, fn func(repo InventoryRepository) error) error
}

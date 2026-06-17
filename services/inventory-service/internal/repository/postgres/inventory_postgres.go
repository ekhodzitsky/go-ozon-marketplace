package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
)

const (
	ledgerOpReserve = "LEDGER_OPERATION_RESERVE"
	ledgerOpRelease = "LEDGER_OPERATION_RELEASE"
)

// Querier is the subset of pgx database operations used by the repository.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type InventoryPostgres struct {
	db Querier
}

func NewInventoryPostgres(db Querier) repository.InventoryRepository {
	return &InventoryPostgres{db: db}
}

func (r *InventoryPostgres) WithTx(tx pgx.Tx) repository.InventoryRepository {
	return &InventoryPostgres{db: tx}
}

func (r *InventoryPostgres) GetStock(ctx context.Context, productID uuid.UUID) (*domain.Stock, error) {
	query := `SELECT product_id, available, reserved FROM inventory WHERE product_id = $1`
	row := r.db.QueryRow(ctx, query, productID)
	var stock domain.Stock
	if err := row.Scan(&stock.ProductID, &stock.Available, &stock.Reserved); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: stock", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get stock: %w", err)
	}
	return &stock, nil
}

func (r *InventoryPostgres) InsertReservation(ctx context.Context, orderID, productID uuid.UUID, quantity int) (int64, error) {
	insertSQL := `
		INSERT INTO reservations (order_id, product_id, quantity, status)
		VALUES ($1, $2, $3, 'reserved')
		ON CONFLICT (order_id, product_id) DO NOTHING
	`
	tag, err := r.db.Exec(ctx, insertSQL, orderID, productID, quantity)
	if err != nil {
		return 0, fmt.Errorf("insert reservation: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *InventoryPostgres) SelectReservation(ctx context.Context, orderID, productID uuid.UUID) (*repository.ReservationRow, error) {
	var row repository.ReservationRow
	if err := r.db.QueryRow(ctx,
		`SELECT quantity, status FROM reservations WHERE order_id = $1 AND product_id = $2`,
		orderID, productID,
	).Scan(&row.Quantity, &row.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: reservation not found", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("select reservation: %w", err)
	}
	return &row, nil
}

func (r *InventoryPostgres) SelectReservationForUpdate(ctx context.Context, orderID, productID uuid.UUID) (*repository.ReservationRow, error) {
	var row repository.ReservationRow
	if err := r.db.QueryRow(ctx,
		`SELECT quantity, status FROM reservations WHERE order_id = $1 AND product_id = $2 FOR UPDATE`,
		orderID, productID,
	).Scan(&row.Quantity, &row.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: reservation not found", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("select reservation: %w", err)
	}
	return &row, nil
}

func (r *InventoryPostgres) UpdateReservationStatus(ctx context.Context, orderID, productID uuid.UUID, status string) error {
	if _, err := r.db.Exec(ctx,
		`UPDATE reservations SET status = $1 WHERE order_id = $2 AND product_id = $3`,
		status, orderID, productID,
	); err != nil {
		return fmt.Errorf("update reservation status: %w", err)
	}
	return nil
}

func (r *InventoryPostgres) UpdateStockForReserve(ctx context.Context, productID uuid.UUID, quantity int) (int64, error) {
	updateSQL := `
		UPDATE inventory
		SET available = available - $1, reserved = reserved + $1
		WHERE product_id = $2 AND available >= $1
	`
	tag, err := r.db.Exec(ctx, updateSQL, quantity, productID)
	if err != nil {
		return 0, fmt.Errorf("update stock: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *InventoryPostgres) UpdateStockForRelease(ctx context.Context, productID uuid.UUID, quantity int) (int64, error) {
	updateSQL := `
		UPDATE inventory
		SET available = available + $1, reserved = reserved - $1
		WHERE product_id = $2 AND reserved >= $1
	`
	tag, err := r.db.Exec(ctx, updateSQL, quantity, productID)
	if err != nil {
		return 0, fmt.Errorf("update stock: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *InventoryPostgres) InsertLedger(ctx context.Context, productID, orderID uuid.UUID, quantityChange int, operationType string) error {
	if _, err := r.db.Exec(ctx,
		`INSERT INTO inventory_ledger (product_id, order_id, quantity_change, operation_type) VALUES ($1, $2, $3, $4)`,
		productID, orderID, quantityChange, operationType,
	); err != nil {
		return fmt.Errorf("insert ledger: %w", err)
	}
	return nil
}

func (r *InventoryPostgres) GetLedger(ctx context.Context, productID uuid.UUID) ([]*domain.LedgerEntry, error) {
	query := `SELECT id, product_id, order_id, quantity_change, operation_type, created_at FROM inventory_ledger WHERE product_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("get ledger: %w", err)
	}
	defer rows.Close()

	var entries []*domain.LedgerEntry
	for rows.Next() {
		var entry domain.LedgerEntry
		var orderID *uuid.UUID
		if err := rows.Scan(&entry.ID, &entry.ProductID, &orderID, &entry.QuantityChange, &entry.OperationType, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		entry.OrderID = orderID
		entries = append(entries, &entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger rows err: %w", err)
	}
	return entries, nil
}

// Ensure InventoryPostgres implements InventoryRepository.
var _ repository.InventoryRepository = (*InventoryPostgres)(nil)

// Ensure pgxpool.Pool implements Querier at compile time.
var _ Querier = (*pgxpool.Pool)(nil)

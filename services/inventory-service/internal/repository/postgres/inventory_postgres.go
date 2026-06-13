package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
)

const (
	DefaultCallTimeout  = 5 * time.Second
	DefaultQueryTimeout = 3 * time.Second
)

const (
	ledgerOpReserve = "LEDGER_OPERATION_RESERVE"
	ledgerOpRelease = "LEDGER_OPERATION_RELEASE"
)

type InventoryPostgres struct {
	db           *pgxpool.Pool
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewInventoryPostgres(db *pgxpool.Pool, callTimeout time.Duration, queryTimeout time.Duration) repository.InventoryRepository {
	if callTimeout == 0 {
		callTimeout = DefaultCallTimeout
	}
	if queryTimeout == 0 {
		queryTimeout = DefaultQueryTimeout
	}
	return &InventoryPostgres{db: db, callTimeout: callTimeout, queryTimeout: queryTimeout}
}

func (r *InventoryPostgres) GetStock(ctx context.Context, productID uuid.UUID) (*domain.Stock, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

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

func (r *InventoryPostgres) Reserve(ctx context.Context, productID uuid.UUID, quantity int, orderID uuid.UUID) (err error) {
	ctx, cancel := context.WithTimeout(ctx, r.callTimeout)
	defer cancel()

	if quantity <= 0 {
		return fmt.Errorf("%w: quantity must be positive", apperrors.ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && err == nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = fmt.Errorf("rollback tx: %w", rbErr)
		}
	}()

	// Idempotent reservation record. If the same (order, product) already exists
	// with a matching quantity and status, this retry must not deduct stock again.
	insertSQL := `
		INSERT INTO reservations (order_id, product_id, quantity, status)
		VALUES ($1, $2, $3, 'reserved')
		ON CONFLICT (order_id, product_id) DO NOTHING
	`
	tag, err := tx.Exec(ctx, insertSQL, orderID, productID, quantity)
	if err != nil {
		return fmt.Errorf("insert reservation: %w", err)
	}

	if tag.RowsAffected() == 0 {
		// Already reserved for this (order, product). Verify it matches the request.
		var existingQty int
		var status string
		if err = tx.QueryRow(ctx,
			`SELECT quantity, status FROM reservations WHERE order_id = $1 AND product_id = $2`,
			orderID, productID,
		).Scan(&existingQty, &status); err != nil {
			return fmt.Errorf("select reservation: %w", err)
		}
		if status != "reserved" {
			return fmt.Errorf("%w: reservation already %s", apperrors.ErrFailedPrecondition, status)
		}
		if existingQty != quantity {
			return fmt.Errorf("%w: reservation quantity mismatch (expected %d, found %d)", apperrors.ErrConflict, quantity, existingQty)
		}
		return nil
	}

	// Atomic stock deduction with oversell protection.
	updateSQL := `
		UPDATE inventory
		SET available = available - $1, reserved = reserved + $1
		WHERE product_id = $2 AND available >= $1
	`
	tag, err = tx.Exec(ctx, updateSQL, quantity, productID)
	if err != nil {
		return fmt.Errorf("update stock: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrInsufficientStock
	}

	// Record the operation in the inventory ledger inside the same transaction.
	if _, err = tx.Exec(ctx,
		`INSERT INTO inventory_ledger (product_id, order_id, quantity_change, operation_type) VALUES ($1, $2, $3, $4)`,
		productID, orderID, -quantity, ledgerOpReserve,
	); err != nil {
		return fmt.Errorf("insert ledger: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *InventoryPostgres) Release(ctx context.Context, productID uuid.UUID, quantity int, orderID uuid.UUID) (err error) {
	ctx, cancel := context.WithTimeout(ctx, r.callTimeout)
	defer cancel()

	if quantity <= 0 {
		return fmt.Errorf("%w: quantity must be positive", apperrors.ErrInvalidArgument)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && err == nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = fmt.Errorf("rollback tx: %w", rbErr)
		}
	}()

	// Bind release to the existing reservation row.
	var reservedQty int
	var status string
	err = tx.QueryRow(ctx,
		`SELECT quantity, status FROM reservations WHERE order_id = $1 AND product_id = $2 FOR UPDATE`,
		orderID, productID,
	).Scan(&reservedQty, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: reservation not found", apperrors.ErrNotFound)
		}
		return fmt.Errorf("select reservation: %w", err)
	}

	if status == "released" && reservedQty == quantity {
		// Idempotent release: already released for this (order, product, quantity).
		return nil
	}
	if status != "reserved" {
		return fmt.Errorf("%w: reservation already %s", apperrors.ErrFailedPrecondition, status)
	}
	if reservedQty != quantity {
		return fmt.Errorf("%w: release quantity mismatch (reserved %d, requested %d)", apperrors.ErrConflict, reservedQty, quantity)
	}

	// Atomic stock release with over-release protection.
	updateSQL := `
		UPDATE inventory
		SET available = available + $1, reserved = reserved - $1
		WHERE product_id = $2 AND reserved >= $1
	`
	tag, err := tx.Exec(ctx, updateSQL, quantity, productID)
	if err != nil {
		return fmt.Errorf("update stock: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrInsufficientStock
	}

	if _, err = tx.Exec(ctx,
		`UPDATE reservations SET status = 'released' WHERE order_id = $1 AND product_id = $2`,
		orderID, productID,
	); err != nil {
		return fmt.Errorf("update reservation status: %w", err)
	}

	// Record the operation in the inventory ledger inside the same transaction.
	if _, err = tx.Exec(ctx,
		`INSERT INTO inventory_ledger (product_id, order_id, quantity_change, operation_type) VALUES ($1, $2, $3, $4)`,
		productID, orderID, quantity, ledgerOpRelease,
	); err != nil {
		return fmt.Errorf("insert ledger: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *InventoryPostgres) GetLedger(ctx context.Context, productID uuid.UUID) ([]*domain.LedgerEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

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

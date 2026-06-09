package postgres

import (
	"context"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InventoryPostgres struct {
	db *pgxpool.Pool
}

func NewInventoryPostgres(db *pgxpool.Pool) repository.InventoryRepository {
	return &InventoryPostgres{db: db}
}

func (r *InventoryPostgres) GetStock(ctx context.Context, productID uuid.UUID) (*domain.Stock, error) {
	query := `SELECT product_id, available, reserved FROM inventory WHERE product_id = $1`
	row := r.db.QueryRow(ctx, query, productID)
	var stock domain.Stock
	if err := row.Scan(&stock.ProductID, &stock.Available, &stock.Reserved); err != nil {
		return nil, fmt.Errorf("get stock: %w", err)
	}
	return &stock, nil
}

func (r *InventoryPostgres) Reserve(ctx context.Context, productID uuid.UUID, quantity int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var available, reserved int
	query := `SELECT available, reserved FROM inventory WHERE product_id = $1 FOR UPDATE`
	if err := tx.QueryRow(ctx, query, productID).Scan(&available, &reserved); err != nil {
		return fmt.Errorf("select stock: %w", err)
	}

	if available < quantity {
		return fmt.Errorf("insufficient available stock")
	}

	update := `UPDATE inventory SET available = available - $1, reserved = reserved + $1 WHERE product_id = $2`
	if _, err := tx.Exec(ctx, update, quantity, productID); err != nil {
		return fmt.Errorf("update stock: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *InventoryPostgres) Release(ctx context.Context, productID uuid.UUID, quantity int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var available, reserved int
	query := `SELECT available, reserved FROM inventory WHERE product_id = $1 FOR UPDATE`
	if err := tx.QueryRow(ctx, query, productID).Scan(&available, &reserved); err != nil {
		return fmt.Errorf("select stock: %w", err)
	}

	if reserved < quantity {
		return fmt.Errorf("insufficient reserved stock")
	}

	update := `UPDATE inventory SET available = available + $1, reserved = reserved - $1 WHERE product_id = $2`
	if _, err := tx.Exec(ctx, update, quantity, productID); err != nil {
		return fmt.Errorf("update stock: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

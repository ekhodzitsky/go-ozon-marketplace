package postgres

import (
	"context"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

type OrderPostgres struct {
	db Querier
}

func NewOrderPostgres(db *pgxpool.Pool) *OrderPostgres {
	return &OrderPostgres{db: db}
}

func (r *OrderPostgres) WithTx(tx pgx.Tx) *OrderPostgres {
	return &OrderPostgres{db: tx}
}

func (r *OrderPostgres) Create(ctx context.Context, order *domain.Order) error {
	query := `INSERT INTO orders (id, user_id, total_amount, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.Exec(ctx, query, order.ID, order.UserID, order.TotalAmount, order.Status, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	batch := &pgx.Batch{}
	for i := range order.Items {
		item := &order.Items[i]
		batch.Queue(`INSERT INTO order_items (id, order_id, product_id, quantity, price) VALUES ($1, $2, $3, $4, $5)`,
			item.ID, item.OrderID, item.ProductID, item.Quantity, item.Price)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for range order.Items {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("insert order item batch: %w", err)
		}
	}

	if err := br.Close(); err != nil {
		return fmt.Errorf("close batch: %w", err)
	}
	return nil
}

func (r *OrderPostgres) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	query := `SELECT id, user_id, total_amount, status, created_at, updated_at FROM orders WHERE id=$1`
	row := r.db.QueryRow(ctx, query, id)
	var order domain.Order
	if err := row.Scan(&order.ID, &order.UserID, &order.TotalAmount, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get order by id: %w", err)
	}
	items, err := r.getItemsByOrderID(ctx, id)
	if err != nil {
		return nil, err
	}
	order.Items = items
	return &order, nil
}

func (r *OrderPostgres) getItemsByOrderID(ctx context.Context, orderID uuid.UUID) ([]domain.OrderItem, error) {
	query := `SELECT id, order_id, product_id, quantity, price FROM order_items WHERE order_id=$1`
	rows, err := r.db.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("get order items: %w", err)
	}
	defer rows.Close()

	var items []domain.OrderItem
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.Quantity, &item.Price); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order items: %w", err)
	}
	return items, nil
}

func (r *OrderPostgres) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := `UPDATE orders SET status=$1, updated_at=NOW() WHERE id=$2`
	_, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	return nil
}

func (r *OrderPostgres) ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]domain.Order, int, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	countQuery := `SELECT COUNT(*) FROM orders WHERE user_id=$1`
	var total int
	if err := r.db.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	query := `SELECT id, user_id, total_amount, status, created_at, updated_at FROM orders WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, query, userID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var order domain.Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.TotalAmount, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate orders: %w", err)
	}

	for i := range orders {
		items, err := r.getItemsByOrderID(ctx, orders[i].ID)
		if err != nil {
			return nil, 0, err
		}
		orders[i].Items = items
	}

	return orders, total, nil
}

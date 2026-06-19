package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
)

type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type ProductPostgres struct {
	db Querier
}

func NewProductPostgres(db Querier) repository.ProductRepository {
	return &ProductPostgres{db: db}
}

func (r *ProductPostgres) WithTx(tx pgx.Tx) *ProductPostgres {
	return &ProductPostgres{db: tx}
}

func (r *ProductPostgres) Create(ctx context.Context, product *domain.Product) (uuid.UUID, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	// Вставляем или возвращаем id существующего товара с таким idempotency_key.
	// Всё в одном запросе, чтобы не ловить aborted-транзакцию после конфликта.
	query := `
		INSERT INTO products (id, name, description, price, categories, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idempotency_key) DO UPDATE SET id = products.id
		RETURNING id
	`
	var id uuid.UUID
	err := r.db.QueryRow(ctx, query, product.ID, product.Name, product.Description, product.Price, product.Categories, product.IdempotencyKey, product.CreatedAt).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert product: %w", err)
	}
	if id != product.ID {
		return id, apperrors.ErrAlreadyExists
	}
	return id, nil
}

func (r *ProductPostgres) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	query := `SELECT id, name, description, price, categories, idempotency_key, created_at FROM products WHERE id=$1`
	row := r.db.QueryRow(ctx, query, id)
	var product domain.Product
	if err := row.Scan(&product.ID, &product.Name, &product.Description, &product.Price, &product.Categories, &product.IdempotencyKey, &product.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: product", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get product by id: %w", err)
	}
	return &product, nil
}

func (r *ProductPostgres) Update(ctx context.Context, product *domain.Product) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	query := `UPDATE products SET name=$1, description=$2, price=$3, categories=$4 WHERE id=$5`
	tag, err := r.db.Exec(ctx, query, product.Name, product.Description, product.Price, product.Categories, product.ID)
	if err != nil {
		return fmt.Errorf("update product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: product", apperrors.ErrNotFound)
	}
	return nil
}

func (r *ProductPostgres) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	query := `DELETE FROM products WHERE id=$1`
	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: product", apperrors.ErrNotFound)
	}
	return nil
}

func (r *ProductPostgres) List(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	var total int
	countQuery := `SELECT COUNT(*) FROM products`
	if err := r.db.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	query := `SELECT id, name, description, price, categories, idempotency_key, created_at FROM products ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(ctx, query, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := make([]*domain.Product, 0)
	for rows.Next() {
		var product domain.Product
		if err := rows.Scan(&product.ID, &product.Name, &product.Description, &product.Price, &product.Categories, &product.IdempotencyKey, &product.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, &product)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate products: %w", err)
	}

	return products, total, nil
}

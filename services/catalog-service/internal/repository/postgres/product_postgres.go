package postgres

import (
	"context"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductPostgres struct {
	db *pgxpool.Pool
}

func NewProductPostgres(db *pgxpool.Pool) repository.ProductRepository {
	return &ProductPostgres{db: db}
}

func (r *ProductPostgres) Create(ctx context.Context, product *domain.Product) error {
	query := `INSERT INTO products (id, name, description, price, stock, categories, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.db.Exec(ctx, query, product.ID, product.Name, product.Description, product.Price, product.Stock, product.Categories, product.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert product: %w", err)
	}
	return nil
}

func (r *ProductPostgres) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	query := `SELECT id, name, description, price, stock, categories, created_at FROM products WHERE id=$1`
	row := r.db.QueryRow(ctx, query, id)
	var product domain.Product
	if err := row.Scan(&product.ID, &product.Name, &product.Description, &product.Price, &product.Stock, &product.Categories, &product.CreatedAt); err != nil {
		return nil, fmt.Errorf("get product by id: %w", err)
	}
	return &product, nil
}

func (r *ProductPostgres) List(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error) {
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

	query := `SELECT id, name, description, price, stock, categories, created_at FROM products ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(ctx, query, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := make([]*domain.Product, 0)
	for rows.Next() {
		var product domain.Product
		if err := rows.Scan(&product.ID, &product.Name, &product.Description, &product.Price, &product.Stock, &product.Categories, &product.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, &product)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate products: %w", err)
	}

	return products, total, nil
}

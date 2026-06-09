package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/google/uuid"
)

type CatalogUsecase struct {
	productRepo repository.ProductRepository
	searchRepo  repository.ProductSearchRepository
}

func NewCatalogUsecase(productRepo repository.ProductRepository, searchRepo repository.ProductSearchRepository) *CatalogUsecase {
	return &CatalogUsecase{
		productRepo: productRepo,
		searchRepo:  searchRepo,
	}
}

func (u *CatalogUsecase) CreateProduct(ctx context.Context, name, description string, price float64, stock int, categories []string) (uuid.UUID, error) {
	product := &domain.Product{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		Price:       price,
		Stock:       stock,
		Categories:  categories,
		CreatedAt:   time.Now().UTC(),
	}

	if err := u.productRepo.Create(ctx, product); err != nil {
		return uuid.Nil, fmt.Errorf("create product: %w", err)
	}

	if err := u.searchRepo.Index(ctx, product); err != nil {
		return uuid.Nil, fmt.Errorf("index product: %w", err)
	}

	return product.ID, nil
}

func (u *CatalogUsecase) GetProduct(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	return u.productRepo.GetByID(ctx, id)
}

func (u *CatalogUsecase) ListProducts(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error) {
	return u.productRepo.List(ctx, page, pageSize)
}

func (u *CatalogUsecase) SearchProducts(ctx context.Context, query string, page, pageSize int) ([]*domain.Product, int, error) {
	return u.searchRepo.Search(ctx, query, page, pageSize)
}

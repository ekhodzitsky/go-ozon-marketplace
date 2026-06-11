package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/unitofwork"
	"github.com/google/uuid"
)

const (
	DefaultCallTimeout  = 5 * time.Second
	DefaultQueryTimeout = 3 * time.Second
)

type catalogUsecase struct {
	uowFactory   func() unitofwork.UnitOfWork
	productRepo  repository.ProductRepository
	searchRepo   repository.ProductSearchRepository
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewCatalogUsecase(
	uowFactory func() unitofwork.UnitOfWork,
	productRepo repository.ProductRepository,
	searchRepo repository.ProductSearchRepository,
	callTimeout time.Duration,
	queryTimeout time.Duration,
) CatalogUsecase {
	if callTimeout == 0 {
		callTimeout = DefaultCallTimeout
	}
	if queryTimeout == 0 {
		queryTimeout = DefaultQueryTimeout
	}
	return &catalogUsecase{
		uowFactory:   uowFactory,
		productRepo:  productRepo,
		searchRepo:   searchRepo,
		callTimeout:  callTimeout,
		queryTimeout: queryTimeout,
	}
}

func (u *catalogUsecase) CreateProduct(ctx context.Context, name, description string, price int64, categories []string) (uuid.UUID, error) {
	product := &domain.Product{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		Price:       price,
		Categories:  categories,
		CreatedAt:   time.Now().UTC(),
	}

	payload, err := json.Marshal(product)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal product payload: %w", err)
	}

	event := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "product",
		AggregateID:   product.ID.String(),
		EventType:     "ProductCreated",
		Payload:       payload,
		CreatedAt:     time.Now().UTC(),
	}

	txCtx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()

	uow := u.uowFactory()
	if err := uow.Begin(txCtx); err != nil {
		return uuid.Nil, fmt.Errorf("begin uow: %w", err)
	}
	defer uow.Rollback(txCtx)

	if err := uow.ProductRepo().Create(txCtx, product); err != nil {
		return uuid.Nil, fmt.Errorf("create product: %w", err)
	}

	if err := uow.OutboxRepo().Create(txCtx, event); err != nil {
		return uuid.Nil, fmt.Errorf("create outbox event: %w", err)
	}

	if err := uow.Commit(txCtx); err != nil {
		return uuid.Nil, fmt.Errorf("commit uow: %w", err)
	}

	return product.ID, nil
}

func (u *catalogUsecase) GetProduct(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.productRepo.GetByID(ctx, id)
}

func (u *catalogUsecase) ListProducts(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.productRepo.List(ctx, page, pageSize)
}

func (u *catalogUsecase) SearchProducts(ctx context.Context, query string, page, pageSize int) ([]*domain.Product, int, error) {
	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	return u.searchRepo.Search(ctx, query, page, pageSize)
}

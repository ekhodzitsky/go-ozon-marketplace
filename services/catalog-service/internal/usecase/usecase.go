package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	DefaultCallTimeout  = 5 * time.Second
	DefaultQueryTimeout = 3 * time.Second
)

// txRunner выполняет функцию внутри БД-транзакции.
// В продакшене это txmanager.RunTx, в тестах — заглушка.
type txRunner func(ctx context.Context, fn func(pgx.Tx) error) error

type catalogUsecase struct {
	runTx        txRunner
	productRepo  repository.ProductRepository
	outboxRepo   repository.OutboxRepository
	searchRepo   repository.ProductSearchRepository
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewCatalogUsecase(
	runTx txRunner,
	productRepo repository.ProductRepository,
	outboxRepo repository.OutboxRepository,
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
		runTx:        runTx,
		productRepo:  productRepo,
		outboxRepo:   outboxRepo,
		searchRepo:   searchRepo,
		callTimeout:  callTimeout,
		queryTimeout: queryTimeout,
	}
}

func (u *catalogUsecase) CreateProduct(ctx context.Context, name, description string, price int64, categories []string, idempotencyKey string) (uuid.UUID, error) {
	product := &domain.Product{
		ID:             uuid.New(),
		Name:           name,
		Description:    description,
		Price:          price,
		Categories:     categories,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now().UTC(),
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

	var createdID uuid.UUID
	if err := u.runTx(txCtx, func(tx pgx.Tx) error {
		productTx := u.productRepo.WithTx(tx)
		outboxTx := u.outboxRepo.WithTx(tx)

		id, err := productTx.Create(txCtx, product)
		createdID = id
		if err != nil {
			return fmt.Errorf("create product: %w", err)
		}

		if err := outboxTx.Create(txCtx, event); err != nil {
			return fmt.Errorf("create outbox event: %w", err)
		}

		return nil
	}); err != nil {
		return createdID, err
	}

	return createdID, nil
}

func (u *catalogUsecase) GetProduct(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.productRepo.GetByID(ctx, id)
}

func (u *catalogUsecase) UpdateProduct(ctx context.Context, id uuid.UUID, name, description string, price int64, categories []string) error {
	txCtx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()

	return u.runTx(txCtx, func(tx pgx.Tx) error {
		productTx := u.productRepo.WithTx(tx)
		outboxTx := u.outboxRepo.WithTx(tx)

		existing, err := productTx.GetByID(txCtx, id)
		if err != nil {
			return fmt.Errorf("get product: %w", err)
		}

		if name != "" {
			existing.Name = name
		}
		if description != "" {
			existing.Description = description
		}
		if price > 0 {
			existing.Price = price
		}
		if len(categories) > 0 {
			existing.Categories = categories
		}

		if err := productTx.Update(txCtx, existing); err != nil {
			return fmt.Errorf("update product: %w", err)
		}

		payload, err := json.Marshal(existing)
		if err != nil {
			return fmt.Errorf("marshal product payload: %w", err)
		}

		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "product",
			AggregateID:   existing.ID.String(),
			EventType:     "ProductUpdated",
			Payload:       payload,
			CreatedAt:     time.Now().UTC(),
		}

		if err := outboxTx.Create(txCtx, event); err != nil {
			return fmt.Errorf("create outbox event: %w", err)
		}

		return nil
	})
}

func (u *catalogUsecase) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	txCtx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()

	return u.runTx(txCtx, func(tx pgx.Tx) error {
		productTx := u.productRepo.WithTx(tx)
		outboxTx := u.outboxRepo.WithTx(tx)

		if err := productTx.Delete(txCtx, id); err != nil {
			return fmt.Errorf("delete product: %w", err)
		}

		payload, err := json.Marshal(map[string]string{"product_id": id.String()})
		if err != nil {
			return fmt.Errorf("marshal delete payload: %w", err)
		}

		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "product",
			AggregateID:   id.String(),
			EventType:     "ProductDeleted",
			Payload:       payload,
			CreatedAt:     time.Now().UTC(),
		}

		if err := outboxTx.Create(txCtx, event); err != nil {
			return fmt.Errorf("create outbox event: %w", err)
		}

		return nil
	})
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

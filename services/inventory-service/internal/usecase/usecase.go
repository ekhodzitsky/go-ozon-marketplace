package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	DefaultCallTimeout  = 5 * time.Second
	DefaultQueryTimeout = 3 * time.Second
	cacheTTL            = 5 * time.Minute
)

type inventoryUsecase struct {
	repo         repository.InventoryRepository
	redis        *redis.Client
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewInventoryUsecase(repo repository.InventoryRepository, redis *redis.Client, callTimeout time.Duration, queryTimeout time.Duration) InventoryUsecase {
	if callTimeout == 0 {
		callTimeout = DefaultCallTimeout
	}
	if queryTimeout == 0 {
		queryTimeout = DefaultQueryTimeout
	}
	return &inventoryUsecase{repo: repo, redis: redis, callTimeout: callTimeout, queryTimeout: queryTimeout}
}

func cacheKey(productID uuid.UUID) string {
	return fmt.Sprintf("inventory:%s", productID.String())
}

func (u *inventoryUsecase) GetStock(ctx context.Context, productID uuid.UUID) (*domain.Stock, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()

	key := cacheKey(productID)
	val, err := u.redis.Get(ctx, key).Result()
	if err == nil {
		var stock domain.Stock
		if err := json.Unmarshal([]byte(val), &stock); err == nil {
			return &stock, nil
		}
	}

	stock, err := u.repo.GetStock(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("get stock: %w", err)
	}

	data, _ := json.Marshal(stock)
	u.redis.Set(ctx, key, data, cacheTTL)
	return stock, nil
}

func (u *inventoryUsecase) Reserve(ctx context.Context, productID uuid.UUID, quantity int, orderID string) error {
	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()

	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		return fmt.Errorf("invalid order_id: %w", err)
	}

	if err := u.repo.Reserve(ctx, productID, quantity, orderUUID); err != nil {
		return fmt.Errorf("reserve: %w", err)
	}
	u.redis.Del(ctx, cacheKey(productID))
	return nil
}

func (u *inventoryUsecase) Release(ctx context.Context, productID uuid.UUID, quantity int, orderID string) error {
	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()

	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		return fmt.Errorf("invalid order_id: %w", err)
	}

	if err := u.repo.Release(ctx, productID, quantity, orderUUID); err != nil {
		return fmt.Errorf("release: %w", err)
	}
	u.redis.Del(ctx, cacheKey(productID))
	return nil
}

func (u *inventoryUsecase) GetLedger(ctx context.Context, productID uuid.UUID) ([]*domain.LedgerEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.repo.GetLedger(ctx, productID)
}

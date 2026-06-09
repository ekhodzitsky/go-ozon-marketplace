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

const cacheTTL = 5 * time.Minute

type InventoryUsecase struct {
	repo  repository.InventoryRepository
	redis *redis.Client
}

func NewInventoryUsecase(repo repository.InventoryRepository, redis *redis.Client) *InventoryUsecase {
	return &InventoryUsecase{repo: repo, redis: redis}
}

func cacheKey(productID uuid.UUID) string {
	return fmt.Sprintf("inventory:%s", productID.String())
}

func (u *InventoryUsecase) GetStock(ctx context.Context, productID uuid.UUID) (*domain.Stock, error) {
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

func (u *InventoryUsecase) Reserve(ctx context.Context, productID uuid.UUID, quantity int, orderID string) error {
	if err := u.repo.Reserve(ctx, productID, quantity); err != nil {
		return fmt.Errorf("reserve: %w", err)
	}
	u.redis.Del(ctx, cacheKey(productID))
	return nil
}

func (u *InventoryUsecase) Release(ctx context.Context, productID uuid.UUID, quantity int, orderID string) error {
	if err := u.repo.Release(ctx, productID, quantity); err != nil {
		return fmt.Errorf("release: %w", err)
	}
	u.redis.Del(ctx, cacheKey(productID))
	return nil
}

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
	// Cache read failure (including redis.Nil) is non-fatal; fall through to the database.

	stock, err := u.repo.GetStock(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("get stock: %w", err)
	}

	// Populate cache write-through style; ignore cache-write errors so the
	// request still succeeds when Redis is unavailable.
	if data, marshalErr := json.Marshal(stock); marshalErr == nil {
		_ = u.redis.Set(ctx, key, data, cacheTTL).Err()
	}
	return stock, nil
}

func (u *inventoryUsecase) publishInventoryEvent(ctx context.Context, productID uuid.UUID) {
	if u.redis == nil {
		return
	}
	stock, err := u.repo.GetStock(ctx, productID)
	if err != nil {
		return
	}
	event := map[string]interface{}{
		"topic": "inventory",
		"payload": map[string]interface{}{
			"product_id": productID.String(),
			"available":  stock.Available,
			"reserved":   stock.Reserved,
		},
	}
	data, _ := json.Marshal(event)
	pubCtx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	u.redis.Publish(pubCtx, "inventory-events", string(data))
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
	// Write updated stock through to cache instead of best-effort deletion.
	u.writeStockToCache(ctx, productID)
	u.publishInventoryEvent(ctx, productID)
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
	// Write updated stock through to cache instead of best-effort deletion.
	u.writeStockToCache(ctx, productID)
	u.publishInventoryEvent(ctx, productID)
	return nil
}

// writeStockToCache refreshes the cached stock value after a mutation.
// Cache failures are intentionally ignored: the authoritative source is Postgres.
func (u *inventoryUsecase) writeStockToCache(ctx context.Context, productID uuid.UUID) {
	stock, err := u.repo.GetStock(ctx, productID)
	if err != nil {
		return
	}
	data, err := json.Marshal(stock)
	if err != nil {
		return
	}
	_ = u.redis.Set(ctx, cacheKey(productID), data, cacheTTL).Err()
}

func (u *inventoryUsecase) GetLedger(ctx context.Context, productID uuid.UUID) ([]*domain.LedgerEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.repo.GetLedger(ctx, productID)
}

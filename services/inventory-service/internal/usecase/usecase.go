package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
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

const (
	ledgerOpReserve = "LEDGER_OPERATION_RESERVE"
	ledgerOpRelease = "LEDGER_OPERATION_RELEASE"
)

type inventoryUsecase struct {
	repo         repository.InventoryRepository
	txm          repository.TxManager
	redis        *redis.Client
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewInventoryUsecase(repo repository.InventoryRepository, txm repository.TxManager, redis *redis.Client, callTimeout time.Duration, queryTimeout time.Duration) InventoryUsecase {
	if callTimeout == 0 {
		callTimeout = DefaultCallTimeout
	}
	if queryTimeout == 0 {
		queryTimeout = DefaultQueryTimeout
	}
	return &inventoryUsecase{repo: repo, txm: txm, redis: redis, callTimeout: callTimeout, queryTimeout: queryTimeout}
}

func cacheKey(productID uuid.UUID) string {
	return fmt.Sprintf("inventory:%s", productID.String())
}

func (u *inventoryUsecase) GetStock(ctx context.Context, productID uuid.UUID) (*domain.Stock, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()

	key := cacheKey(productID)
	if u.redis != nil {
		val, err := u.redis.Get(ctx, key).Result()
		if err == nil {
			var stock domain.Stock
			if err := json.Unmarshal([]byte(val), &stock); err == nil {
				return &stock, nil
			}
		}
		// Cache read failure (including redis.Nil) is non-fatal; fall through to the database.
	}

	stock, err := u.repo.GetStock(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("get stock: %w", err)
	}

	// Populate cache write-through style; ignore cache-write errors so the
	// request still succeeds when Redis is unavailable.
	if u.redis != nil {
		if data, marshalErr := json.Marshal(stock); marshalErr == nil {
			_ = u.redis.Set(ctx, key, data, cacheTTL).Err()
		}
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

	if quantity <= 0 {
		return fmt.Errorf("%w: quantity must be positive", apperrors.ErrInvalidArgument)
	}

	if err := u.txm.Run(ctx, func(repo repository.InventoryRepository) error {
		return reserve(ctx, repo, productID, quantity, orderUUID)
	}); err != nil {
		return fmt.Errorf("reserve: %w", err)
	}

	// Write updated stock through to cache instead of best-effort deletion.
	u.writeStockToCache(ctx, productID)
	u.publishInventoryEvent(ctx, productID)
	return nil
}

func reserve(ctx context.Context, repo repository.InventoryRepository, productID uuid.UUID, quantity int, orderID uuid.UUID) error {
	// Idempotent reservation record. If the same (order, product) already exists
	// with a matching quantity and status, this retry must not deduct stock again.
	rowsAffected, err := repo.InsertReservation(ctx, orderID, productID, quantity)
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		// Already reserved for this (order, product). Verify it matches the request.
		existing, err := repo.SelectReservation(ctx, orderID, productID)
		if err != nil {
			return err
		}
		if existing.Status != "reserved" {
			return fmt.Errorf("%w: reservation already %s", apperrors.ErrFailedPrecondition, existing.Status)
		}
		if existing.Quantity != quantity {
			return fmt.Errorf("%w: reservation quantity mismatch (expected %d, found %d)", apperrors.ErrConflict, quantity, existing.Quantity)
		}
		return nil
	}

	// Atomic stock deduction with oversell protection.
	affected, err := repo.UpdateStockForReserve(ctx, productID, quantity)
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.ErrInsufficientStock
	}

	// Record the operation in the inventory ledger inside the same transaction.
	if err := repo.InsertLedger(ctx, productID, orderID, -quantity, ledgerOpReserve); err != nil {
		return err
	}
	return nil
}

func (u *inventoryUsecase) Release(ctx context.Context, productID uuid.UUID, quantity int, orderID string) error {
	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()

	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		return fmt.Errorf("invalid order_id: %w", err)
	}

	if quantity <= 0 {
		return fmt.Errorf("%w: quantity must be positive", apperrors.ErrInvalidArgument)
	}

	if err := u.txm.Run(ctx, func(repo repository.InventoryRepository) error {
		return release(ctx, repo, productID, quantity, orderUUID)
	}); err != nil {
		return fmt.Errorf("release: %w", err)
	}

	// Write updated stock through to cache instead of best-effort deletion.
	u.writeStockToCache(ctx, productID)
	u.publishInventoryEvent(ctx, productID)
	return nil
}

func release(ctx context.Context, repo repository.InventoryRepository, productID uuid.UUID, quantity int, orderID uuid.UUID) error {
	// Bind release to the existing reservation row.
	reserved, err := repo.SelectReservationForUpdate(ctx, orderID, productID)
	if err != nil {
		return err
	}

	if reserved.Status == "released" && reserved.Quantity == quantity {
		// Idempotent release: already released for this (order, product, quantity).
		return nil
	}
	if reserved.Status != "reserved" {
		return fmt.Errorf("%w: reservation already %s", apperrors.ErrFailedPrecondition, reserved.Status)
	}
	if reserved.Quantity != quantity {
		return fmt.Errorf("%w: release quantity mismatch (reserved %d, requested %d)", apperrors.ErrConflict, reserved.Quantity, quantity)
	}

	// Atomic stock release with over-release protection.
	affected, err := repo.UpdateStockForRelease(ctx, productID, quantity)
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.ErrInsufficientStock
	}

	if err := repo.UpdateReservationStatus(ctx, orderID, productID, "released"); err != nil {
		return err
	}

	// Record the operation in the inventory ledger inside the same transaction.
	if err := repo.InsertLedger(ctx, productID, orderID, quantity, ledgerOpRelease); err != nil {
		return err
	}
	return nil
}

// writeStockToCache refreshes the cached stock value after a mutation.
// Cache failures are intentionally ignored: the authoritative source is Postgres.
func (u *inventoryUsecase) writeStockToCache(ctx context.Context, productID uuid.UUID) {
	if u.redis == nil {
		return
	}
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

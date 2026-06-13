package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockInventoryRepository is a test double for InventoryRepository.
type mockInventoryRepository struct {
	stock         *domain.Stock
	stockErr      error
	reserveErr    error
	releaseErr    error
	ledger        []*domain.LedgerEntry
	ledgerErr     error
	reserveCalled int
	releaseCalled int
}

func (m *mockInventoryRepository) GetStock(_ context.Context, _ uuid.UUID) (*domain.Stock, error) {
	if m.stockErr != nil {
		return nil, m.stockErr
	}
	if m.stock == nil {
		return nil, errors.New("not found")
	}
	return m.stock, nil
}

func (m *mockInventoryRepository) Reserve(_ context.Context, _ uuid.UUID, _ int, _ uuid.UUID) error {
	m.reserveCalled++
	return m.reserveErr
}

func (m *mockInventoryRepository) Release(_ context.Context, _ uuid.UUID, _ int, _ uuid.UUID) error {
	m.releaseCalled++
	return m.releaseErr
}

func (m *mockInventoryRepository) GetLedger(_ context.Context, _ uuid.UUID) ([]*domain.LedgerEntry, error) {
	if m.ledgerErr != nil {
		return nil, m.ledgerErr
	}
	return m.ledger, nil
}

var _ repository.InventoryRepository = (*mockInventoryRepository)(nil)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestInventoryUsecase_GetStock_FromCache(t *testing.T) {
	productID := uuid.New()
	stock := &domain.Stock{ProductID: productID, Available: 42, Reserved: 3}

	redisClient := newTestRedis(t)
	data, err := json.Marshal(stock)
	require.NoError(t, err)
	require.NoError(t, redisClient.Set(context.Background(), cacheKey(productID), data, cacheTTL).Err())

	repo := &mockInventoryRepository{stockErr: errors.New("should not be called")}
	uc := NewInventoryUsecase(repo, redisClient, time.Second, time.Second)

	got, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, stock, got)
}

func TestInventoryUsecase_GetStock_FallsBackToRepository(t *testing.T) {
	productID := uuid.New()
	stock := &domain.Stock{ProductID: productID, Available: 10, Reserved: 1}

	redisClient := newTestRedis(t)
	repo := &mockInventoryRepository{stock: stock}
	uc := NewInventoryUsecase(repo, redisClient, time.Second, time.Second)

	got, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, stock, got)

	// Cache should be populated.
	cached, err := redisClient.Get(context.Background(), cacheKey(productID)).Result()
	require.NoError(t, err)
	var cachedStock domain.Stock
	require.NoError(t, json.Unmarshal([]byte(cached), &cachedStock))
	assert.Equal(t, *stock, cachedStock)
}

func TestInventoryUsecase_GetStock_PropagatesError(t *testing.T) {
	repo := &mockInventoryRepository{stockErr: errors.New("db error")}
	uc := NewInventoryUsecase(repo, newTestRedis(t), time.Second, time.Second)

	_, err := uc.GetStock(context.Background(), uuid.New())
	require.Error(t, err)
	assert.ErrorContains(t, err, "get stock")
}

func TestInventoryUsecase_GetStock_InvalidCacheJSON(t *testing.T) {
	productID := uuid.New()
	stock := &domain.Stock{ProductID: productID, Available: 7, Reserved: 2}

	redisClient := newTestRedis(t)
	require.NoError(t, redisClient.Set(context.Background(), cacheKey(productID), "not-json", cacheTTL).Err())

	repo := &mockInventoryRepository{stock: stock}
	uc := NewInventoryUsecase(repo, redisClient, time.Second, time.Second)

	got, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, stock, got)
}

func TestInventoryUsecase_Reserve_Success(t *testing.T) {
	productID := uuid.New()
	orderID := uuid.New()

	repo := &mockInventoryRepository{}
	uc := NewInventoryUsecase(repo, newTestRedis(t), time.Second, time.Second)

	err := uc.Reserve(context.Background(), productID, 5, orderID.String())
	require.NoError(t, err)
	assert.Equal(t, 1, repo.reserveCalled)
}

func TestInventoryUsecase_Reserve_InvalidOrderID(t *testing.T) {
	uc := NewInventoryUsecase(&mockInventoryRepository{}, nil, time.Second, time.Second)

	err := uc.Reserve(context.Background(), uuid.New(), 1, "not-a-uuid")
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid order_id")
}

func TestInventoryUsecase_Reserve_RepositoryError(t *testing.T) {
	productID := uuid.New()
	orderID := uuid.New()

	repo := &mockInventoryRepository{reserveErr: errors.New("reserve failed")}
	uc := NewInventoryUsecase(repo, newTestRedis(t), time.Second, time.Second)

	err := uc.Reserve(context.Background(), productID, 5, orderID.String())
	require.Error(t, err)
	assert.ErrorContains(t, err, "reserve:")
}

func TestInventoryUsecase_Release_Success(t *testing.T) {
	productID := uuid.New()
	orderID := uuid.New()

	repo := &mockInventoryRepository{}
	uc := NewInventoryUsecase(repo, newTestRedis(t), time.Second, time.Second)

	err := uc.Release(context.Background(), productID, 5, orderID.String())
	require.NoError(t, err)
	assert.Equal(t, 1, repo.releaseCalled)
}

func TestInventoryUsecase_Release_InvalidOrderID(t *testing.T) {
	uc := NewInventoryUsecase(&mockInventoryRepository{}, nil, time.Second, time.Second)

	err := uc.Release(context.Background(), uuid.New(), 1, "not-a-uuid")
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid order_id")
}

func TestInventoryUsecase_Release_RepositoryError(t *testing.T) {
	productID := uuid.New()
	orderID := uuid.New()

	repo := &mockInventoryRepository{releaseErr: errors.New("release failed")}
	uc := NewInventoryUsecase(repo, newTestRedis(t), time.Second, time.Second)

	err := uc.Release(context.Background(), productID, 5, orderID.String())
	require.Error(t, err)
	assert.ErrorContains(t, err, "release:")
}

func TestInventoryUsecase_GetLedger_Success(t *testing.T) {
	productID := uuid.New()
	entries := []*domain.LedgerEntry{
		{ProductID: productID, QuantityChange: -5, OperationType: "LEDGER_OPERATION_RESERVE"},
	}

	repo := &mockInventoryRepository{ledger: entries}
	uc := NewInventoryUsecase(repo, nil, time.Second, time.Second)

	got, err := uc.GetLedger(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, entries, got)
}

func TestInventoryUsecase_GetLedger_Error(t *testing.T) {
	repo := &mockInventoryRepository{ledgerErr: errors.New("ledger error")}
	uc := NewInventoryUsecase(repo, nil, time.Second, time.Second)

	_, err := uc.GetLedger(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestInventoryUsecase_publishInventoryEvent_NilRedis(t *testing.T) {
	repo := &mockInventoryRepository{stock: &domain.Stock{Available: 1, Reserved: 0}}
	uc := &inventoryUsecase{repo: repo, redis: nil, callTimeout: time.Second, queryTimeout: time.Second}

	// Should not panic.
	uc.publishInventoryEvent(context.Background(), uuid.New())
}

func TestInventoryUsecase_writeStockToCache_IgnoresErrors(t *testing.T) {
	repo := &mockInventoryRepository{stockErr: errors.New("db error")}
	uc := &inventoryUsecase{repo: repo, redis: newTestRedis(t), callTimeout: time.Second, queryTimeout: time.Second}

	// Should not panic or return error.
	uc.writeStockToCache(context.Background(), uuid.New())
}

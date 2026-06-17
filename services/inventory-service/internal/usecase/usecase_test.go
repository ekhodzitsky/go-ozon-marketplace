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
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockInventoryRepository is a test double for InventoryRepository.
type mockInventoryRepository struct {
	stock                           *domain.Stock
	stockErr                        error
	ledger                          []*domain.LedgerEntry
	ledgerErr                       error
	insertReservationRows           int64
	insertReservationErr            error
	selectReservation               *repository.ReservationRow
	selectReservationErr            error
	selectReservationForUpdate      *repository.ReservationRow
	selectReservationForUpdateErr   error
	updateReservationStatusErr      error
	updateStockForReserveRows       int64
	updateStockForReserveErr        error
	updateStockForReleaseRows       int64
	updateStockForReleaseErr        error
	insertLedgerErr                 error

	insertReservationCalled          int
	selectReservationCalled          int
	selectReservationForUpdateCalled int
	updateReservationStatusCalled    int
	updateStockForReserveCalled      int
	updateStockForReleaseCalled      int
	insertLedgerCalled               int
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

func (m *mockInventoryRepository) GetLedger(_ context.Context, _ uuid.UUID) ([]*domain.LedgerEntry, error) {
	if m.ledgerErr != nil {
		return nil, m.ledgerErr
	}
	return m.ledger, nil
}

func (m *mockInventoryRepository) InsertReservation(_ context.Context, _, _ uuid.UUID, _ int) (int64, error) {
	m.insertReservationCalled++
	return m.insertReservationRows, m.insertReservationErr
}

func (m *mockInventoryRepository) SelectReservation(_ context.Context, _, _ uuid.UUID) (*repository.ReservationRow, error) {
	m.selectReservationCalled++
	return m.selectReservation, m.selectReservationErr
}

func (m *mockInventoryRepository) SelectReservationForUpdate(_ context.Context, _, _ uuid.UUID) (*repository.ReservationRow, error) {
	m.selectReservationForUpdateCalled++
	return m.selectReservationForUpdate, m.selectReservationForUpdateErr
}

func (m *mockInventoryRepository) UpdateReservationStatus(_ context.Context, _, _ uuid.UUID, _ string) error {
	m.updateReservationStatusCalled++
	return m.updateReservationStatusErr
}

func (m *mockInventoryRepository) UpdateStockForReserve(_ context.Context, _ uuid.UUID, _ int) (int64, error) {
	m.updateStockForReserveCalled++
	return m.updateStockForReserveRows, m.updateStockForReserveErr
}

func (m *mockInventoryRepository) UpdateStockForRelease(_ context.Context, _ uuid.UUID, _ int) (int64, error) {
	m.updateStockForReleaseCalled++
	return m.updateStockForReleaseRows, m.updateStockForReleaseErr
}

func (m *mockInventoryRepository) InsertLedger(_ context.Context, _, _ uuid.UUID, _ int, _ string) error {
	m.insertLedgerCalled++
	return m.insertLedgerErr
}

func (m *mockInventoryRepository) WithTx(_ pgx.Tx) repository.InventoryRepository {
	return m
}

var _ repository.InventoryRepository = (*mockInventoryRepository)(nil)

// fakeTxManager runs the callback against the provided repository without an actual DB transaction.
type fakeTxManager struct {
	repo repository.InventoryRepository
}

func (f *fakeTxManager) Run(_ context.Context, fn func(repo repository.InventoryRepository) error) error {
	return fn(f.repo)
}

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func newTestUsecase(t *testing.T, repo repository.InventoryRepository, redisClient *redis.Client) InventoryUsecase {
	t.Helper()
	return NewInventoryUsecase(repo, &fakeTxManager{repo: repo}, redisClient, time.Second, time.Second)
}

func TestInventoryUsecase_GetStock_FromCache(t *testing.T) {
	productID := uuid.New()
	stock := &domain.Stock{ProductID: productID, Available: 42, Reserved: 3}

	redisClient := newTestRedis(t)
	data, err := json.Marshal(stock)
	require.NoError(t, err)
	require.NoError(t, redisClient.Set(context.Background(), cacheKey(productID), data, cacheTTL).Err())

	repo := &mockInventoryRepository{stockErr: errors.New("should not be called")}
	uc := newTestUsecase(t, repo, redisClient)

	got, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, stock, got)
}

func TestInventoryUsecase_GetStock_FallsBackToRepository(t *testing.T) {
	productID := uuid.New()
	stock := &domain.Stock{ProductID: productID, Available: 10, Reserved: 1}

	redisClient := newTestRedis(t)
	repo := &mockInventoryRepository{stock: stock}
	uc := newTestUsecase(t, repo, redisClient)

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
	uc := newTestUsecase(t, repo, newTestRedis(t))

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
	uc := newTestUsecase(t, repo, redisClient)

	got, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, stock, got)
}

func TestInventoryUsecase_Reserve_Success(t *testing.T) {
	productID := uuid.New()
	orderID := uuid.New()

	repo := &mockInventoryRepository{
		insertReservationRows:     1,
		updateStockForReserveRows: 1,
	}
	uc := newTestUsecase(t, repo, newTestRedis(t))

	err := uc.Reserve(context.Background(), productID, 5, orderID.String())
	require.NoError(t, err)
	assert.Equal(t, 1, repo.insertReservationCalled)
	assert.Equal(t, 1, repo.updateStockForReserveCalled)
	assert.Equal(t, 1, repo.insertLedgerCalled)
}

func TestInventoryUsecase_Reserve_InvalidOrderID(t *testing.T) {
	uc := NewInventoryUsecase(&mockInventoryRepository{}, &fakeTxManager{repo: &mockInventoryRepository{}}, nil, time.Second, time.Second)

	err := uc.Reserve(context.Background(), uuid.New(), 1, "not-a-uuid")
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid order_id")
}

func TestInventoryUsecase_Reserve_InvalidQuantity(t *testing.T) {
	uc := newTestUsecase(t, &mockInventoryRepository{}, nil)

	err := uc.Reserve(context.Background(), uuid.New(), 0, uuid.New().String())
	require.Error(t, err)
	assert.ErrorContains(t, err, "quantity must be positive")
}

func TestInventoryUsecase_Reserve_RepositoryError(t *testing.T) {
	productID := uuid.New()
	orderID := uuid.New()

	repo := &mockInventoryRepository{insertReservationErr: errors.New("reserve failed")}
	uc := newTestUsecase(t, repo, newTestRedis(t))

	err := uc.Reserve(context.Background(), productID, 5, orderID.String())
	require.Error(t, err)
	assert.ErrorContains(t, err, "reserve:")
}

func TestInventoryUsecase_Reserve_Idempotent(t *testing.T) {
	productID := uuid.New()
	orderID := uuid.New()

	repo := &mockInventoryRepository{
		insertReservationRows: 0,
		selectReservation:     &repository.ReservationRow{Quantity: 5, Status: "reserved"},
	}
	uc := newTestUsecase(t, repo, newTestRedis(t))

	err := uc.Reserve(context.Background(), productID, 5, orderID.String())
	require.NoError(t, err)
	assert.Equal(t, 1, repo.insertReservationCalled)
	assert.Equal(t, 1, repo.selectReservationCalled)
	assert.Equal(t, 0, repo.updateStockForReserveCalled)
}

func TestInventoryUsecase_Reserve_IdempotentQuantityMismatch(t *testing.T) {
	productID := uuid.New()
	orderID := uuid.New()

	repo := &mockInventoryRepository{
		insertReservationRows: 0,
		selectReservation:     &repository.ReservationRow{Quantity: 3, Status: "reserved"},
	}
	uc := newTestUsecase(t, repo, nil)

	err := uc.Reserve(context.Background(), productID, 5, orderID.String())
	require.Error(t, err)
	assert.ErrorContains(t, err, "reserve:")
}

func TestInventoryUsecase_Release_Success(t *testing.T) {
	productID := uuid.New()
	orderID := uuid.New()

	repo := &mockInventoryRepository{
		selectReservationForUpdate: &repository.ReservationRow{Quantity: 5, Status: "reserved"},
		updateStockForReleaseRows:  1,
	}
	uc := newTestUsecase(t, repo, newTestRedis(t))

	err := uc.Release(context.Background(), productID, 5, orderID.String())
	require.NoError(t, err)
	assert.Equal(t, 1, repo.selectReservationForUpdateCalled)
	assert.Equal(t, 1, repo.updateStockForReleaseCalled)
	assert.Equal(t, 1, repo.updateReservationStatusCalled)
	assert.Equal(t, 1, repo.insertLedgerCalled)
}

func TestInventoryUsecase_Release_InvalidOrderID(t *testing.T) {
	uc := NewInventoryUsecase(&mockInventoryRepository{}, &fakeTxManager{repo: &mockInventoryRepository{}}, nil, time.Second, time.Second)

	err := uc.Release(context.Background(), uuid.New(), 1, "not-a-uuid")
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid order_id")
}

func TestInventoryUsecase_Release_InvalidQuantity(t *testing.T) {
	uc := newTestUsecase(t, &mockInventoryRepository{}, nil)

	err := uc.Release(context.Background(), uuid.New(), 0, uuid.New().String())
	require.Error(t, err)
	assert.ErrorContains(t, err, "quantity must be positive")
}

func TestInventoryUsecase_Release_RepositoryError(t *testing.T) {
	productID := uuid.New()
	orderID := uuid.New()

	repo := &mockInventoryRepository{selectReservationForUpdateErr: errors.New("release failed")}
	uc := newTestUsecase(t, repo, newTestRedis(t))

	err := uc.Release(context.Background(), productID, 5, orderID.String())
	require.Error(t, err)
	assert.ErrorContains(t, err, "release:")
}

func TestInventoryUsecase_Release_Idempotent(t *testing.T) {
	productID := uuid.New()
	orderID := uuid.New()

	repo := &mockInventoryRepository{
		selectReservationForUpdate: &repository.ReservationRow{Quantity: 5, Status: "released"},
	}
	uc := newTestUsecase(t, repo, newTestRedis(t))

	err := uc.Release(context.Background(), productID, 5, orderID.String())
	require.NoError(t, err)
	assert.Equal(t, 1, repo.selectReservationForUpdateCalled)
	assert.Equal(t, 0, repo.updateStockForReleaseCalled)
}

func TestInventoryUsecase_GetLedger_Success(t *testing.T) {
	productID := uuid.New()
	entries := []*domain.LedgerEntry{
		{ProductID: productID, QuantityChange: -5, OperationType: "LEDGER_OPERATION_RESERVE"},
	}

	repo := &mockInventoryRepository{ledger: entries}
	uc := newTestUsecase(t, repo, nil)

	got, err := uc.GetLedger(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, entries, got)
}

func TestInventoryUsecase_GetLedger_Error(t *testing.T) {
	repo := &mockInventoryRepository{ledgerErr: errors.New("ledger error")}
	uc := newTestUsecase(t, repo, nil)

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

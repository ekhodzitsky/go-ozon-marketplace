//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/usecase"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	sharedDSN     string
	sharedOnce    sync.Once
	sharedCleanup func()
)

func startSharedPostgres(t *testing.T) string {
	t.Helper()

	sharedOnce.Do(func() {
		ctx := context.Background()
		container, err := postgres.Run(ctx, "postgres:16-alpine",
			postgres.WithDatabase("marketplace"),
			postgres.WithUsername("ozon"),
			postgres.WithPassword("ozonpass"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			),
		)
		require.NoError(t, err)

		sharedCleanup = func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = container.Terminate(ctx)
		}

		sharedDSN, err = container.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, err)

		runMigrations(t, sharedDSN)
	})

	return sharedDSN
}

func runMigrations(t *testing.T, dsn string) {
	t.Helper()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	dir := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		content, err := os.ReadFile(filepath.Join(dir, file))
		require.NoError(t, err)

		tx, err := pool.Begin(ctx)
		require.NoError(t, err)

		_, err = tx.Exec(ctx, string(content))
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("failed to execute migration %s: %v", file, err)
		}
		require.NoError(t, tx.Commit(ctx))
	}
}

func newTestUsecase(t *testing.T) (usecase.InventoryUsecase, *pgxpool.Pool) {
	t.Helper()

	dsn := startSharedPostgres(t)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	repo := NewInventoryPostgres(pool)
	txm := NewInventoryTxManager(pool, repo)
	return usecase.NewInventoryUsecase(repo, txm, nil, 5*time.Second, 3*time.Second), pool
}

func TestMain(m *testing.M) {
	code := m.Run()
	if sharedCleanup != nil {
		sharedCleanup()
	}
	os.Exit(code)
}

func seedProduct(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID, available int) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`INSERT INTO inventory (product_id, available, reserved) VALUES ($1, $2, 0)
		 ON CONFLICT (product_id) DO UPDATE SET available = $2, reserved = 0`,
		productID, available)
	require.NoError(t, err)
}

func TestInventoryPostgres_GetStock_NotFound(t *testing.T) {
	uc, _ := newTestUsecase(t)

	_, err := uc.GetStock(context.Background(), uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

func TestInventoryPostgres_GetStock_Success(t *testing.T) {
	uc, pool := newTestUsecase(t)
	productID := uuid.New()
	seedProduct(t, pool, productID, 100)

	stock, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, productID, stock.ProductID)
	assert.Equal(t, 100, stock.Available)
	assert.Equal(t, 0, stock.Reserved)
}

func TestInventoryPostgres_Reserve_Success(t *testing.T) {
	uc, pool := newTestUsecase(t)
	productID := uuid.New()
	orderID := uuid.New()
	seedProduct(t, pool, productID, 10)

	err := uc.Reserve(context.Background(), productID, 5, orderID.String())
	require.NoError(t, err)

	stock, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, 5, stock.Available)
	assert.Equal(t, 5, stock.Reserved)
}

func TestInventoryPostgres_Reserve_InsufficientStock(t *testing.T) {
	uc, pool := newTestUsecase(t)
	productID := uuid.New()
	orderID := uuid.New()
	seedProduct(t, pool, productID, 3)

	err := uc.Reserve(context.Background(), productID, 5, orderID.String())
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInsufficientStock))
}

func TestInventoryPostgres_Reserve_InvalidQuantity(t *testing.T) {
	uc, _ := newTestUsecase(t)

	err := uc.Reserve(context.Background(), uuid.New(), 0, uuid.New().String())
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidArgument))
}

func TestInventoryPostgres_Reserve_Idempotent(t *testing.T) {
	uc, pool := newTestUsecase(t)
	productID := uuid.New()
	orderID := uuid.New()
	seedProduct(t, pool, productID, 10)

	// First reservation.
	require.NoError(t, uc.Reserve(context.Background(), productID, 5, orderID.String()))

	// Second reservation with same parameters is idempotent.
	require.NoError(t, uc.Reserve(context.Background(), productID, 5, orderID.String()))

	stock, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, 5, stock.Available)
	assert.Equal(t, 5, stock.Reserved)
}

func TestInventoryPostgres_Reserve_IdempotentQuantityMismatch(t *testing.T) {
	uc, pool := newTestUsecase(t)
	productID := uuid.New()
	orderID := uuid.New()
	seedProduct(t, pool, productID, 10)

	require.NoError(t, uc.Reserve(context.Background(), productID, 5, orderID.String()))

	err := uc.Reserve(context.Background(), productID, 3, orderID.String())
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrConflict))
}

func TestInventoryPostgres_Reserve_Concurrent(t *testing.T) {
	uc, pool := newTestUsecase(t)
	productID := uuid.New()
	seedProduct(t, pool, productID, 10)

	const workers = 5
	const quantity = 3

	var wg sync.WaitGroup
	successes := make(chan bool, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := uc.Reserve(context.Background(), productID, quantity, uuid.New().String())
			successes <- err == nil
		}()
	}

	wg.Wait()
	close(successes)

	successCount := 0
	for ok := range successes {
		if ok {
			successCount++
		}
	}

	// Only floor(10/3) = 3 reservations can succeed without overselling.
	assert.Equal(t, 3, successCount)

	stock, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, 1, stock.Available)
	assert.Equal(t, 9, stock.Reserved)
}

func TestInventoryPostgres_Release_Success(t *testing.T) {
	uc, pool := newTestUsecase(t)
	productID := uuid.New()
	orderID := uuid.New()
	seedProduct(t, pool, productID, 10)

	require.NoError(t, uc.Reserve(context.Background(), productID, 5, orderID.String()))
	require.NoError(t, uc.Release(context.Background(), productID, 5, orderID.String()))

	stock, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, 10, stock.Available)
	assert.Equal(t, 0, stock.Reserved)
}

func TestInventoryPostgres_Release_Idempotent(t *testing.T) {
	uc, pool := newTestUsecase(t)
	productID := uuid.New()
	orderID := uuid.New()
	seedProduct(t, pool, productID, 10)

	require.NoError(t, uc.Reserve(context.Background(), productID, 5, orderID.String()))
	require.NoError(t, uc.Release(context.Background(), productID, 5, orderID.String()))
	require.NoError(t, uc.Release(context.Background(), productID, 5, orderID.String()))

	stock, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, 10, stock.Available)
	assert.Equal(t, 0, stock.Reserved)
}

func TestInventoryPostgres_Release_ReservationNotFound(t *testing.T) {
	uc, _ := newTestUsecase(t)

	err := uc.Release(context.Background(), uuid.New(), 1, uuid.New().String())
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

func TestInventoryPostgres_Release_AlreadyReleased(t *testing.T) {
	uc, pool := newTestUsecase(t)
	productID := uuid.New()
	orderID := uuid.New()
	seedProduct(t, pool, productID, 10)

	require.NoError(t, uc.Reserve(context.Background(), productID, 5, orderID.String()))
	require.NoError(t, uc.Release(context.Background(), productID, 5, orderID.String()))

	err := uc.Reserve(context.Background(), productID, 5, orderID.String())
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrFailedPrecondition))
}

func TestInventoryPostgres_Release_QuantityMismatch(t *testing.T) {
	uc, pool := newTestUsecase(t)
	productID := uuid.New()
	orderID := uuid.New()
	seedProduct(t, pool, productID, 10)

	require.NoError(t, uc.Reserve(context.Background(), productID, 5, orderID.String()))

	err := uc.Release(context.Background(), productID, 3, orderID.String())
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrConflict))
}

func TestInventoryPostgres_Release_InvalidQuantity(t *testing.T) {
	uc, _ := newTestUsecase(t)

	err := uc.Release(context.Background(), uuid.New(), 0, uuid.New().String())
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidArgument))
}

func TestInventoryPostgres_GetLedger(t *testing.T) {
	uc, pool := newTestUsecase(t)
	productID := uuid.New()
	orderID := uuid.New()
	seedProduct(t, pool, productID, 10)

	require.NoError(t, uc.Reserve(context.Background(), productID, 5, orderID.String()))
	require.NoError(t, uc.Release(context.Background(), productID, 5, orderID.String()))

	entries, err := uc.GetLedger(context.Background(), productID)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// Query orders by created_at DESC, so release (newer) comes first.
	assert.Equal(t, 5, entries[0].QuantityChange)
	assert.Equal(t, "LEDGER_OPERATION_RELEASE", entries[0].OperationType)

	assert.Equal(t, -5, entries[1].QuantityChange)
	assert.Equal(t, "LEDGER_OPERATION_RESERVE", entries[1].OperationType)
}

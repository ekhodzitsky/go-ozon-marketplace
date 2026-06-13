//go:build integration

package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	pgrepo "github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startPostgres(t *testing.T) string {
	t.Helper()
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
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return connStr
}

func runMigrations(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	migrationDir := "../../../migrations"
	entries, err := os.ReadDir(migrationDir)
	require.NoError(t, err)

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		content, err := os.ReadFile(filepath.Join(migrationDir, file))
		require.NoError(t, err)

		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, string(content))
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("migration %s failed: %v", file, err)
		}
		require.NoError(t, tx.Commit(ctx))
	}
	return pool
}

func newPayment(orderID, userID uuid.UUID, amount int64, status domain.Status) *domain.Payment {
	return &domain.Payment{
		ID:      uuid.New(),
		OrderID: orderID,
		UserID:  userID,
		Amount:  amount,
		Status:  status,
	}
}

func TestPaymentPostgres_CreateAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dsn := startPostgres(t)
	pool := runMigrations(t, dsn)
	repo := pgrepo.NewPaymentPostgres(pool)
	ctx := context.Background()

	orderID := uuid.New()
	userID := uuid.New()
	payment := newPayment(orderID, userID, 12345, domain.StatusPending)

	require.NoError(t, repo.Create(ctx, payment))

	got, err := repo.GetByID(ctx, payment.ID)
	require.NoError(t, err)
	assert.Equal(t, payment.ID, got.ID)
	assert.Equal(t, payment.OrderID, got.OrderID)
	assert.Equal(t, payment.UserID, got.UserID)
	assert.Equal(t, payment.Amount, got.Amount)
	assert.Equal(t, payment.Status, got.Status)
}

func TestPaymentPostgres_Create_DuplicateID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dsn := startPostgres(t)
	pool := runMigrations(t, dsn)
	repo := pgrepo.NewPaymentPostgres(pool)
	ctx := context.Background()

	payment := newPayment(uuid.New(), uuid.New(), 100, domain.StatusPending)
	require.NoError(t, repo.Create(ctx, payment))

	err := repo.Create(ctx, payment)
	require.Error(t, err)
}

func TestPaymentPostgres_CreateOrGet_Idempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dsn := startPostgres(t)
	pool := runMigrations(t, dsn)
	repo := pgrepo.NewPaymentPostgres(pool)
	ctx := context.Background()

	orderID := uuid.New()
	userID := uuid.New()
	p1 := newPayment(orderID, userID, 5000, domain.StatusPending)

	got1, err := repo.CreateOrGet(ctx, p1)
	require.NoError(t, err)
	assert.Equal(t, p1.ID, got1.ID)

	p2 := newPayment(orderID, userID, 9999, domain.StatusPending)
	got2, err := repo.CreateOrGet(ctx, p2)
	require.NoError(t, err)
	assert.Equal(t, got1.ID, got2.ID)
	assert.Equal(t, p1.Amount, got2.Amount)
}

func TestPaymentPostgres_UpdateStatusIf(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dsn := startPostgres(t)
	pool := runMigrations(t, dsn)
	repo := pgrepo.NewPaymentPostgres(pool)
	ctx := context.Background()

	payment := newPayment(uuid.New(), uuid.New(), 1000, domain.StatusPending)
	require.NoError(t, repo.Create(ctx, payment))

	updated, err := repo.UpdateStatusIf(ctx, payment.ID, domain.StatusSuccess, domain.StatusPending)
	require.NoError(t, err)
	assert.True(t, updated)

	got, err := repo.GetByID(ctx, payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSuccess, got.Status)

	updated, err = repo.UpdateStatusIf(ctx, payment.ID, domain.StatusSuccess, domain.StatusPending)
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestPaymentPostgres_UpdateStatusIf_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dsn := startPostgres(t)
	pool := runMigrations(t, dsn)
	repo := pgrepo.NewPaymentPostgres(pool)
	ctx := context.Background()

	updated, err := repo.UpdateStatusIf(ctx, uuid.New(), domain.StatusSuccess, domain.StatusPending)
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestPaymentPostgres_CreateRefund(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dsn := startPostgres(t)
	pool := runMigrations(t, dsn)
	repo := pgrepo.NewPaymentPostgres(pool)
	ctx := context.Background()

	payment := newPayment(uuid.New(), uuid.New(), 7500, domain.StatusSuccess)
	require.NoError(t, repo.Create(ctx, payment))

	refund := &domain.Refund{
		ID:             uuid.New(),
		PaymentID:      payment.ID,
		Amount:         payment.Amount,
		Reason:         "customer request",
		Status:         domain.StatusRefunded,
		IdempotencyKey: "idem-key-1",
		CreatedAt:      time.Now().UTC(),
	}
	require.NoError(t, repo.CreateRefund(ctx, refund))

	got, err := repo.GetRefund(ctx, refund.ID)
	require.NoError(t, err)
	assert.Equal(t, refund.ID, got.ID)
	assert.Equal(t, refund.PaymentID, got.PaymentID)
	assert.Equal(t, refund.Amount, got.Amount)
	assert.Equal(t, refund.IdempotencyKey, got.IdempotencyKey)

	list, err := repo.ListRefunds(ctx, payment.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, refund.ID, list[0].ID)
}

func TestPaymentPostgres_CreateRefund_DuplicateIdempotencyKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dsn := startPostgres(t)
	pool := runMigrations(t, dsn)
	repo := pgrepo.NewPaymentPostgres(pool)
	ctx := context.Background()

	payment := newPayment(uuid.New(), uuid.New(), 3000, domain.StatusSuccess)
	require.NoError(t, repo.Create(ctx, payment))

	refund1 := &domain.Refund{
		ID:             uuid.New(),
		PaymentID:      payment.ID,
		Amount:         payment.Amount,
		Status:         domain.StatusRefunded,
		IdempotencyKey: "shared-idem-key",
		CreatedAt:      time.Now().UTC(),
	}
	require.NoError(t, repo.CreateRefund(ctx, refund1))

	refund2 := &domain.Refund{
		ID:             uuid.New(),
		PaymentID:      payment.ID,
		Amount:         payment.Amount,
		Status:         domain.StatusRefunded,
		IdempotencyKey: "shared-idem-key",
		CreatedAt:      time.Now().UTC(),
	}
	err := repo.CreateRefund(ctx, refund2)
	require.Error(t, err)
}

func TestPaymentPostgres_ConcurrentCreateOrGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dsn := startPostgres(t)
	pool := runMigrations(t, dsn)
	ctx := context.Background()

	orderID := uuid.New()
	userID := uuid.New()

	const workers = 10
	var wg sync.WaitGroup
	results := make(chan uuid.UUID, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			repo := pgrepo.NewPaymentPostgres(pool)
			payment := newPayment(orderID, userID, int64(1000+i), domain.StatusPending)
			got, err := repo.CreateOrGet(ctx, payment)
			require.NoError(t, err)
			results <- got.ID
		}(i)
	}
	wg.Wait()
	close(results)

	var first uuid.UUID
	count := 0
	for id := range results {
		if count == 0 {
			first = id
		}
		assert.Equal(t, first, id)
		count++
	}
	assert.Equal(t, workers, count)
}

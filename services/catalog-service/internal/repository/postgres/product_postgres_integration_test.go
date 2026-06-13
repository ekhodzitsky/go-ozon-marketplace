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

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	catalogpostgres "github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("marketplace"),
		tcpostgres.WithUsername("ozon"),
		tcpostgres.WithPassword("ozonpass"),
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

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return dsn
}

func runMigrations(t *testing.T, dsn string) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	dir := "../../../migrations"
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

func newPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestProductPostgres_Integration_CreateGetUpdateDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)
	repo := catalogpostgres.NewProductPostgres(pool)

	ctx := context.Background()
	product := &domain.Product{
		ID:             uuid.New(),
		Name:           "Test Product",
		Description:    "Description",
		Price:          1234,
		Categories:     []string{"cat"},
		IdempotencyKey: uuid.NewString(),
		CreatedAt:      time.Now().UTC(),
	}

	id, err := repo.Create(ctx, product)
	require.NoError(t, err)
	assert.Equal(t, product.ID, id)

	got, err := repo.GetByID(ctx, product.ID)
	require.NoError(t, err)
	assert.Equal(t, product.Name, got.Name)
	assert.Equal(t, product.Price, got.Price)
	assert.Equal(t, product.Categories, got.Categories)

	got.Name = "Updated"
	got.Price = 5678
	got.Categories = []string{"new"}
	err = repo.Update(ctx, got)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, product.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, int64(5678), updated.Price)

	products, total, err := repo.List(ctx, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, products, 1)

	err = repo.Delete(ctx, product.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, product.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestProductPostgres_Integration_UniqueIdempotencyKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)
	repo := catalogpostgres.NewProductPostgres(pool)

	ctx := context.Background()
	key := uuid.NewString()
	product := &domain.Product{
		ID:             uuid.New(),
		Name:           "P1",
		Price:          100,
		IdempotencyKey: key,
		CreatedAt:      time.Now().UTC(),
	}

	id, err := repo.Create(ctx, product)
	require.NoError(t, err)

	duplicate := &domain.Product{
		ID:             uuid.New(),
		Name:           "P2",
		Price:          200,
		IdempotencyKey: key,
		CreatedAt:      time.Now().UTC(),
	}
	duplicateID, err := repo.Create(ctx, duplicate)
	assert.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrAlreadyExists)
	assert.Equal(t, id, duplicateID)
}

func TestProductPostgres_Integration_UpdateNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)
	repo := catalogpostgres.NewProductPostgres(pool)

	err := repo.Update(context.Background(), &domain.Product{ID: uuid.New(), Name: "Ghost", Price: 100})
	assert.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestProductPostgres_Integration_DeleteNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)
	repo := catalogpostgres.NewProductPostgres(pool)

	err := repo.Delete(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestProductPostgres_Integration_ConcurrentIdempotencyKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)

	ctx := context.Background()
	key := uuid.NewString()
	product := &domain.Product{
		ID:             uuid.New(),
		Name:           "Concurrent",
		Price:          100,
		IdempotencyKey: key,
		CreatedAt:      time.Now().UTC(),
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	ids := make(chan uuid.UUID, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repo := catalogpostgres.NewProductPostgres(pool)
			id, err := repo.Create(ctx, product)
			errs <- err
			ids <- id
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)

	var successCount, conflictCount int
	var successID uuid.UUID
	for err := range errs {
		if err == nil {
			successCount++
			successID = <-ids
		} else if assert.ErrorIs(t, err, apperrors.ErrAlreadyExists) {
			conflictCount++
			<-ids
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, conflictCount)

	repo := catalogpostgres.NewProductPostgres(pool)
	got, err := repo.GetByID(ctx, successID)
	require.NoError(t, err)
	assert.Equal(t, key, got.IdempotencyKey)
}

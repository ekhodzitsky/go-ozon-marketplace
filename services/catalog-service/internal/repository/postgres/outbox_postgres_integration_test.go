//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	catalogpostgres "github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newOutboxEvent(t *testing.T) *domain.OutboxEvent {
	t.Helper()
	return &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "product",
		AggregateID:   uuid.NewString(),
		EventType:     "ProductCreated",
		Payload:       []byte(`{"name":"test"}`),
		CreatedAt:     time.Now().UTC(),
		RetryCount:    0,
		NextRetryAt:   time.Now().UTC().Add(-time.Second),
	}
}

func TestOutboxPostgres_Integration_CreateAndGetUnprocessed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)
	repo := catalogpostgres.NewOutboxPostgres(pool)

	event := newOutboxEvent(t)
	require.NoError(t, repo.Create(context.Background(), event))

	events, err := repo.GetUnprocessed(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, event.ID, events[0].ID)
	assert.Equal(t, event.AggregateID, events[0].AggregateID)
	assert.Equal(t, 0, events[0].RetryCount)
}

func TestOutboxPostgres_Integration_MarkProcessed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)
	repo := catalogpostgres.NewOutboxPostgres(pool)

	event := newOutboxEvent(t)
	require.NoError(t, repo.Create(context.Background(), event))
	require.NoError(t, repo.MarkProcessed(context.Background(), event.ID))

	events, err := repo.GetUnprocessed(context.Background(), 10)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestOutboxPostgres_Integration_BatchMarkProcessed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)
	repo := catalogpostgres.NewOutboxPostgres(pool)

	e1 := newOutboxEvent(t)
	e2 := newOutboxEvent(t)
	require.NoError(t, repo.Create(context.Background(), e1))
	require.NoError(t, repo.Create(context.Background(), e2))

	require.NoError(t, repo.BatchMarkProcessed(context.Background(), []uuid.UUID{e1.ID, e2.ID}))

	events, err := repo.GetUnprocessed(context.Background(), 10)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestOutboxPostgres_Integration_IncrementRetryAndSetError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)
	repo := catalogpostgres.NewOutboxPostgres(pool)

	event := newOutboxEvent(t)
	require.NoError(t, repo.Create(context.Background(), event))

	nextRetry := time.Now().UTC().Add(time.Hour)
	require.NoError(t, repo.IncrementRetryAndSetError(context.Background(), event.ID, "transient", nextRetry))

	events, err := repo.GetUnprocessed(context.Background(), 10)
	require.NoError(t, err)
	assert.Empty(t, events)

	var retryCount int
	var lastError string
	query := `SELECT retry_count, last_error FROM outbox WHERE id=$1`
	require.NoError(t, pool.QueryRow(context.Background(), query, event.ID).Scan(&retryCount, &lastError))
	assert.Equal(t, 1, retryCount)
	assert.Equal(t, "transient", lastError)
}

func TestOutboxPostgres_Integration_MoveToDLQ(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)
	repo := catalogpostgres.NewOutboxPostgres(pool)

	event := newOutboxEvent(t)
	require.NoError(t, repo.Create(context.Background(), event))

	failedAt := time.Now().UTC()
	require.NoError(t, repo.MoveToDLQ(context.Background(), event, failedAt, "permanent"))

	events, err := repo.GetUnprocessed(context.Background(), 10)
	require.NoError(t, err)
	assert.Empty(t, events)

	var count int
	query := `SELECT COUNT(*) FROM outbox_dlq WHERE id=$1`
	require.NoError(t, pool.QueryRow(context.Background(), query, event.ID).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestOutboxPostgres_Integration_TransactionRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)
	repo := catalogpostgres.NewOutboxPostgres(pool)

	require.NoError(t, repo.Begin(context.Background()))
	event := newOutboxEvent(t)
	require.NoError(t, repo.Create(context.Background(), event))
	require.NoError(t, repo.Rollback(context.Background()))

	events, err := repo.GetUnprocessed(context.Background(), 10)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestOutboxPostgres_Integration_TransactionCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dsn := startPostgres(t)
	runMigrations(t, dsn)
	pool := newPool(t, dsn)
	repo := catalogpostgres.NewOutboxPostgres(pool)

	require.NoError(t, repo.Begin(context.Background()))
	event := newOutboxEvent(t)
	require.NoError(t, repo.Create(context.Background(), event))
	require.NoError(t, repo.Commit(context.Background()))

	events, err := repo.GetUnprocessed(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, event.ID, events[0].ID)
}

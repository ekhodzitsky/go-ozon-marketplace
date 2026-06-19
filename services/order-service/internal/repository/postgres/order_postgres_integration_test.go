//go:build integration

package postgres_test

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
	pkgpostgres "github.com/ekhodzitsky/go-ozon-marketplace/pkg/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/txmanager"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork"
	postgresuow "github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork/postgres"
	"github.com/jackc/pgx/v5"

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
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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

	migrationDir := filepath.Join("..", "..", "..", "migrations")
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
			t.Fatalf("failed to execute migration %s: %v", file, err)
		}
		require.NoError(t, tx.Commit(ctx))
	}
}

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := startPostgres(t)
	runMigrations(t, dsn)

	ctx := context.Background()
	pool, err := pkgpostgres.NewPool(ctx, dsn)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func newOrder(userID uuid.UUID, items []domain.OrderItem) *domain.Order {
	now := time.Now().UTC()
	orderID := uuid.New()
	var total int64
	for i := range items {
		items[i].ID = uuid.New()
		items[i].OrderID = orderID
		total += items[i].Price * int64(items[i].Quantity)
	}
	return &domain.Order{
		ID:          orderID,
		UserID:      userID,
		Items:       items,
		TotalAmount: total,
		Status:      domain.OrderStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func newOrderItem(productID uuid.UUID, quantity int, price int64) domain.OrderItem {
	return domain.OrderItem{
		ProductID: productID,
		Quantity:  quantity,
		Price:     price,
	}
}

func TestOrderPostgres_CreateAndGet(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOrderPostgres(pool)
	ctx := context.Background()

	userID := uuid.New()
	productID := uuid.New()
	order := newOrder(userID, []domain.OrderItem{newOrderItem(productID, 2, 500)})

	require.NoError(t, repo.Create(ctx, order))

	got, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, order.ID, got.ID)
	assert.Equal(t, userID, got.UserID)
	assert.Equal(t, order.TotalAmount, got.TotalAmount)
	assert.Equal(t, domain.OrderStatusPending, got.Status)
	require.Len(t, got.Items, 1)
	assert.Equal(t, productID, got.Items[0].ProductID)
	assert.Equal(t, 2, got.Items[0].Quantity)
	assert.Equal(t, int64(500), got.Items[0].Price)
}

func TestOrderPostgres_GetByID_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOrderPostgres(pool)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestOrderPostgres_UpdateStatus(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOrderPostgres(pool)
	ctx := context.Background()

	order := newOrder(uuid.New(), []domain.OrderItem{newOrderItem(uuid.New(), 1, 100)})
	require.NoError(t, repo.Create(ctx, order))

	require.NoError(t, repo.UpdateStatus(ctx, order.ID, domain.OrderStatusAwaitingPayment))

	got, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusAwaitingPayment, got.Status)
}

func TestOrderPostgres_UpdateStatus_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOrderPostgres(pool)
	ctx := context.Background()

	err := repo.UpdateStatus(ctx, uuid.New(), domain.OrderStatusCancelled)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestOrderPostgres_ListByUser(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOrderPostgres(pool)
	ctx := context.Background()

	userID := uuid.New()
	order := newOrder(userID, []domain.OrderItem{newOrderItem(uuid.New(), 1, 100)})
	require.NoError(t, repo.Create(ctx, order))

	orders, total, err := repo.ListByUser(ctx, userID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, orders, 1)
	assert.Equal(t, order.ID, orders[0].ID)
}

func TestOrderPostgres_ListByUser_Pagination(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOrderPostgres(pool)
	ctx := context.Background()

	userID := uuid.New()
	for i := 0; i < 5; i++ {
		order := newOrder(userID, []domain.OrderItem{newOrderItem(uuid.New(), 1, int64(100+i))})
		require.NoError(t, repo.Create(ctx, order))
	}

	orders, total, err := repo.ListByUser(ctx, userID, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, orders, 2)

	orders, total, err = repo.ListByUser(ctx, userID, 3, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, orders, 1)
}

func TestOrderPostgres_Create_DuplicateID(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOrderPostgres(pool)
	ctx := context.Background()

	order := newOrder(uuid.New(), []domain.OrderItem{newOrderItem(uuid.New(), 1, 100)})
	require.NoError(t, repo.Create(ctx, order))

	err := repo.Create(ctx, order)
	require.Error(t, err)
}

func TestOrderPostgres_ConcurrentCreate(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOrderPostgres(pool)
	ctx := context.Background()

	userID := uuid.New()
	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			order := newOrder(userID, []domain.OrderItem{newOrderItem(uuid.New(), 1, 100)})
			if err := repo.Create(ctx, order); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	var errCount int
	for err := range errs {
		if err != nil {
			errCount++
		}
	}
	assert.Equal(t, 0, errCount)

	orders, total, err := repo.ListByUser(ctx, userID, 1, 100)
	require.NoError(t, err)
	assert.Equal(t, 10, total)
	assert.Len(t, orders, 10)
}

func TestOutboxPostgres_CreateAndGetUnprocessed(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOutboxPostgres(pool)
	ctx := context.Background()

	event := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "order",
		AggregateID:   uuid.New().String(),
		EventType:     "OrderCreated",
		Payload:       []byte(`{"test": true}`),
		CreatedAt:     time.Now().UTC(),
		RetryCount:    0,
		NextRetryAt:   time.Now().UTC().Add(-time.Hour),
	}

	require.NoError(t, repo.Create(ctx, event))

	events, err := repo.GetUnprocessed(ctx, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, event.ID, events[0].ID)
}

func TestOutboxPostgres_MarkProcessed(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOutboxPostgres(pool)
	ctx := context.Background()

	event := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "order",
		AggregateID:   uuid.New().String(),
		EventType:     "OrderCreated",
		Payload:       []byte(`{}`),
		CreatedAt:     time.Now().UTC(),
		NextRetryAt:   time.Now().UTC().Add(-time.Hour),
	}
	require.NoError(t, repo.Create(ctx, event))

	require.NoError(t, repo.MarkProcessed(ctx, event.ID))

	events, err := repo.GetUnprocessed(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestOutboxPostgres_BatchMarkProcessed(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOutboxPostgres(pool)
	ctx := context.Background()

	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "order",
			AggregateID:   uuid.New().String(),
			EventType:     "OrderCreated",
			Payload:       []byte(`{}`),
			CreatedAt:     time.Now().UTC(),
			NextRetryAt:   time.Now().UTC().Add(-time.Hour),
		}
		require.NoError(t, repo.Create(ctx, event))
		ids = append(ids, event.ID)
	}

	require.NoError(t, repo.BatchMarkProcessed(ctx, ids))

	events, err := repo.GetUnprocessed(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestOutboxPostgres_IncrementRetryAndSetError(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOutboxPostgres(pool)
	ctx := context.Background()

	event := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "order",
		AggregateID:   uuid.New().String(),
		EventType:     "OrderCreated",
		Payload:       []byte(`{}`),
		CreatedAt:     time.Now().UTC(),
		NextRetryAt:   time.Now().UTC().Add(-time.Hour),
	}
	require.NoError(t, repo.Create(ctx, event))

	next := time.Now().UTC()
	require.NoError(t, repo.IncrementRetryAndSetError(ctx, event.ID, "boom", next))

	events, err := repo.GetUnprocessed(ctx, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, 1, events[0].RetryCount)
	assert.NotNil(t, events[0].LastError)
	assert.Equal(t, "boom", *events[0].LastError)
}

func TestOutboxPostgres_MoveToDLQ(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOutboxPostgres(pool)
	ctx := context.Background()

	event := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "order",
		AggregateID:   uuid.New().String(),
		EventType:     "OrderCreated",
		Payload:       []byte(`{}`),
		CreatedAt:     time.Now().UTC(),
		NextRetryAt:   time.Now().UTC().Add(-time.Hour),
		RetryCount:    2,
	}
	require.NoError(t, repo.Create(ctx, event))

	require.NoError(t, repo.MoveToDLQ(ctx, event, time.Now().UTC(), "permanent failure"))

	_, err := repo.GetUnprocessed(ctx, 10)
	require.NoError(t, err)

	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_dlq WHERE id=$1", event.ID).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestOutboxPostgres_Transaction(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewOutboxPostgres(pool)
	ctx := context.Background()

	require.NoError(t, repo.Begin(ctx))
	event := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "order",
		AggregateID:   uuid.New().String(),
		EventType:     "OrderCreated",
		Payload:       []byte(`{}`),
		CreatedAt:     time.Now().UTC(),
		NextRetryAt:   time.Now().UTC().Add(-time.Hour),
	}
	require.NoError(t, repo.Create(ctx, event))
	require.NoError(t, repo.Rollback(ctx))

	// Use a fresh repository because Rollback/Commit leave the original
	// repository bound to the closed transaction.
	freshRepo := postgres.NewOutboxPostgres(pool)
	events, err := freshRepo.GetUnprocessed(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, events, 0)
}

func createOrderForSaga(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	orderRepo := postgres.NewOrderPostgres(pool)
	order := newOrder(uuid.New(), []domain.OrderItem{newOrderItem(uuid.New(), 1, 100)})
	require.NoError(t, orderRepo.Create(ctx, order))
	return order.ID
}

func TestSagaPostgres_CreateAndGet(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSagaPostgres(pool)
	ctx := context.Background()

	orderID := createOrderForSaga(t, ctx, pool)
	saga := &saga.Saga{
		ID:          uuid.New(),
		OrderID:     orderID,
		Status:      saga.SagaStatusPending,
		CurrentStep: "init",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, saga))

	got, err := repo.GetByOrderID(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, saga.ID, got.ID)
	assert.Equal(t, saga.SagaStatusPending, got.Status)
	assert.Equal(t, "init", got.CurrentStep)
}

func TestSagaPostgres_GetByOrderID_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSagaPostgres(pool)
	ctx := context.Background()

	_, err := repo.GetByOrderID(ctx, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestSagaPostgres_UpdateStatus(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSagaPostgres(pool)
	ctx := context.Background()

	orderID := createOrderForSaga(t, ctx, pool)
	saga := &saga.Saga{
		ID:        uuid.New(),
		OrderID:   orderID,
		Status:    saga.SagaStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, saga))

	require.NoError(t, repo.UpdateStatus(ctx, orderID, saga.SagaStatusReserving, "reserve", ""))

	got, err := repo.GetByOrderID(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, saga.SagaStatusReserving, got.Status)
	assert.Equal(t, "reserve", got.CurrentStep)
}

func TestSagaPostgres_UpdateStatus_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSagaPostgres(pool)
	ctx := context.Background()

	err := repo.UpdateStatus(ctx, uuid.New(), saga.SagaStatusReserving, "reserve", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestSagaPostgres_Save(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSagaPostgres(pool)
	ctx := context.Background()

	orderID := createOrderForSaga(t, ctx, pool)
	saga := &saga.Saga{
		ID:            uuid.New(),
		OrderID:       orderID,
		Status:        saga.SagaStatusPending,
		ReservedItems: []saga.SagaReservedItem{{ProductID: uuid.New().String(), Quantity: 2}},
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, saga))

	saga.Status = saga.SagaStatusReserved
	saga.PaymentID = "pay-123"
	require.NoError(t, repo.Save(ctx, saga))

	got, err := repo.GetByOrderID(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, saga.SagaStatusReserved, got.Status)
	assert.Equal(t, "pay-123", got.PaymentID)
	require.Len(t, got.ReservedItems, 1)
	assert.Equal(t, int32(2), got.ReservedItems[0].Quantity)
}

func TestSagaPostgres_Save_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSagaPostgres(pool)
	ctx := context.Background()

	saga := &saga.Saga{
		ID:        uuid.New(),
		OrderID:   uuid.New(),
		Status:    saga.SagaStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := repo.Save(ctx, saga)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestSagaPostgres_ListIncomplete(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSagaPostgres(pool)
	ctx := context.Background()

	incompleteOrderID := createOrderForSaga(t, ctx, pool)
	incomplete := &saga.Saga{
		ID:        uuid.New(),
		OrderID:   incompleteOrderID,
		Status:    saga.SagaStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, incomplete))

	completeOrderID := createOrderForSaga(t, ctx, pool)
	complete := &saga.Saga{
		ID:        uuid.New(),
		OrderID:   completeOrderID,
		Status:    saga.SagaStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, complete))
	require.NoError(t, repo.UpdateStatus(ctx, completeOrderID, saga.SagaStatusConfirmed, "confirmed", ""))

	sagas, err := repo.ListIncomplete(ctx, 10)
	require.NoError(t, err)
	require.Len(t, sagas, 1)
	assert.Equal(t, incomplete.ID, sagas[0].ID)
}

func TestUnitOfWork_CommitAndRollback(t *testing.T) {
	pool := newTestPool(t)
	txm := txmanager.New(pool, func(tx pgx.Tx) unitofwork.UnitOfWork {
		return postgresuow.NewUnitOfWork(tx)
	})
	ctx := context.Background()

	var createdOrderID uuid.UUID
	err := txm.Run(ctx, func(uow unitofwork.UnitOfWork) error {
		orderRepo := uow.OrderRepo()
		outboxRepo := uow.OutboxRepo()
		require.NotNil(t, orderRepo)
		require.NotNil(t, outboxRepo)

		order := newOrder(uuid.New(), []domain.OrderItem{newOrderItem(uuid.New(), 1, 100)})
		if err := orderRepo.Create(ctx, order); err != nil {
			return err
		}
		createdOrderID = order.ID
		return errors.New("force rollback")
	})
	require.Error(t, err)

	standalone := postgres.NewOrderPostgres(pool)
	_, err = standalone.GetByID(ctx, createdOrderID)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestUnitOfWork_Commit(t *testing.T) {
	pool := newTestPool(t)
	txm := txmanager.New(pool, func(tx pgx.Tx) unitofwork.UnitOfWork {
		return postgresuow.NewUnitOfWork(tx)
	})
	ctx := context.Background()

	var createdOrderID uuid.UUID
	err := txm.Run(ctx, func(uow unitofwork.UnitOfWork) error {
		order := newOrder(uuid.New(), []domain.OrderItem{newOrderItem(uuid.New(), 1, 100)})
		if err := uow.OrderRepo().Create(ctx, order); err != nil {
			return err
		}

		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "order",
			AggregateID:   order.ID.String(),
			EventType:     "OrderCreated",
			Payload:       []byte(`{}`),
			CreatedAt:     time.Now().UTC(),
			NextRetryAt:   time.Now().UTC().Add(-time.Hour),
		}
		if err := uow.OutboxRepo().Create(ctx, event); err != nil {
			return err
		}
		createdOrderID = order.ID
		return nil
	})
	require.NoError(t, err)

	standalone := postgres.NewOrderPostgres(pool)
	got, err := standalone.GetByID(ctx, createdOrderID)
	require.NoError(t, err)
	assert.Equal(t, createdOrderID, got.ID)
}

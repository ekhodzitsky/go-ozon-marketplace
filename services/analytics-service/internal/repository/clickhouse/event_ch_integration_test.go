//go:build integration

package clickhouse_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/repository/clickhouse"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcch "github.com/testcontainers/testcontainers-go/modules/clickhouse"
)

func serviceRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

var (
	sharedRepo    *clickhouse.EventRepo
	sharedOnce    sync.Once
	sharedCleanup func()
)

func setupSharedClickHouse(t *testing.T) *clickhouse.EventRepo {
	t.Helper()

	sharedOnce.Do(func() {
		root := serviceRoot()
		require.NoError(t, os.Chdir(root))

		ctx := context.Background()
		container, err := tcch.Run(ctx, "clickhouse/clickhouse-server:24.3")
		require.NoError(t, err)

		addr, err := container.ConnectionHost(ctx)
		require.NoError(t, err)

		sharedRepo, err = clickhouse.NewEventRepo(addr, container.User, container.Password, 10*time.Second, 10*time.Second, migrations.FS)
		require.NoError(t, err)

		sharedCleanup = func() {
			sharedRepo.Close()
			_ = container.Terminate(ctx)
		}
	})

	return sharedRepo
}

func TestMain(m *testing.M) {
	code := m.Run()
	if sharedCleanup != nil {
		sharedCleanup()
	}
	os.Exit(code)
}

func TestEventRepo_BatchInsertAndGetDailyRevenue(t *testing.T) {
	repo := setupSharedClickHouse(t)

	ctx := context.Background()
	now := time.Now().UTC()
	today := now.Format("2006-01-02")

	events := []domain.Event{
		{
			EventType:      domain.EventTypePaymentSuccess,
			AggregateID:    "order-1",
			Payload:        "{}",
			Amount:         100.50,
			Currency:       "RUB",
			OccurredAt:     now,
			CreatedAt:      now,
			AggregationKey: "payment-1",
		},
		{
			EventType:      domain.EventTypePaymentSuccess,
			AggregateID:    "order-2",
			Payload:        "{}",
			Amount:         49.50,
			Currency:       "RUB",
			OccurredAt:     now,
			CreatedAt:      now,
			AggregationKey: "payment-2",
		},
		{
			EventType:      domain.EventTypeOrderCreated,
			AggregateID:    "order-3",
			Payload:        "{}",
			Amount:         0,
			Currency:       "",
			OccurredAt:     now,
			CreatedAt:      now,
			AggregationKey: "order-3",
		},
	}

	require.NoError(t, repo.BatchInsert(ctx, events))

	revenue, err := repo.GetDailyRevenue(ctx, today)
	require.NoError(t, err)
	assert.InDelta(t, 150.0, revenue, 0.001)
}

func TestEventRepo_TrackABTestEvent(t *testing.T) {
	repo := setupSharedClickHouse(t)

	ctx := context.Background()
	event := domain.ABTestEvent{
		EventID:      uuid.Nil,
		Experiment:   "exp-1",
		Variation:    "var-a",
		UserID:       uuid.Must(uuid.NewV7()),
		Conversion:   true,
		RevenueMinor: 1000,
		CreatedAt:    time.Time{},
	}

	require.NoError(t, repo.TrackABTestEvent(ctx, event))
}

func TestEventRepo_GetDailyRevenue_NoRows(t *testing.T) {
	repo := setupSharedClickHouse(t)

	revenue, err := repo.GetDailyRevenue(context.Background(), "2099-01-01")
	require.NoError(t, err)
	assert.InDelta(t, 0.0, revenue, 0.001)
}

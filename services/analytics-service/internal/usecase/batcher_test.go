package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestEventBatcher_FlushEmptyBuffer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())

	// No BatchInsert expected because the buffer is empty.
	require.NoError(t, uc.Flush(context.Background()))
}

func TestEventBatcher_PeriodicFlush(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())
	defer func() { _ = uc.Flush(context.Background()) }()

	repo.EXPECT().BatchInsert(gomock.Any(), gomock.Len(1)).Return(nil).Times(1)

	require.NoError(t, uc.TrackEvent(context.Background(), domain.EventTypeOrderCreated, "order-1", "{}", "", 1.0))

	require.Eventually(t, func() bool {
		return ctrl.Satisfied()
	}, 2*time.Second, 10*time.Millisecond)
}

func TestEventBatcher_FlushWhenBufferFull(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())
	defer func() { _ = uc.Flush(context.Background()) }()

	repo.EXPECT().BatchInsert(gomock.Any(), gomock.Len(100)).Return(nil).Times(1)

	ctx := context.Background()
	for i := 0; i < 100; i++ {
		require.NoError(t, uc.TrackEvent(ctx, domain.EventTypeOrderCreated, fmt.Sprintf("order-%d", i), "{}", "", float64(i)))
	}

	require.Eventually(t, func() bool {
		return ctrl.Satisfied()
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestEventBatcher_StopFlushesRemaining(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())

	repo.EXPECT().BatchInsert(gomock.Any(), gomock.Len(2)).Return(nil).Times(1)

	ctx := context.Background()
	require.NoError(t, uc.TrackEvent(ctx, domain.EventTypeOrderCreated, "order-1", "{}", "agg-1", 1.0))
	require.NoError(t, uc.TrackEvent(ctx, domain.EventTypeOrderConfirmed, "order-2", "{}", "agg-2", 2.0))
	require.NoError(t, uc.Flush(ctx))
}

func TestEventBatcher_FlushError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())

	repo.EXPECT().BatchInsert(gomock.Any(), gomock.Len(100)).Return(errors.New("batch failed")).Times(1)

	ctx := context.Background()
	var flushErr error
	for i := 0; i < 100; i++ {
		err := uc.TrackEvent(ctx, domain.EventTypeOrderCreated, fmt.Sprintf("order-%d", i), "{}", "", float64(i))
		// The error is returned only for the event that triggers the flush.
		if err != nil {
			flushErr = err
			break
		}
	}
	require.Error(t, flushErr)
	assert.Contains(t, flushErr.Error(), "batch failed")

	// The failed batch should remain in the buffer and be retried on the next flush.
	repo.EXPECT().BatchInsert(gomock.Any(), gomock.Len(100)).Return(nil).Times(1)
	require.NoError(t, uc.Flush(ctx))
}

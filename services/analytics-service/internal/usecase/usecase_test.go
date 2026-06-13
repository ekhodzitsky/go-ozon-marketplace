package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestAnalyticsUsecase_TrackEvent_DeduplicatesByAggregationKey(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())

	repo.EXPECT().BatchInsert(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	ctx := context.Background()
	require.NoError(t, uc.TrackEvent(ctx, domain.EventTypeOrderCreated, "order-1", "{}", "agg-1", 10.0))
	require.NoError(t, uc.TrackEvent(ctx, domain.EventTypeOrderCreated, "order-1", "{}", "agg-1", 20.0))

	require.NoError(t, uc.Flush(ctx))
}

func TestAnalyticsUsecase_TrackEvent_FlushesWhenBufferFull(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	// Use a tiny batcher so the first event triggers an immediate flush.
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())
	defer func() { _ = uc.Flush(context.Background()) }()

	repo.EXPECT().BatchInsert(gomock.Any(), gomock.Len(1)).Return(nil).Times(1)

	ctx := context.Background()
	require.NoError(t, uc.TrackEvent(ctx, domain.EventTypePaymentSuccess, "order-2", "{}", "", 15.0))
}

func TestAnalyticsUsecase_GetDailyRevenue(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())

	repo.EXPECT().GetDailyRevenue(gomock.Any(), "2024-01-01").Return(1234.56, nil)

	revenue, err := uc.GetDailyRevenue(context.Background(), "2024-01-01")
	require.NoError(t, err)
	assert.InDelta(t, 1234.56, revenue, 0.001)
}

func TestAnalyticsUsecase_GetDailyRevenue_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())

	repo.EXPECT().GetDailyRevenue(gomock.Any(), "2024-01-01").Return(0.0, errors.New("db down"))

	_, err := uc.GetDailyRevenue(context.Background(), "2024-01-01")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestAnalyticsUsecase_TrackABTestEvent(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())

	event := domain.ABTestEvent{
		EventID:      uuid.Must(uuid.NewV7()),
		Experiment:   "exp-1",
		Variation:    "var-a",
		UserID:       uuid.Must(uuid.NewV7()),
		Conversion:   true,
		RevenueMinor: 100,
		CreatedAt:    time.Now().UTC(),
	}

	repo.EXPECT().TrackABTestEvent(gomock.Any(), event).Return(nil)

	require.NoError(t, uc.TrackABTestEvent(context.Background(), event))
}

func TestAnalyticsUsecase_TrackABTestEvent_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())

	repo.EXPECT().TrackABTestEvent(gomock.Any(), gomock.Any()).Return(errors.New("insert failed"))

	err := uc.TrackABTestEvent(context.Background(), domain.ABTestEvent{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert failed")
}

func TestAnalyticsUsecase_Flush_ForwardError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEventRepository(ctrl)
	uc := usecase.NewAnalyticsUsecase(repo, 5*time.Second, 3*time.Second, zap.NewNop())

	repo.EXPECT().BatchInsert(gomock.Any(), gomock.Len(1)).Return(errors.New("flush failed")).Times(1)

	ctx := context.Background()
	require.NoError(t, uc.TrackEvent(ctx, domain.EventTypeOrderCreated, "order-3", "{}", "agg-3", 3.0))

	err := uc.Flush(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flush failed")
}

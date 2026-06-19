package executor_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/executor"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestRetryExecutor(log *zap.Logger) *executor.RetryExecutor {
	return executor.NewRetryExecutor(executor.RetryConfig{
		MaxRetries:  3,
		BaseDelay:   10 * time.Millisecond,
		CallTimeout: 500 * time.Millisecond,
	}, log)
}

func TestRetryExecutor_Execute_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	step := mocks.NewMockStep(ctrl)
	saga := &domain.Saga{ID: uuid.New()}
	order := &domain.Order{ID: uuid.New()}

	step.EXPECT().Execute(gomock.Any(), saga, order, "key").Return(nil)

	exec := newTestRetryExecutor(zap.NewNop())
	err := exec.Execute(context.Background(), step, saga, order, "key")
	require.NoError(t, err)
}

func TestRetryExecutor_Execute_RetriesThenSucceeds(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	step := mocks.NewMockStep(ctrl)
	saga := &domain.Saga{ID: uuid.New()}
	order := &domain.Order{ID: uuid.New()}
	transient := errors.New("transient")

	step.EXPECT().Execute(gomock.Any(), saga, order, "key").Return(transient)
	step.EXPECT().Execute(gomock.Any(), saga, order, "key").Return(nil)

	exec := newTestRetryExecutor(zap.NewNop())
	err := exec.Execute(context.Background(), step, saga, order, "key")
	require.NoError(t, err)
}

func TestRetryExecutor_Execute_ExhaustsRetries(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	step := mocks.NewMockStep(ctrl)
	saga := &domain.Saga{ID: uuid.New()}
	order := &domain.Order{ID: uuid.New()}
	want := errors.New("persistent")

	step.EXPECT().Execute(gomock.Any(), saga, order, "key").Return(want).Times(3)

	exec := newTestRetryExecutor(zap.NewNop())
	err := exec.Execute(context.Background(), step, saga, order, "key")
	assert.ErrorIs(t, err, want)
}

func TestRetryExecutor_Execute_ContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	step := mocks.NewMockStep(ctrl)
	saga := &domain.Saga{ID: uuid.New()}
	order := &domain.Order{ID: uuid.New()}

	step.EXPECT().Execute(gomock.Any(), saga, order, "key").Return(errors.New("transient"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	exec := newTestRetryExecutor(zap.NewNop())
	err := exec.Execute(ctx, step, saga, order, "key")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRetryExecutor_Compensate_Delegates(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	step := mocks.NewMockStep(ctrl)
	saga := &domain.Saga{ID: uuid.New()}
	order := &domain.Order{ID: uuid.New()}

	step.EXPECT().Compensate(gomock.Any(), saga, order, "key").Return(nil)

	exec := newTestRetryExecutor(zap.NewNop())
	err := exec.Compensate(context.Background(), step, saga, order, "key")
	require.NoError(t, err)
}

package steps_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestStartStep_Execute_AdvancesSaga(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mocks.NewMockOrderRepository(ctrl)
	step := steps.NewStartStep(orderRepo, zap.NewNop())
	order := &domain.Order{ID: uuid.New()}
	saga := &domain.Saga{Status: domain.SagaStatusPending}

	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusAwaitingPayment).Return(nil)

	err := step.Execute(context.Background(), saga, order, "key")
	require.NoError(t, err)
	assert.Equal(t, domain.SagaStatusReserving, saga.Status)
	assert.Equal(t, "reserve", saga.CurrentStep)
}

func TestStartStep_Execute_KeepsSagaPendingOnError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mocks.NewMockOrderRepository(ctrl)
	step := steps.NewStartStep(orderRepo, zap.NewNop())
	order := &domain.Order{ID: uuid.New()}
	saga := &domain.Saga{Status: domain.SagaStatusPending}
	want := errors.New("db error")

	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusAwaitingPayment).Return(want)

	err := step.Execute(context.Background(), saga, order, "key")
	assert.ErrorIs(t, err, want)
	assert.Equal(t, domain.SagaStatusPending, saga.Status)
}

func TestStartStep_Compensate_IsNoOp(t *testing.T) {
	t.Parallel()

	step := steps.NewStartStep(nil, zap.NewNop())
	err := step.Compensate(context.Background(), &domain.Saga{}, &domain.Order{}, "key")
	require.NoError(t, err)
}

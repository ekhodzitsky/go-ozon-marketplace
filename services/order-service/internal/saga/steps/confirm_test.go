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
)

func TestConfirmOrderStep_Name(t *testing.T) {
	t.Parallel()

	step := steps.NewConfirmOrderStep(nil)
	assert.Equal(t, "confirm", step.Name())
}

func TestConfirmOrderStep_Prepare(t *testing.T) {
	t.Parallel()

	step := steps.NewConfirmOrderStep(nil)
	saga := &domain.Saga{Status: domain.SagaStatusPaid}

	prepared := step.Prepare(saga)
	assert.True(t, prepared)
	assert.Equal(t, domain.SagaStatusConfirming, saga.Status)
	assert.Equal(t, "confirm", saga.CurrentStep)
}

func TestConfirmOrderStep_Prepare_AlreadyConfirming(t *testing.T) {
	t.Parallel()

	step := steps.NewConfirmOrderStep(nil)
	saga := &domain.Saga{Status: domain.SagaStatusConfirming}

	prepared := step.Prepare(saga)
	assert.False(t, prepared)
	assert.Equal(t, domain.SagaStatusConfirming, saga.Status)
}

func TestConfirmOrderStep_Execute_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mocks.NewMockOrderRepository(ctrl)
	step := steps.NewConfirmOrderStep(orderRepo)
	order := &domain.Order{ID: uuid.New()}
	saga := &domain.Saga{Status: domain.SagaStatusConfirming}

	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusPaid).Return(nil)

	err := step.Execute(context.Background(), saga, order, "key")
	require.NoError(t, err)
	assert.Equal(t, domain.SagaStatusConfirmed, saga.Status)
	assert.Equal(t, "confirmed", saga.CurrentStep)
}

func TestConfirmOrderStep_Execute_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mocks.NewMockOrderRepository(ctrl)
	step := steps.NewConfirmOrderStep(orderRepo)
	order := &domain.Order{ID: uuid.New()}
	saga := &domain.Saga{Status: domain.SagaStatusConfirming}
	want := errors.New("db unavailable")

	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusPaid).Return(want)

	err := step.Execute(context.Background(), saga, order, "key")
	assert.ErrorIs(t, err, want)
}

func TestConfirmOrderStep_Compensate_IsNoOp(t *testing.T) {
	t.Parallel()

	step := steps.NewConfirmOrderStep(nil)
	err := step.Compensate(context.Background(), &domain.Saga{}, &domain.Order{}, "key")
	require.NoError(t, err)
}

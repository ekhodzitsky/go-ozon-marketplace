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

func TestProcessPaymentStep_Name(t *testing.T) {
	t.Parallel()

	step := steps.NewProcessPaymentStep(nil)
	assert.Equal(t, "payment", step.Name())
}

func TestProcessPaymentStep_Prepare(t *testing.T) {
	t.Parallel()

	step := steps.NewProcessPaymentStep(nil)
	saga := &domain.Saga{Status: domain.SagaStatusReserved}

	prepared := step.Prepare(saga)
	assert.True(t, prepared)
	assert.Equal(t, domain.SagaStatusPaying, saga.Status)
	assert.Equal(t, "payment", saga.CurrentStep)
}

func TestProcessPaymentStep_Prepare_AlreadyPaying(t *testing.T) {
	t.Parallel()

	step := steps.NewProcessPaymentStep(nil)
	saga := &domain.Saga{Status: domain.SagaStatusPaying}

	prepared := step.Prepare(saga)
	assert.False(t, prepared)
	assert.Equal(t, domain.SagaStatusPaying, saga.Status)
}

func TestProcessPaymentStep_Execute_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPaymentClient(ctrl)
	step := steps.NewProcessPaymentStep(client)
	orderID := uuid.New()
	order := &domain.Order{ID: orderID, TotalAmount: 999}
	saga := &domain.Saga{Status: domain.SagaStatusPaying}

	client.EXPECT().ProcessPayment(gomock.Any(), orderID.String(), int64(999), gomock.Any()).Return("pay-123", nil)

	err := step.Execute(context.Background(), saga, order, "key")
	require.NoError(t, err)
	assert.Equal(t, "pay-123", saga.PaymentID)
	assert.Equal(t, domain.SagaStatusPaid, saga.Status)
	assert.Equal(t, "paid", saga.CurrentStep)
}

func TestProcessPaymentStep_Execute_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPaymentClient(ctrl)
	step := steps.NewProcessPaymentStep(client)
	order := &domain.Order{ID: uuid.New(), TotalAmount: 100}
	saga := &domain.Saga{Status: domain.SagaStatusPaying}
	want := errors.New("declined")

	client.EXPECT().ProcessPayment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", want)

	err := step.Execute(context.Background(), saga, order, "key")
	assert.ErrorIs(t, err, want)
	assert.Empty(t, saga.PaymentID)
}

func TestProcessPaymentStep_Compensate_RefundsWhenPaymentIDPresent(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPaymentClient(ctrl)
	step := steps.NewProcessPaymentStep(client)
	saga := &domain.Saga{PaymentID: "pay-456"}

	client.EXPECT().Refund(gomock.Any(), "pay-456", gomock.Any()).Return(nil)

	err := step.Compensate(context.Background(), saga, &domain.Order{}, "key")
	require.NoError(t, err)
}

func TestProcessPaymentStep_Compensate_SkipsWhenNoPaymentID(t *testing.T) {
	t.Parallel()

	step := steps.NewProcessPaymentStep(nil)
	saga := &domain.Saga{PaymentID: ""}

	err := step.Compensate(context.Background(), saga, &domain.Order{}, "key")
	require.NoError(t, err)
}

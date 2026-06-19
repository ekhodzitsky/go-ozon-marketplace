package saga_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
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
	step := saga.NewStartStep(orderRepo, zap.NewNop())
	order := &domain.Order{ID: uuid.New()}
	s := &saga.Saga{Status: saga.SagaStatusPending}

	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusAwaitingPayment).Return(nil)

	err := step.Execute(context.Background(), s, order, "key")
	require.NoError(t, err)
	assert.Equal(t, saga.SagaStatusReserving, s.Status)
	assert.Equal(t, "reserve", s.CurrentStep)
}

func TestStartStep_Execute_KeepsSagaPendingOnError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mocks.NewMockOrderRepository(ctrl)
	step := saga.NewStartStep(orderRepo, zap.NewNop())
	order := &domain.Order{ID: uuid.New()}
	s := &saga.Saga{Status: saga.SagaStatusPending}
	want := errors.New("db error")

	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusAwaitingPayment).Return(want)

	err := step.Execute(context.Background(), s, order, "key")
	assert.ErrorIs(t, err, want)
	assert.Equal(t, saga.SagaStatusPending, s.Status)
}

func TestStartStep_Compensate_IsNoOp(t *testing.T) {
	t.Parallel()

	step := saga.NewStartStep(nil, zap.NewNop())
	err := step.Compensate(context.Background(), &saga.Saga{}, &domain.Order{}, "key")
	require.NoError(t, err)
}

func TestReserveInventoryStep_Name(t *testing.T) {
	t.Parallel()

	step := saga.NewReserveInventoryStep(nil)
	assert.Equal(t, "inventory", step.Name())
}

func TestReserveInventoryStep_Execute_ReservesNextItem(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockInventoryClient(ctrl)
	step := saga.NewReserveInventoryStep(client)
	orderID := uuid.New()
	productID := uuid.New()
	order := &domain.Order{
		ID: orderID,
		Items: []domain.OrderItem{
			{ProductID: productID, Quantity: 5},
		},
	}
	s := &saga.Saga{Status: saga.SagaStatusReserving}

	client.EXPECT().Reserve(gomock.Any(), productID.String(), int32(5), orderID.String(), gomock.Any()).Return(nil)

	err := step.Execute(context.Background(), s, order, "key")
	require.NoError(t, err)
	require.Len(t, s.ReservedItems, 1)
	assert.Equal(t, productID.String(), s.ReservedItems[0].ProductID)
	assert.Equal(t, int32(5), s.ReservedItems[0].Quantity)
	assert.Equal(t, saga.SagaStatusReserving, s.Status)
}

func TestReserveInventoryStep_Execute_FinalizesWhenAllReserved(t *testing.T) {
	t.Parallel()

	step := saga.NewReserveInventoryStep(nil)
	order := &domain.Order{
		ID: uuid.New(),
		Items: []domain.OrderItem{
			{ProductID: uuid.New(), Quantity: 1},
		},
	}
	s := &saga.Saga{
		Status: saga.SagaStatusReserving,
		ReservedItems: []saga.SagaReservedItem{
			{ProductID: order.Items[0].ProductID.String(), Quantity: 1},
		},
	}

	err := step.Execute(context.Background(), s, order, "key")
	require.NoError(t, err)
	assert.Equal(t, saga.SagaStatusReserved, s.Status)
	assert.Equal(t, "reserved", s.CurrentStep)
}

func TestReserveInventoryStep_Execute_PropagatesReserveError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockInventoryClient(ctrl)
	step := saga.NewReserveInventoryStep(client)
	order := &domain.Order{
		ID: uuid.New(),
		Items: []domain.OrderItem{
			{ProductID: uuid.New(), Quantity: 2},
		},
	}
	s := &saga.Saga{Status: saga.SagaStatusReserving}
	want := errors.New("out of stock")

	client.EXPECT().Reserve(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(want)

	err := step.Execute(context.Background(), s, order, "key")
	assert.ErrorIs(t, err, want)
}

func TestReserveInventoryStep_Compensate_ReleasesReservedItems(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockInventoryClient(ctrl)
	step := saga.NewReserveInventoryStep(client)
	orderID := uuid.New()
	order := &domain.Order{ID: orderID}
	s := &saga.Saga{
		ReservedItems: []saga.SagaReservedItem{
			{ProductID: "p1", Quantity: 2},
			{ProductID: "p2", Quantity: 3},
		},
	}

	client.EXPECT().Release(gomock.Any(), "p1", int32(2), orderID.String(), gomock.Any()).Return(nil)
	client.EXPECT().Release(gomock.Any(), "p2", int32(3), orderID.String(), gomock.Any()).Return(nil)

	err := step.Compensate(context.Background(), s, order, "key")
	require.NoError(t, err)
}

func TestReserveInventoryStep_Compensate_JoinsErrors(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockInventoryClient(ctrl)
	step := saga.NewReserveInventoryStep(client)
	orderID := uuid.New()
	order := &domain.Order{ID: orderID}
	s := &saga.Saga{
		ReservedItems: []saga.SagaReservedItem{
			{ProductID: "p1", Quantity: 1},
		},
	}
	want := errors.New("release failed")

	client.EXPECT().Release(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(want)

	err := step.Compensate(context.Background(), s, order, "key")
	assert.ErrorIs(t, err, want)
}

func TestProcessPaymentStep_Name(t *testing.T) {
	t.Parallel()

	step := saga.NewProcessPaymentStep(nil)
	assert.Equal(t, "payment", step.Name())
}

func TestProcessPaymentStep_Prepare(t *testing.T) {
	t.Parallel()

	step := saga.NewProcessPaymentStep(nil)
	s := &saga.Saga{Status: saga.SagaStatusReserved}

	prepared := step.(saga.Preparable).Prepare(s)
	assert.True(t, prepared)
	assert.Equal(t, saga.SagaStatusPaying, s.Status)
	assert.Equal(t, "payment", s.CurrentStep)
}

func TestProcessPaymentStep_Prepare_AlreadyPaying(t *testing.T) {
	t.Parallel()

	step := saga.NewProcessPaymentStep(nil)
	s := &saga.Saga{Status: saga.SagaStatusPaying}

	prepared := step.(saga.Preparable).Prepare(s)
	assert.False(t, prepared)
	assert.Equal(t, saga.SagaStatusPaying, s.Status)
}

func TestProcessPaymentStep_Execute_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPaymentClient(ctrl)
	step := saga.NewProcessPaymentStep(client)
	orderID := uuid.New()
	order := &domain.Order{ID: orderID, TotalAmount: 999}
	s := &saga.Saga{Status: saga.SagaStatusPaying}

	client.EXPECT().ProcessPayment(gomock.Any(), orderID.String(), int64(999), gomock.Any()).Return("pay-123", nil)

	err := step.Execute(context.Background(), s, order, "key")
	require.NoError(t, err)
	assert.Equal(t, "pay-123", s.PaymentID)
	assert.Equal(t, saga.SagaStatusPaid, s.Status)
	assert.Equal(t, "paid", s.CurrentStep)
}

func TestProcessPaymentStep_Execute_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPaymentClient(ctrl)
	step := saga.NewProcessPaymentStep(client)
	order := &domain.Order{ID: uuid.New(), TotalAmount: 100}
	s := &saga.Saga{Status: saga.SagaStatusPaying}
	want := errors.New("declined")

	client.EXPECT().ProcessPayment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", want)

	err := step.Execute(context.Background(), s, order, "key")
	assert.ErrorIs(t, err, want)
	assert.Empty(t, s.PaymentID)
}

func TestProcessPaymentStep_Compensate_RefundsWhenPaymentIDPresent(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPaymentClient(ctrl)
	step := saga.NewProcessPaymentStep(client)
	s := &saga.Saga{PaymentID: "pay-456"}

	client.EXPECT().Refund(gomock.Any(), "pay-456", gomock.Any()).Return(nil)

	err := step.Compensate(context.Background(), s, &domain.Order{}, "key")
	require.NoError(t, err)
}

func TestProcessPaymentStep_Compensate_SkipsWhenNoPaymentID(t *testing.T) {
	t.Parallel()

	step := saga.NewProcessPaymentStep(nil)
	s := &saga.Saga{PaymentID: ""}

	err := step.Compensate(context.Background(), s, &domain.Order{}, "key")
	require.NoError(t, err)
}

func TestConfirmOrderStep_Name(t *testing.T) {
	t.Parallel()

	step := saga.NewConfirmOrderStep(nil)
	assert.Equal(t, "confirm", step.Name())
}

func TestConfirmOrderStep_Prepare(t *testing.T) {
	t.Parallel()

	step := saga.NewConfirmOrderStep(nil)
	s := &saga.Saga{Status: saga.SagaStatusPaid}

	prepared := step.(saga.Preparable).Prepare(s)
	assert.True(t, prepared)
	assert.Equal(t, saga.SagaStatusConfirming, s.Status)
	assert.Equal(t, "confirm", s.CurrentStep)
}

func TestConfirmOrderStep_Prepare_AlreadyConfirming(t *testing.T) {
	t.Parallel()

	step := saga.NewConfirmOrderStep(nil)
	s := &saga.Saga{Status: saga.SagaStatusConfirming}

	prepared := step.(saga.Preparable).Prepare(s)
	assert.False(t, prepared)
	assert.Equal(t, saga.SagaStatusConfirming, s.Status)
}

func TestConfirmOrderStep_Execute_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mocks.NewMockOrderRepository(ctrl)
	step := saga.NewConfirmOrderStep(orderRepo)
	order := &domain.Order{ID: uuid.New()}
	s := &saga.Saga{Status: saga.SagaStatusConfirming}

	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusPaid).Return(nil)

	err := step.Execute(context.Background(), s, order, "key")
	require.NoError(t, err)
	assert.Equal(t, saga.SagaStatusConfirmed, s.Status)
	assert.Equal(t, "confirmed", s.CurrentStep)
}

func TestConfirmOrderStep_Execute_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mocks.NewMockOrderRepository(ctrl)
	step := saga.NewConfirmOrderStep(orderRepo)
	order := &domain.Order{ID: uuid.New()}
	s := &saga.Saga{Status: saga.SagaStatusConfirming}
	want := errors.New("db unavailable")

	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusPaid).Return(want)

	err := step.Execute(context.Background(), s, order, "key")
	assert.ErrorIs(t, err, want)
}

func TestConfirmOrderStep_Compensate_IsNoOp(t *testing.T) {
	t.Parallel()

	step := saga.NewConfirmOrderStep(nil)
	err := step.Compensate(context.Background(), &saga.Saga{}, &domain.Order{}, "key")
	require.NoError(t, err)
}

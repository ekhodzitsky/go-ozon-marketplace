package saga_test

import (
	"context"
	stderrors "errors"
	"sync"
	"testing"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestOrder() *domain.Order {
	return &domain.Order{
		ID:     uuid.New(),
		UserID: uuid.New(),
		Items: []domain.OrderItem{
			{ProductID: uuid.New(), Quantity: 2},
			{ProductID: uuid.New(), Quantity: 3},
		},
		TotalAmount: 5000,
		Status:      domain.OrderStatusPending,
	}
}

func newTestOrchestrator(
	ctrl *gomock.Controller,
) (*saga.Orchestrator, *mocks.MockOrderRepository, *mocks.MockSagaRepository, *mocks.MockInventoryClient, *mocks.MockPaymentClient) {
	orderRepo := mocks.NewMockOrderRepository(ctrl)
	sagaRepo := mocks.NewMockSagaRepository(ctrl)
	invClient := mocks.NewMockInventoryClient(ctrl)
	payClient := mocks.NewMockPaymentClient(ctrl)
	log := zap.NewNop()
	o := saga.NewOrchestrator(orderRepo, sagaRepo, invClient, payClient, log, 100*time.Millisecond, 100*time.Millisecond)
	return o, orderRepo, sagaRepo, invClient, payClient
}

type sagaTransition struct {
	status      domain.SagaStatus
	step        string
	reservedLen int
	paymentID   string
}

func collectTransitions(sagaRepo *mocks.MockSagaRepository) (*sync.Mutex, *[]sagaTransition) {
	var mu sync.Mutex
	transitions := make([]sagaTransition, 0)
	sagaRepo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s *domain.Saga) error {
		mu.Lock()
		defer mu.Unlock()
		transitions = append(transitions, sagaTransition{
			status:      s.Status,
			step:        s.CurrentStep,
			reservedLen: len(s.ReservedItems),
			paymentID:   s.PaymentID,
		})
		return nil
	}).AnyTimes()
	return &mu, &transitions
}

func TestOrchestrator_ProcessOrder_HappyPath(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o, orderRepo, sagaRepo, invClient, payClient := newTestOrchestrator(ctrl)
	order := newTestOrder()
	idempKey := "test-key"

	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), order.ID).Return(nil, apperrors.ErrNotFound)
	sagaRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s *domain.Saga) error {
		assert.Equal(t, order.ID, s.OrderID)
		assert.Equal(t, domain.SagaStatusPending, s.Status)
		return nil
	}).Times(1)

	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusAwaitingPayment).Return(nil).Times(1)
	invClient.EXPECT().Reserve(gomock.Any(), order.Items[0].ProductID.String(), int32(order.Items[0].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	invClient.EXPECT().Reserve(gomock.Any(), order.Items[1].ProductID.String(), int32(order.Items[1].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	payClient.EXPECT().ProcessPayment(gomock.Any(), order.ID.String(), order.TotalAmount, gomock.Any()).Return("pay-123", nil).Times(1)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusPaid).Return(nil).Times(1)

	mu, transitions := collectTransitions(sagaRepo)

	err := o.ProcessOrder(context.Background(), order, idempKey)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, *transitions, 8)
	assert.Equal(t, domain.SagaStatusReserving, (*transitions)[0].status)
	assert.Equal(t, "reserve", (*transitions)[0].step)
	assert.Equal(t, 0, (*transitions)[0].reservedLen)

	assert.Equal(t, domain.SagaStatusReserving, (*transitions)[1].status)
	assert.Equal(t, 1, (*transitions)[1].reservedLen)

	assert.Equal(t, domain.SagaStatusReserving, (*transitions)[2].status)
	assert.Equal(t, 2, (*transitions)[2].reservedLen)

	assert.Equal(t, domain.SagaStatusReserved, (*transitions)[3].status)
	assert.Equal(t, "reserved", (*transitions)[3].step)
	assert.Equal(t, 2, (*transitions)[3].reservedLen)

	assert.Equal(t, domain.SagaStatusPaying, (*transitions)[4].status)
	assert.Equal(t, "payment", (*transitions)[4].step)

	assert.Equal(t, domain.SagaStatusPaid, (*transitions)[5].status)
	assert.Equal(t, "paid", (*transitions)[5].step)
	assert.Equal(t, "pay-123", (*transitions)[5].paymentID)

	assert.Equal(t, domain.SagaStatusConfirming, (*transitions)[6].status)
	assert.Equal(t, "confirm", (*transitions)[6].step)

	assert.Equal(t, domain.SagaStatusConfirmed, (*transitions)[7].status)
	assert.Equal(t, "confirmed", (*transitions)[7].step)
}

func TestOrchestrator_ProcessOrder_AlreadyCompleted(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o, _, sagaRepo, _, _ := newTestOrchestrator(ctrl)
	order := newTestOrder()

	tests := []struct {
		name   string
		status domain.SagaStatus
	}{
		{"confirmed", domain.SagaStatusConfirmed},
		{"cancelled", domain.SagaStatusCancelled},
		{"failed", domain.SagaStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sagaRepo.EXPECT().GetByOrderID(gomock.Any(), order.ID).Return(&domain.Saga{
				ID:      uuid.New(),
				OrderID: order.ID,
				Status:  tt.status,
			}, nil)

			err := o.ProcessOrder(context.Background(), order, "key")
			require.NoError(t, err)
		})
	}
}

func TestOrchestrator_ProcessOrder_ReserveFailsOnSecondItem(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o, orderRepo, sagaRepo, invClient, _ := newTestOrchestrator(ctrl)
	order := newTestOrder()
	reserveErr := stderrors.New("insufficient stock")
	idempKey := "test-key"

	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), order.ID).Return(nil, apperrors.ErrNotFound)
	sagaRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusAwaitingPayment).Return(nil).Times(1)
	invClient.EXPECT().Reserve(gomock.Any(), order.Items[0].ProductID.String(), int32(order.Items[0].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	invClient.EXPECT().Reserve(gomock.Any(), order.Items[1].ProductID.String(), int32(order.Items[1].Quantity), order.ID.String(), gomock.Any()).Return(reserveErr).Times(3) // retry
	invClient.EXPECT().Release(gomock.Any(), order.Items[0].ProductID.String(), int32(order.Items[0].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusCancelled).Return(nil).Times(1)

	mu, transitions := collectTransitions(sagaRepo)

	err := o.ProcessOrder(context.Background(), order, idempKey)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, *transitions, 4)
	assert.Equal(t, domain.SagaStatusReserving, (*transitions)[0].status)
	assert.Equal(t, domain.SagaStatusReserving, (*transitions)[1].status)
	assert.Equal(t, 1, (*transitions)[1].reservedLen)
	assert.Equal(t, domain.SagaStatusCompensating, (*transitions)[2].status)
	assert.Equal(t, domain.SagaStatusCancelled, (*transitions)[3].status)
}

func TestOrchestrator_ProcessOrder_PaymentFails(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o, orderRepo, sagaRepo, invClient, payClient := newTestOrchestrator(ctrl)
	order := newTestOrder()
	payErr := stderrors.New("payment declined")
	idempKey := "test-key"

	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), order.ID).Return(nil, apperrors.ErrNotFound)
	sagaRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusAwaitingPayment).Return(nil).Times(1)
	invClient.EXPECT().Reserve(gomock.Any(), order.Items[0].ProductID.String(), int32(order.Items[0].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	invClient.EXPECT().Reserve(gomock.Any(), order.Items[1].ProductID.String(), int32(order.Items[1].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	payClient.EXPECT().ProcessPayment(gomock.Any(), order.ID.String(), order.TotalAmount, gomock.Any()).Return("", payErr).Times(3) // retry
	invClient.EXPECT().Release(gomock.Any(), order.Items[0].ProductID.String(), int32(order.Items[0].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	invClient.EXPECT().Release(gomock.Any(), order.Items[1].ProductID.String(), int32(order.Items[1].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusCancelled).Return(nil).Times(1)

	mu, transitions := collectTransitions(sagaRepo)

	err := o.ProcessOrder(context.Background(), order, idempKey)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, *transitions, 7)
	assert.Equal(t, domain.SagaStatusReserved, (*transitions)[3].status)
	assert.Equal(t, domain.SagaStatusPaying, (*transitions)[4].status)
	assert.Equal(t, domain.SagaStatusCompensating, (*transitions)[5].status)
	assert.Equal(t, domain.SagaStatusCancelled, (*transitions)[6].status)
}

func TestOrchestrator_ProcessOrder_ConfirmFails(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o, orderRepo, sagaRepo, invClient, payClient := newTestOrchestrator(ctrl)
	order := newTestOrder()
	confirmErr := stderrors.New("db unavailable")
	idempKey := "test-key"

	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), order.ID).Return(nil, apperrors.ErrNotFound)
	sagaRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusAwaitingPayment).Return(nil).Times(1)
	invClient.EXPECT().Reserve(gomock.Any(), order.Items[0].ProductID.String(), int32(order.Items[0].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	invClient.EXPECT().Reserve(gomock.Any(), order.Items[1].ProductID.String(), int32(order.Items[1].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	payClient.EXPECT().ProcessPayment(gomock.Any(), order.ID.String(), order.TotalAmount, gomock.Any()).Return("pay-456", nil).Times(1)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusPaid).Return(confirmErr).Times(3) // retry
	payClient.EXPECT().Refund(gomock.Any(), "pay-456", gomock.Any()).Return(nil).Times(1)
	invClient.EXPECT().Release(gomock.Any(), order.Items[0].ProductID.String(), int32(order.Items[0].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	invClient.EXPECT().Release(gomock.Any(), order.Items[1].ProductID.String(), int32(order.Items[1].Quantity), order.ID.String(), gomock.Any()).Return(nil).Times(1)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusCancelled).Return(nil).Times(1)

	mu, transitions := collectTransitions(sagaRepo)

	err := o.ProcessOrder(context.Background(), order, idempKey)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, *transitions, 9)
	assert.Equal(t, domain.SagaStatusPaid, (*transitions)[5].status)
	assert.Equal(t, domain.SagaStatusConfirming, (*transitions)[6].status)
	assert.Equal(t, domain.SagaStatusCompensating, (*transitions)[7].status)
	assert.Equal(t, "pay-456", (*transitions)[7].paymentID)
	assert.Equal(t, domain.SagaStatusCancelled, (*transitions)[8].status)
}

func TestOrchestrator_ProcessOrder_ResumeFromReserved(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o, orderRepo, sagaRepo, _, payClient := newTestOrchestrator(ctrl)
	order := newTestOrder()
	existingSaga := &domain.Saga{
		ID:      uuid.New(),
		OrderID: order.ID,
		Status:  domain.SagaStatusReserved,
		ReservedItems: []domain.SagaReservedItem{
			{ProductID: order.Items[0].ProductID.String(), Quantity: int32(order.Items[0].Quantity)},
			{ProductID: order.Items[1].ProductID.String(), Quantity: int32(order.Items[1].Quantity)},
		},
	}
	idempKey := "test-key"

	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), order.ID).Return(existingSaga, nil)
	payClient.EXPECT().ProcessPayment(gomock.Any(), order.ID.String(), order.TotalAmount, gomock.Any()).Return("pay-789", nil).Times(1)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusPaid).Return(nil).Times(1)

	mu, transitions := collectTransitions(sagaRepo)

	err := o.ProcessOrder(context.Background(), order, idempKey)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, *transitions, 4)
	assert.Equal(t, domain.SagaStatusPaying, (*transitions)[0].status)
	assert.Equal(t, domain.SagaStatusPaid, (*transitions)[1].status)
	assert.Equal(t, "pay-789", (*transitions)[1].paymentID)
	assert.Equal(t, domain.SagaStatusConfirming, (*transitions)[2].status)
	assert.Equal(t, domain.SagaStatusConfirmed, (*transitions)[3].status)
}

func TestOrchestrator_Recover(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o, orderRepo, sagaRepo, _, payClient := newTestOrchestrator(ctrl)
	order := newTestOrder()
	incompleteSaga := domain.Saga{
		ID:      uuid.New(),
		OrderID: order.ID,
		Status:  domain.SagaStatusPaying,
		ReservedItems: []domain.SagaReservedItem{
			{ProductID: order.Items[0].ProductID.String(), Quantity: int32(order.Items[0].Quantity)},
		},
	}

	sagaRepo.EXPECT().ListIncomplete(gomock.Any(), 100).Return([]domain.Saga{incompleteSaga}, nil)
	orderRepo.EXPECT().GetByID(gomock.Any(), order.ID).Return(order, nil)
	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), order.ID).Return(&incompleteSaga, nil)
	payClient.EXPECT().ProcessPayment(gomock.Any(), order.ID.String(), order.TotalAmount, gomock.Any()).Return("pay-rec", nil).Times(1)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), order.ID, domain.OrderStatusPaid).Return(nil).Times(1)

	mu, transitions := collectTransitions(sagaRepo)

	err := o.Recover(context.Background())
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, *transitions, 3)
	assert.Equal(t, domain.SagaStatusPaid, (*transitions)[0].status)
	assert.Equal(t, domain.SagaStatusConfirming, (*transitions)[1].status)
	assert.Equal(t, domain.SagaStatusConfirmed, (*transitions)[2].status)
}

func TestOrchestrator_Recover_GetOrderFails(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o, orderRepo, sagaRepo, _, _ := newTestOrchestrator(ctrl)
	order := newTestOrder()
	incompleteSaga := domain.Saga{
		ID:      uuid.New(),
		OrderID: order.ID,
		Status:  domain.SagaStatusPaying,
	}

	sagaRepo.EXPECT().ListIncomplete(gomock.Any(), 100).Return([]domain.Saga{incompleteSaga}, nil)
	orderRepo.EXPECT().GetByID(gomock.Any(), order.ID).Return(nil, stderrors.New("db error"))

	err := o.Recover(context.Background())
	require.NoError(t, err)
}

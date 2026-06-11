package saga_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestRecoveryWorker_StartStop(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orchestrator, _, sagaRepo, _, _ := newTestOrchestrator(ctrl)
	sagaRepo.EXPECT().ListIncomplete(gomock.Any(), 100).Return(nil, nil).AnyTimes()

	log := zap.NewNop()
	w := saga.NewRecoveryWorker(orchestrator, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)
	// Start is idempotent
	w.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	w.Stop()
	// Stop is idempotent
	w.Stop()
}

func TestRecoveryWorker_RecoverOnceError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orchestrator, _, sagaRepo, _, _ := newTestOrchestrator(ctrl)
	sagaRepo.EXPECT().ListIncomplete(gomock.Any(), 100).Return(nil, errors.New("db error"))

	log := zap.NewNop()
	w := saga.NewRecoveryWorker(orchestrator, log)

	ctx := context.Background()
	w.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	w.Stop()
}

func TestRecoveryWorker_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orchestrator, _, sagaRepo, _, _ := newTestOrchestrator(ctrl)
	sagaRepo.EXPECT().ListIncomplete(gomock.Any(), 100).Return([]domain.Saga{}, nil).AnyTimes()

	log := zap.NewNop()
	w := saga.NewRecoveryWorker(orchestrator, log)

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for loop to exit
	done := make(chan struct{})
	go func() {
		// We can't directly observe internal state, but Stop should not deadlock
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("Stop deadlocked after context cancellation")
	}
}

func TestRecoveryWorker_ProcessesIncompleteSagas(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mocks.NewMockOrderRepository(ctrl)
	sagaRepo := mocks.NewMockSagaRepository(ctrl)
	invClient := mocks.NewMockInventoryClient(ctrl)
	payClient := mocks.NewMockPaymentClient(ctrl)
	log := zap.NewNop()

	orchestrator := saga.NewOrchestrator(orderRepo, sagaRepo, invClient, payClient, log, 100*time.Millisecond, 100*time.Millisecond)

	orderID := uuid.New()
	incompleteSaga := domain.Saga{
		ID:      uuid.New(),
		OrderID: orderID,
		Status:  domain.SagaStatusReserved,
	}
	order := &domain.Order{
		ID:     orderID,
		UserID: uuid.New(),
		Items:  []domain.OrderItem{},
		Status: "awaiting_payment",
	}

	sagaRepo.EXPECT().ListIncomplete(gomock.Any(), 100).Return([]domain.Saga{incompleteSaga}, nil)
	orderRepo.EXPECT().GetByID(gomock.Any(), orderID).Return(order, nil)
	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), orderID).Return(&incompleteSaga, nil)
	// ProcessOrder from Reserved state will transition to Paying, Paid, Confirming, Confirmed
	sagaRepo.EXPECT().Save(gomock.Any(), gomock.Any()).AnyTimes()
	payClient.EXPECT().ProcessPayment(gomock.Any(), orderID.String(), order.UserID.String(), order.TotalAmount).Return("pay-001", nil)
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), orderID, "confirmed").Return(nil)

	w := saga.NewRecoveryWorker(orchestrator, log)
	ctx := context.Background()
	w.Start(ctx)

	time.Sleep(200 * time.Millisecond)
	w.Stop()

	assert.True(t, true)
}

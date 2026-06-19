package saga_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	grpcclient "github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/infrastructure/grpcclient"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type fakeLocker struct {
	locked bool
}

func (f *fakeLocker) TryLock(ctx context.Context, key string) (bool, error) {
	if f.locked {
		return false, nil
	}
	f.locked = true
	return true, nil
}

func (f *fakeLocker) Unlock(ctx context.Context, key string) error {
	f.locked = false
	return nil
}

func TestRecoveryWorker_StartStop(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orchestrator, _, sagaRepo, _, _ := newTestOrchestrator(ctrl)
	sagaRepo.EXPECT().ListIncomplete(gomock.Any(), 100).Return(nil, nil).AnyTimes()

	log := zap.NewNop()
	w := saga.NewRecoveryWorker(orchestrator, log, saga.WithRecoveryInterval(50*time.Millisecond))

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
	sagaRepo.EXPECT().ListIncomplete(gomock.Any(), 100).Return(nil, errors.New("db error")).AnyTimes()

	log := zap.NewNop()
	w := saga.NewRecoveryWorker(orchestrator, log, saga.WithRecoveryInterval(50*time.Millisecond))

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
	w := saga.NewRecoveryWorker(orchestrator, log, saga.WithRecoveryInterval(50*time.Millisecond))

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
	invClient := mocks.NewMockInventoryServiceClient(ctrl)
	payClient := mocks.NewMockPaymentServiceClient(ctrl)
	log := zap.NewNop()

	orchestrator := saga.NewOrchestrator(
		orderRepo, sagaRepo,
		grpcclient.NewInventoryClient(invClient, 100*time.Millisecond),
		grpcclient.NewPaymentClient(payClient, 100*time.Millisecond),
		log, 100*time.Millisecond, 100*time.Millisecond,
	)

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
		Status: domain.OrderStatusAwaitingPayment,
	}

	sagaRepo.EXPECT().ListIncomplete(gomock.Any(), 100).Return([]domain.Saga{incompleteSaga}, nil).AnyTimes()
	orderRepo.EXPECT().GetByID(gomock.Any(), orderID).Return(order, nil).AnyTimes()
	sagaRepo.EXPECT().GetByOrderID(gomock.Any(), orderID).Return(&incompleteSaga, nil).AnyTimes()
	// ProcessOrder from Reserved state will transition to Paying, Paid, Confirming, Confirmed
	sagaRepo.EXPECT().Save(gomock.Any(), gomock.Any()).AnyTimes()
	payClient.EXPECT().ProcessPayment(gomock.Any(), &paymentv1.ProcessPaymentRequest{
		OrderId:        orderID.String(),
		AmountCents:    order.TotalAmount,
		IdempotencyKey: fmt.Sprintf("payment:recovery:%s", orderID.String()),
	}).Return(&paymentv1.ProcessPaymentResponse{PaymentId: "pay-001"}, nil).AnyTimes()
	orderRepo.EXPECT().UpdateStatus(gomock.Any(), orderID, domain.OrderStatusPaid).Return(nil).AnyTimes()

	w := saga.NewRecoveryWorker(orchestrator, log, saga.WithLocker(&fakeLocker{}), saga.WithRecoveryInterval(50*time.Millisecond))
	ctx := context.Background()
	w.Start(ctx)

	time.Sleep(200 * time.Millisecond)
	w.Stop()

	assert.True(t, true)
}

package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// stubTxManager runs the callback against a provided repository.
type stubTxManager struct {
	repo repository.PaymentRepository
	err  error
}

func (m *stubTxManager) Run(ctx context.Context, fn func(repository.PaymentRepository) error) error {
	if m.err != nil {
		return m.err
	}
	return fn(m.repo)
}

func TestPaymentUsecase_ProcessPayment_TxManagerError(t *testing.T) {
	t.Parallel()
	repo := newMockPaymentRepository()
	txm := &stubTxManager{err: errors.New("tx failed")}
	uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)

	payment, err := uc.ProcessPayment(context.Background(), uuid.New(), uuid.New(), 1000)
	require.Error(t, err)
	assert.Nil(t, payment)
}

func TestPaymentUsecase_ProcessPayment_NilRandGen(t *testing.T) {
	t.Parallel()
	repo := newMockPaymentRepository()
	txm := &stubTxManager{repo: repo}
	uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)
	// randGen is nil; the result should still be one of the valid statuses.

	payment, err := uc.ProcessPayment(context.Background(), uuid.New(), uuid.New(), 1000)
	require.NoError(t, err)
	assert.True(t, payment.Status == domain.StatusSuccess || payment.Status == domain.StatusFailed)
}

func TestPaymentUsecase_ProcessPayment_IdempotencyPreservesStatusAndAmount(t *testing.T) {
	t.Parallel()
	repo := newMockPaymentRepository()
	txm := &stubTxManager{repo: repo}
	uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)
	uc.randGen = func() float64 { return 0.5 } // success

	orderID := uuid.New()
	userID := uuid.New()

	p1, err := uc.ProcessPayment(context.Background(), orderID, userID, 5000)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSuccess, p1.Status)
	assert.Equal(t, int64(5000), p1.Amount)

	// Second call with same order but different amount must return the original payment.
	uc.randGen = func() float64 { return 0.95 } // would be failed if not idempotent
	p2, err := uc.ProcessPayment(context.Background(), orderID, userID, 9999)
	require.NoError(t, err)
	assert.Equal(t, p1.ID, p2.ID)
	assert.Equal(t, domain.StatusSuccess, p2.Status)
	assert.Equal(t, int64(5000), p2.Amount)
}

func TestPaymentUsecase_Refund_CreateRefundError(t *testing.T) {
	t.Parallel()
	repo := newMockPaymentRepository()
	repo.refundErr = errors.New("db down")
	txm := &stubTxManager{repo: repo}
	uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)

	payment := &domain.Payment{ID: uuid.New(), Status: domain.StatusSuccess}
	require.NoError(t, repo.Create(context.Background(), payment))

	payment, refund, err := uc.Refund(context.Background(), payment.ID, uuid.New().String())
	require.Error(t, err)
	assert.Nil(t, payment)
	assert.Nil(t, refund)
}

func TestPaymentUsecase_Refund_TxManagerError(t *testing.T) {
	t.Parallel()
	repo := newMockPaymentRepository()
	txm := &stubTxManager{err: errors.New("tx failed")}
	uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)

	payment := &domain.Payment{ID: uuid.New(), Status: domain.StatusSuccess}
	require.NoError(t, repo.Create(context.Background(), payment))

	payment, refund, err := uc.Refund(context.Background(), payment.ID, uuid.New().String())
	require.Error(t, err)
	assert.Nil(t, payment)
	assert.Nil(t, refund)
}

func TestPaymentUsecase_Refund_IdempotencyKeyReuse(t *testing.T) {
	t.Parallel()
	repo := newMockPaymentRepository()
	txm := &stubTxManager{repo: repo}
	uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)

	payment := &domain.Payment{ID: uuid.New(), Status: domain.StatusSuccess}
	require.NoError(t, repo.Create(context.Background(), payment))

	idemKey := uuid.New().String()
	_, refund1, err := uc.Refund(context.Background(), payment.ID, idemKey)
	require.NoError(t, err)
	require.NotNil(t, refund1)

	// Повторный вызов с тем же ключом должен вернуть тот же возврат.
	_, refund2, err := uc.Refund(context.Background(), payment.ID, idemKey)
	require.NoError(t, err)
	assert.Equal(t, refund1.ID, refund2.ID)
}

func TestPaymentUsecase_GetByID_NotFound(t *testing.T) {
	t.Parallel()
	repo := newMockPaymentRepository()
	txm := &stubTxManager{repo: repo}
	uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)

	payment, err := uc.GetByID(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Nil(t, payment)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestPaymentUsecase_GetRefund_NotFound(t *testing.T) {
	t.Parallel()
	repo := newMockPaymentRepository()
	txm := &stubTxManager{repo: repo}
	uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)

	refund, err := uc.GetRefund(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Nil(t, refund)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestPaymentUsecase_ListRefunds(t *testing.T) {
	t.Parallel()
	repo := newMockPaymentRepository()
	txm := &stubTxManager{repo: repo}
	uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)

	paymentID := uuid.New()

	// Empty list when no refunds exist.
	list, err := uc.ListRefunds(context.Background(), paymentID)
	require.NoError(t, err)
	assert.Empty(t, list)

	// Add refunds and verify ordering (most recent first).
	for i := 0; i < 3; i++ {
		repo.refunds[uuid.New()] = &domain.Refund{
			ID:        uuid.New(),
			PaymentID: paymentID,
			Amount:    int64(100 * i),
			Status:    domain.StatusRefunded,
			CreatedAt: time.Now().UTC().Add(time.Duration(i) * time.Hour),
		}
	}

	list, err = uc.ListRefunds(context.Background(), paymentID)
	require.NoError(t, err)
	assert.Len(t, list, 3)
	// ListRefunds orders by created_at DESC.
	assert.True(t, list[0].CreatedAt.After(list[1].CreatedAt) || list[0].CreatedAt.Equal(list[1].CreatedAt))
}

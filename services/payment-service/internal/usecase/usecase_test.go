package usecase

import (
	"context"
	"sort"
	"testing"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockPaymentRepository is a test double for PaymentRepository
type mockPaymentRepository struct {
	payments      map[uuid.UUID]*domain.Payment
	orderPayments map[uuid.UUID]*domain.Payment
	refunds       map[uuid.UUID]*domain.Refund
	createErr     error
	refundErr     error
	updateErr     error
}

func newMockPaymentRepository() *mockPaymentRepository {
	return &mockPaymentRepository{
		payments:      make(map[uuid.UUID]*domain.Payment),
		orderPayments: make(map[uuid.UUID]*domain.Payment),
		refunds:       make(map[uuid.UUID]*domain.Refund),
	}
}

func (m *mockPaymentRepository) WithTx(_ pgx.Tx) repository.PaymentRepository {
	return m
}

func (m *mockPaymentRepository) Create(ctx context.Context, payment *domain.Payment) error {
	if m.createErr != nil {
		return m.createErr
	}
	if _, exists := m.payments[payment.ID]; exists {
		return assert.AnError
	}
	m.payments[payment.ID] = payment
	m.orderPayments[payment.OrderID] = payment
	return nil
}

func (m *mockPaymentRepository) CreateOrGet(ctx context.Context, payment *domain.Payment) (*domain.Payment, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if existing, exists := m.orderPayments[payment.OrderID]; exists {
		return existing, nil
	}
	m.payments[payment.ID] = payment
	m.orderPayments[payment.OrderID] = payment
	return payment, nil
}

func (m *mockPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	payment, ok := m.payments[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return payment, nil
}

func (m *mockPaymentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	payment, ok := m.payments[id]
	if !ok {
		return assert.AnError
	}
	payment.Status = status
	return nil
}

func (m *mockPaymentRepository) UpdateStatusIf(ctx context.Context, id uuid.UUID, newStatus, expectedStatus domain.Status) (bool, error) {
	if m.updateErr != nil {
		return false, m.updateErr
	}
	payment, ok := m.payments[id]
	if !ok {
		return false, nil
	}
	if payment.Status != expectedStatus {
		return false, nil
	}
	payment.Status = newStatus
	return true, nil
}

func (m *mockPaymentRepository) CreateRefund(ctx context.Context, refund *domain.Refund) error {
	if m.refundErr != nil {
		return m.refundErr
	}
	for _, existing := range m.refunds {
		if existing.IdempotencyKey != "" && existing.IdempotencyKey == refund.IdempotencyKey {
			return assert.AnError
		}
	}
	m.refunds[refund.ID] = refund
	return nil
}

func (m *mockPaymentRepository) GetRefund(ctx context.Context, id uuid.UUID) (*domain.Refund, error) {
	refund, ok := m.refunds[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return refund, nil
}

func (m *mockPaymentRepository) ListRefunds(ctx context.Context, paymentID uuid.UUID) ([]*domain.Refund, error) {
	var out []*domain.Refund
	for _, refund := range m.refunds {
		if refund.PaymentID == paymentID {
			out = append(out, refund)
		}
	}
	// Descending by created_at to match the production implementation.
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

type mockTxManager struct {
	repo repository.PaymentRepository
	err  error
}

func (m *mockTxManager) Run(ctx context.Context, fn func(repo repository.PaymentRepository) error) error {
	if m.err != nil {
		return m.err
	}
	return fn(m.repo)
}

func TestPaymentUsecase_ProcessPayment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		amount     int64
		randGen    func() float64
		createErr  error
		updateErr  error
		wantStatus domain.Status
		wantErr    bool
	}{
		{
			name:       "success",
			amount:     10000,
			randGen:    func() float64 { return 0.5 },
			wantStatus: domain.StatusSuccess,
			wantErr:    false,
		},
		{
			name:       "failure",
			amount:     10000,
			randGen:    func() float64 { return 0.95 },
			wantStatus: domain.StatusFailed,
			wantErr:    false,
		},
		{
			name:    "amount_zero",
			amount:  0,
			randGen: func() float64 { return 0.5 },
			wantErr: true,
		},
		{
			name:      "create_error",
			amount:    10000,
			randGen:   func() float64 { return 0.5 },
			createErr: assert.AnError,
			wantErr:   true,
		},
		{
			name:      "update_status_error",
			amount:    10000,
			randGen:   func() float64 { return 0.5 },
			updateErr: assert.AnError,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newMockPaymentRepository()
			repo.createErr = tt.createErr
			repo.updateErr = tt.updateErr
			txm := &mockTxManager{repo: repo}
			uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)
			uc.randGen = tt.randGen

			orderID := uuid.New()
			userID := uuid.New()

			payment, err := uc.ProcessPayment(context.Background(), orderID, userID, tt.amount)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, payment.ID)
			assert.Equal(t, tt.wantStatus, payment.Status)
		})
	}
}

func TestPaymentUsecase_ProcessPayment_Idempotent(t *testing.T) {
	t.Parallel()

	repo := newMockPaymentRepository()
	txm := &mockTxManager{repo: repo}
	uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)
	uc.randGen = func() float64 { return 0.5 } // success

	orderID := uuid.New()
	userID := uuid.New()

	// first call
	p1, err := uc.ProcessPayment(context.Background(), orderID, userID, 10000)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSuccess, p1.Status)

	// second call with same order_id should return existing payment
	uc.randGen = func() float64 { return 0.95 } // would be failed if not idempotent
	p2, err := uc.ProcessPayment(context.Background(), orderID, userID, 10000)
	require.NoError(t, err)
	assert.Equal(t, p1.ID, p2.ID)
	assert.Equal(t, domain.StatusSuccess, p2.Status)
}

func TestPaymentUsecase_Refund(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		initialStatus domain.Status
		wantErr       bool
	}{
		{
			name:          "success",
			initialStatus: domain.StatusSuccess,
			wantErr:       false,
		},
		{
			name:    "not_found",
			wantErr: true,
		},
		{
			name:          "already_refunded",
			initialStatus: domain.StatusRefunded,
			wantErr:       true,
		},
		{
			name:          "failed_status",
			initialStatus: domain.StatusFailed,
			wantErr:       true,
		},
		{
			name:          "pending_status",
			initialStatus: domain.StatusPending,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newMockPaymentRepository()
			txm := &mockTxManager{repo: repo}
			uc := NewPaymentUsecase(repo, txm, zap.NewNop(), time.Second, time.Second)

			var paymentID uuid.UUID
			if tt.name != "not_found" {
				payment := &domain.Payment{
					ID:     uuid.New(),
					Status: tt.initialStatus,
				}
				require.NoError(t, repo.Create(context.Background(), payment))
				paymentID = payment.ID
			} else {
				paymentID = uuid.New()
			}

			payment, refund, err := uc.Refund(context.Background(), paymentID, uuid.New().String())

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, domain.StatusRefunded, payment.Status)
			assert.NotNil(t, refund)
			assert.Equal(t, domain.StatusRefunded, refund.Status)

			// Verify repository was updated
			updated, err := repo.GetByID(context.Background(), paymentID)
			require.NoError(t, err)
			assert.Equal(t, domain.StatusRefunded, updated.Status)
		})
	}
}

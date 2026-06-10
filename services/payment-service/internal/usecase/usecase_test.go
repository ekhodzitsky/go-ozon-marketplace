package usecase

import (
	"context"
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockPaymentRepository is a test double for PaymentRepository
type mockPaymentRepository struct {
	payments   map[uuid.UUID]*domain.Payment
	createErr  error
	updateErr  error
}

func newMockPaymentRepository() *mockPaymentRepository {
	return &mockPaymentRepository{
		payments: make(map[uuid.UUID]*domain.Payment),
	}
}

func (m *mockPaymentRepository) Create(ctx context.Context, payment *domain.Payment) error {
	if m.createErr != nil {
		return m.createErr
	}
	if _, exists := m.payments[payment.ID]; exists {
		return assert.AnError
	}
	m.payments[payment.ID] = payment
	return nil
}

func (m *mockPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	payment, ok := m.payments[id]
	if !ok {
		return nil, assert.AnError
	}
	return payment, nil
}

func (m *mockPaymentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
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

func TestPaymentUsecase_ProcessPayment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		randGen    func() float64
		createErr  error
		updateErr  error
		wantStatus string
		wantErr    bool
	}{
		{
			name:       "success",
			randGen:    func() float64 { return 0.5 },
			wantStatus: domain.StatusSuccess,
			wantErr:    false,
		},
		{
			name:       "failure",
			randGen:    func() float64 { return 0.95 },
			wantStatus: domain.StatusFailed,
			wantErr:    false,
		},
		{
			name:      "create_error",
			randGen:   func() float64 { return 0.5 },
			createErr: assert.AnError,
			wantErr:   true,
		},
		{
			name:      "update_status_error",
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
			uc := NewPaymentUsecase(repo, zap.NewNop())
			uc.randGen = tt.randGen
			uc.sleeper = func() {} // no-op for deterministic tests

			payment, err := uc.ProcessPayment(context.Background(), uuid.New(), uuid.New(), 100.0)

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

func TestPaymentUsecase_Refund(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success",
			wantErr: false,
		},
		{
			name:    "not_found",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newMockPaymentRepository()
			uc := NewPaymentUsecase(repo, zap.NewNop())

			var paymentID uuid.UUID
			if tt.name == "success" {
				payment := &domain.Payment{
					ID:     uuid.New(),
					Status: domain.StatusSuccess,
				}
				require.NoError(t, repo.Create(context.Background(), payment))
				paymentID = payment.ID
			} else {
				paymentID = uuid.New()
			}

			payment, err := uc.Refund(context.Background(), paymentID)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, domain.StatusRefunded, payment.Status)

			// Verify repository was updated
			updated, err := repo.GetByID(context.Background(), paymentID)
			require.NoError(t, err)
			assert.Equal(t, domain.StatusRefunded, updated.Status)
		})
	}
}

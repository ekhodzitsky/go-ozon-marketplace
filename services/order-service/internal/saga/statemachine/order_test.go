package statemachine_test

import (
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/statemachine"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newStateMachine(t *testing.T) (*statemachine.OrderSagaStateMachine, *steps.StartStep, *steps.ReserveInventoryStep, *steps.ProcessPaymentStep, *steps.ConfirmOrderStep) {
	t.Helper()
	startStep := steps.NewStartStep(nil, zap.NewNop())
	reserveStep := steps.NewReserveInventoryStep(nil)
	paymentStep := steps.NewProcessPaymentStep(nil)
	confirmStep := steps.NewConfirmOrderStep(nil)
	return statemachine.NewOrderSagaStateMachine(startStep, reserveStep, paymentStep, confirmStep), startStep, reserveStep, paymentStep, confirmStep
}

func TestOrderSagaStateMachine_Next(t *testing.T) {
	t.Parallel()

	sm, startStep, reserveStep, paymentStep, confirmStep := newStateMachine(t)
	order := &domain.Order{ID: uuid.New()}

	cases := []struct {
		name   string
		status domain.SagaStatus
		want   steps.Step
	}{
		{"pending", domain.SagaStatusPending, startStep},
		{"reserving", domain.SagaStatusReserving, reserveStep},
		{"reserved", domain.SagaStatusReserved, paymentStep},
		{"paying", domain.SagaStatusPaying, paymentStep},
		{"paid", domain.SagaStatusPaid, confirmStep},
		{"confirming", domain.SagaStatusConfirming, confirmStep},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saga := &domain.Saga{Status: tc.status}
			got, err := sm.Next(saga, order)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestOrderSagaStateMachine_Next_Terminal(t *testing.T) {
	t.Parallel()

	sm, _, _, _, _ := newStateMachine(t)
	order := &domain.Order{ID: uuid.New()}

	terminal := []domain.SagaStatus{
		domain.SagaStatusConfirmed,
		domain.SagaStatusCancelled,
		domain.SagaStatusFailed,
		domain.SagaStatusCompensating,
	}

	for _, status := range terminal {
		saga := &domain.Saga{Status: status}
		got, err := sm.Next(saga, order)
		require.NoError(t, err)
		assert.Nil(t, got)
	}
}

func TestOrderSagaStateMachine_IsTerminal(t *testing.T) {
	t.Parallel()

	sm, _, _, _, _ := newStateMachine(t)

	assert.True(t, sm.IsTerminal(&domain.Saga{Status: domain.SagaStatusConfirmed}))
	assert.True(t, sm.IsTerminal(&domain.Saga{Status: domain.SagaStatusCancelled}))
	assert.True(t, sm.IsTerminal(&domain.Saga{Status: domain.SagaStatusFailed}))
	assert.False(t, sm.IsTerminal(&domain.Saga{Status: domain.SagaStatusPending}))
	assert.False(t, sm.IsTerminal(&domain.Saga{Status: domain.SagaStatusReserving}))
}

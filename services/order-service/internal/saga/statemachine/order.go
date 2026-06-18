package statemachine

import (
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
)

// OrderSagaStateMachine is the adapter that encodes the order saga graph. It
// maps each saga status to the next step that should run.
type OrderSagaStateMachine struct {
	startStep   *steps.StartStep
	reserveStep *steps.ReserveInventoryStep
	paymentStep *steps.ProcessPaymentStep
	confirmStep *steps.ConfirmOrderStep
}

// NewOrderSagaStateMachine builds the state machine adapter from the step
// adapters. The constructor gives the caller explicit locality over the
// concrete steps without exposing them through the StateMachine interface.
func NewOrderSagaStateMachine(
	startStep *steps.StartStep,
	reserveStep *steps.ReserveInventoryStep,
	paymentStep *steps.ProcessPaymentStep,
	confirmStep *steps.ConfirmOrderStep,
) *OrderSagaStateMachine {
	return &OrderSagaStateMachine{
		startStep:   startStep,
		reserveStep: reserveStep,
		paymentStep: paymentStep,
		confirmStep: confirmStep,
	}
}

var _ StateMachine = (*OrderSagaStateMachine)(nil)

// Next returns the step that should run for the current saga status.
func (sm *OrderSagaStateMachine) Next(saga *domain.Saga, order *domain.Order) (steps.Step, error) {
	switch saga.Status {
	case domain.SagaStatusPending:
		return sm.startStep, nil
	case domain.SagaStatusReserving:
		return sm.reserveStep, nil
	case domain.SagaStatusReserved, domain.SagaStatusPaying:
		return sm.paymentStep, nil
	case domain.SagaStatusPaid, domain.SagaStatusConfirming:
		return sm.confirmStep, nil
	case domain.SagaStatusConfirmed, domain.SagaStatusCancelled, domain.SagaStatusFailed, domain.SagaStatusCompensating:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected saga status: %s", saga.Status)
	}
}

// IsTerminal reports whether the saga has reached a final state.
func (sm *OrderSagaStateMachine) IsTerminal(saga *domain.Saga) bool {
	switch saga.Status {
	case domain.SagaStatusConfirmed, domain.SagaStatusCancelled, domain.SagaStatusFailed:
		return true
	}
	return false
}

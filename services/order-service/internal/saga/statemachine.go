package saga

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/statemachine"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
)

// NewOrderSagaStateMachine is the adapter constructor for the StateMachine
// seam. It wires the concrete step adapters into the order saga graph.
func NewOrderSagaStateMachine(
	startStep Step,
	reserveStep Step,
	paymentStep Step,
	confirmStep Step,
) StateMachine {
	return statemachine.NewOrderSagaStateMachine(
		startStep.(*steps.StartStep),
		reserveStep.(*steps.ReserveInventoryStep),
		paymentStep.(*steps.ProcessPaymentStep),
		confirmStep.(*steps.ConfirmOrderStep),
	)
}

package saga

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/compensator"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
)

// NewOrderCompensator is the adapter constructor for the CompensationPlanner
// seam. It wires the concrete step adapters that will be used to compensate.
func NewOrderCompensator(reserveStep Step, paymentStep Step) CompensationPlanner {
	return compensator.NewOrderCompensator(
		reserveStep.(*steps.ReserveInventoryStep),
		paymentStep.(*steps.ProcessPaymentStep),
	)
}

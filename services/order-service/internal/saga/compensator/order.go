package compensator

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
)

// OrderCompensator is the adapter that plans compensation for the order saga.
// The plan is always payment first, then inventory, matching the required
// compensation order.
type OrderCompensator struct {
	reserveStep *steps.ReserveInventoryStep
	paymentStep *steps.ProcessPaymentStep
}

// NewOrderCompensator creates an OrderCompensator from the concrete step
// adapters it will return in plans.
func NewOrderCompensator(reserveStep *steps.ReserveInventoryStep, paymentStep *steps.ProcessPaymentStep) *OrderCompensator {
	return &OrderCompensator{
		reserveStep: reserveStep,
		paymentStep: paymentStep,
	}
}

var _ CompensationPlanner = (*OrderCompensator)(nil)

// Plan returns the compensation steps for a failed step. Inventory-only
// failures do not invoke the payment refund step because there is no payment
// to refund; confirm failures refund payment before releasing inventory.
func (c *OrderCompensator) Plan(saga *domain.Saga, failed steps.Step) []steps.Step {
	switch failed.Name() {
	case "confirm":
		return []steps.Step{c.paymentStep, c.reserveStep}
	case "inventory", "payment":
		return []steps.Step{c.reserveStep}
	default:
		return []steps.Step{c.reserveStep}
	}
}

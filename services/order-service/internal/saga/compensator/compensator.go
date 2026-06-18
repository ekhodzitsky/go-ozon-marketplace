package compensator

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
)

// CompensationPlanner is the seam that decides which compensation steps must
// run after a step failure. Keeping this logic in its own module gives the
// orchestrator a clean, deep interface.
type CompensationPlanner interface {
	Plan(saga *domain.Saga, failed steps.Step) []steps.Step
}

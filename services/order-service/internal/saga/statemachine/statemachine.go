package statemachine

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
)

// StateMachine is the seam that decides which step the saga should run next.
// Keeping this decision behind an interface gives the orchestrator a single,
// deep lever for controlling saga flow.
type StateMachine interface {
	Next(saga *domain.Saga, order *domain.Order) (steps.Step, error)
	IsTerminal(saga *domain.Saga) bool
}

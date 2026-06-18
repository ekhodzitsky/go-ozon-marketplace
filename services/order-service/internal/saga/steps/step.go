package steps

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
)

// Step is the seam for a single saga step. Each adapter implements the same
// small interface, which gives the state machine depth without leaking
// orchestration concerns.
type Step interface {
	Name() string
	Execute(ctx context.Context, saga *domain.Saga, order *domain.Order, idempotencyKey string) error
	Compensate(ctx context.Context, saga *domain.Saga, order *domain.Order, idempotencyKey string) error
}

// Preparable is implemented by steps that need to record an intermediate
// status before their main action runs. The orchestrator uses it to persist
// the pending state (for example, "paying") before invoking remote calls.
// The returned boolean reports whether the saga was mutated.
type Preparable interface {
	Prepare(saga *domain.Saga) bool
}

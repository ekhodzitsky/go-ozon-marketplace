package executor

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
)

// StepExecutor is the seam that runs a step and its compensation. Isolating
// retry policy here gives the orchestrator depth: it decides what to run, this
// module decides how to run it reliably.
type StepExecutor interface {
	Execute(ctx context.Context, step steps.Step, saga *domain.Saga, order *domain.Order, idempotencyKey string) error
	Compensate(ctx context.Context, step steps.Step, saga *domain.Saga, order *domain.Order, idempotencyKey string) error
}

// RetryConfig controls the executor's retry behavior.
type RetryConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	CallTimeout time.Duration
}

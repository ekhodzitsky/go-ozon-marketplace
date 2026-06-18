package saga

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/compensator"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/executor"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/statemachine"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/store"
)

// InventoryClient is the seam used by the saga to reserve and release
// inventory. The concrete client lives outside this module; this interface
// keeps the dependency narrow and deep.
type InventoryClient interface {
	Reserve(ctx context.Context, productID string, quantity int32, orderID string, idempotencyKey string) error
	Release(ctx context.Context, productID string, quantity int32, orderID string, idempotencyKey string) error
}

// PaymentClient is the seam used by the saga to process and refund payments.
type PaymentClient interface {
	ProcessPayment(ctx context.Context, orderID string, amountCents int64, idempotencyKey string) (string, error)
	Refund(ctx context.Context, paymentID string, idempotencyKey string) error
}

// Locker is a distributed lock used to ensure only one recovery worker runs
// at a time.
type Locker interface {
	TryLock(ctx context.Context, key string) (bool, error)
	Unlock(ctx context.Context, key string) error
}

// Recoverer is the seam that RecoveryWorker leverages to run recovery. The
// Orchestrator satisfies this interface, which decouples the worker from the
// concrete orchestrator type.
type Recoverer interface {
	Recover(ctx context.Context) error
}

// The following type aliases re-export the seam interfaces from their deep
// modules so callers can depend on a single saga package while the adapters
// live in focused subpackages.
type (
	// Store is the seam for saga persistence.
	Store = store.Store
	// Step is the seam for a single saga step.
	Step = steps.Step
	// StateMachine is the seam that decides the next step.
	StateMachine = statemachine.StateMachine
	// StepExecutor is the seam that runs a step and its compensation.
	StepExecutor = executor.StepExecutor
	// RetryConfig configures the retry executor.
	RetryConfig = executor.RetryConfig
	// CompensationPlanner is the seam that decides compensation steps.
	CompensationPlanner = compensator.CompensationPlanner
)

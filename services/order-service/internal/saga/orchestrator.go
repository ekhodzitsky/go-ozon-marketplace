package saga

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga/steps"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Orchestrator is a thin facade that wires the deep saga modules together.
// ProcessOrder and Recover preserve the original status transitions and
// idempotency semantics; the heavy lifting lives in Store, StateMachine,
// StepExecutor, CompensationPlanner, and the step adapters.
type Orchestrator struct {
	store        Store
	orderRepo    repository.OrderRepository
	stateMachine StateMachine
	executor     StepExecutor
	compensator  CompensationPlanner
	log          *zap.Logger
	queryTimeout time.Duration
}

// NewOrchestrator builds an Orchestrator from the existing constructor
// signature. It assembles the default adapters internally so callers keep the
// same shallow interface while benefiting from the deeper module structure.
func NewOrchestrator(orderRepo repository.OrderRepository, sagaRepo repository.SagaRepository, invClient InventoryClient, payClient PaymentClient, log *zap.Logger, callTimeout time.Duration, queryTimeout time.Duration) *Orchestrator {
	if callTimeout == 0 {
		callTimeout = 5 * time.Second
	}
	if queryTimeout == 0 {
		queryTimeout = 3 * time.Second
	}

	sagaStore := NewRepositoryStore(sagaRepo)
	startStep := NewStartStep(orderRepo, log)
	reserveStep := NewReserveInventoryStep(invClient)
	paymentStep := NewProcessPaymentStep(payClient)
	confirmStep := NewConfirmOrderStep(orderRepo)

	sm := NewOrderSagaStateMachine(startStep, reserveStep, paymentStep, confirmStep)
	exec := NewRetryExecutor(RetryConfig{
		MaxRetries:  3,
		BaseDelay:   200 * time.Millisecond,
		CallTimeout: callTimeout,
	}, log)
	comp := NewOrderCompensator(reserveStep, paymentStep)

	return &Orchestrator{
		store:        sagaStore,
		orderRepo:    orderRepo,
		stateMachine: sm,
		executor:     exec,
		compensator:  comp,
		log:          log,
		queryTimeout: queryTimeout,
	}
}

func (o *Orchestrator) withQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, o.queryTimeout)
}

func (o *Orchestrator) persistSaga(ctx context.Context, saga *domain.Saga) error {
	saga.UpdatedAt = time.Now().UTC()
	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()
	if err := o.store.Save(qCtx, saga); err != nil {
		o.log.Error("failed to save saga", zap.Error(err), zap.String("order_id", saga.OrderID.String()))
		return err
	}
	return nil
}

func (o *Orchestrator) saveSaga(ctx context.Context, saga *domain.Saga, status domain.SagaStatus, step, errMsg string) error {
	saga.Status = status
	saga.CurrentStep = step
	saga.ErrorMessage = errMsg
	return o.persistSaga(ctx, saga)
}

// ProcessOrder drives the saga from its current state to completion. It keeps
// the original transition order and idempotency-key generation; failures are
// compensated in payment-first-then-inventory order.
func (o *Orchestrator) ProcessOrder(ctx context.Context, order *domain.Order, idempotencyKey string) error {
	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()
	saga, err := o.store.GetByOrderID(qCtx, order.ID)
	if err != nil {
		if !errors.Is(err, apperrors.ErrNotFound) {
			return fmt.Errorf("get saga: %w", err)
		}
		saga = &domain.Saga{
			ID:        uuid.New(),
			OrderID:   order.ID,
			Status:    domain.SagaStatusPending,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		qCtx, cancel = o.withQueryTimeout(ctx)
		defer cancel()
		if err := o.store.Create(qCtx, saga); err != nil {
			return fmt.Errorf("create saga: %w", err)
		}
	}

	if o.stateMachine.IsTerminal(saga) {
		return nil
	}

	for {
		step, err := o.stateMachine.Next(saga, order)
		if err != nil {
			return fmt.Errorf("state machine: %w", err)
		}
		if step == nil {
			break
		}

		if p, ok := step.(steps.Preparable); ok {
			if p.Prepare(saga) {
				if err := o.persistSaga(ctx, saga); err != nil {
					return err
				}
			}
		}

		if err := o.executor.Execute(ctx, step, saga, order, idempotencyKey); err != nil {
			return o.handleStepFailure(ctx, saga, order, step, idempotencyKey, err)
		}

		if err := o.persistSaga(ctx, saga); err != nil {
			return err
		}
	}

	return nil
}

func (o *Orchestrator) handleStepFailure(ctx context.Context, saga *domain.Saga, order *domain.Order, step steps.Step, idempotencyKey string, execErr error) error {
	// The start step is special: there is nothing to compensate yet, so the
	// saga moves straight to failed and the error is returned to the caller.
	if saga.Status == domain.SagaStatusPending {
		_ = o.saveSaga(ctx, saga, domain.SagaStatusFailed, step.Name(), execErr.Error())
		return fmt.Errorf("update status awaiting_payment: %w", execErr)
	}

	plan := o.compensator.Plan(saga, step)
	currentStep := "compensate_" + joinStepNames(plan)
	_ = o.saveSaga(ctx, saga, domain.SagaStatusCompensating, currentStep, execErr.Error())

	for _, compStep := range plan {
		if err := o.executor.Compensate(ctx, compStep, saga, order, idempotencyKey); err != nil {
			o.log.Error("compensation step failed", zap.Error(err), zap.String("step", compStep.Name()))
		}
	}

	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()
	if err := o.orderRepo.UpdateStatus(qCtx, order.ID, domain.OrderStatusCancelled); err != nil {
		o.log.Error("failed to cancel order", zap.Error(err), zap.String("order_id", order.ID.String()))
	}

	_ = o.saveSaga(ctx, saga, domain.SagaStatusCancelled, "cancelled", "")
	o.log.Warn("order cancelled after step failure", zap.String("order_id", order.ID.String()), zap.String("step", step.Name()), zap.Error(execErr))
	return nil
}

func joinStepNames(plan []steps.Step) string {
	names := make([]string, len(plan))
	for i, s := range plan {
		names[i] = s.Name()
	}
	return strings.Join(names, "+")
}

// Recover lists incomplete sagas and re-processes each one with a
// deterministic recovery idempotency key.
func (o *Orchestrator) Recover(ctx context.Context) error {
	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()
	sagas, err := o.store.ListIncomplete(qCtx, 100)
	if err != nil {
		return fmt.Errorf("list incomplete sagas: %w", err)
	}
	for _, s := range sagas {
		func(s domain.Saga) {
			qCtx, cancel := o.withQueryTimeout(ctx)
			defer cancel()
			order, err := o.orderRepo.GetByID(qCtx, s.OrderID)
			if err != nil {
				o.log.Error("recover: failed to get order", zap.Error(err), zap.String("order_id", s.OrderID.String()))
				return
			}
			if err := o.ProcessOrder(ctx, order, recoveryIdempotencyKey(s.OrderID.String())); err != nil {
				o.log.Error("recover: process order failed", zap.Error(err), zap.String("order_id", s.OrderID.String()))
			}
		}(s)
	}
	return nil
}

func recoveryIdempotencyKey(orderID string) string {
	return fmt.Sprintf("recovery:%s", orderID)
}

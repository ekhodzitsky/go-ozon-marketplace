package steps

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"go.uber.org/zap"
)

// StartStep moves the order from pending to awaiting payment. It is the first
// step in the order saga and has no compensation.
type StartStep struct {
	orderRepo repository.OrderRepository
	log       *zap.Logger
}

// NewStartStep creates a StartStep adapter.
func NewStartStep(orderRepo repository.OrderRepository, log *zap.Logger) *StartStep {
	return &StartStep{orderRepo: orderRepo, log: log}
}

// Name returns the step identifier.
func (s *StartStep) Name() string {
	return "start"
}

// Execute updates the order status to awaiting payment and advances the saga
// to the reserving state on success.
func (s *StartStep) Execute(ctx context.Context, saga *domain.Saga, order *domain.Order, idempotencyKey string) error {
	if err := s.orderRepo.UpdateStatus(ctx, order.ID, domain.OrderStatusAwaitingPayment); err != nil {
		return err
	}
	saga.Status = domain.SagaStatusReserving
	saga.CurrentStep = "reserve"
	return nil
}

// Compensate is a no-op for the start step.
func (s *StartStep) Compensate(ctx context.Context, saga *domain.Saga, order *domain.Order, idempotencyKey string) error {
	return nil
}

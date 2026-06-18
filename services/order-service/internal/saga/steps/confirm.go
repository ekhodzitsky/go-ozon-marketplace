package steps

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
)

// ConfirmOrderStep finalizes the order by marking it as paid. It records the
// pending "confirming" status before the repository call and the final
// "confirmed" status after success.
type ConfirmOrderStep struct {
	orderRepo repository.OrderRepository
}

// NewConfirmOrderStep creates a ConfirmOrderStep adapter.
func NewConfirmOrderStep(orderRepo repository.OrderRepository) *ConfirmOrderStep {
	return &ConfirmOrderStep{orderRepo: orderRepo}
}

// Name returns the step identifier.
func (s *ConfirmOrderStep) Name() string {
	return "confirm"
}

// Prepare records the intermediate confirming state before the repository
// call. It returns false when the saga is already confirming so recovery does
// not produce duplicate transitions.
func (s *ConfirmOrderStep) Prepare(saga *domain.Saga) bool {
	if saga.Status == domain.SagaStatusConfirming {
		return false
	}
	saga.Status = domain.SagaStatusConfirming
	saga.CurrentStep = "confirm"
	return true
}

// Execute updates the order status to paid.
func (s *ConfirmOrderStep) Execute(ctx context.Context, saga *domain.Saga, order *domain.Order, idempotencyKey string) error {
	if err := s.orderRepo.UpdateStatus(ctx, order.ID, domain.OrderStatusPaid); err != nil {
		return err
	}

	saga.Status = domain.SagaStatusConfirmed
	saga.CurrentStep = "confirmed"
	return nil
}

// Compensate is a no-op for the confirm step; compensation is handled by the
// planner (payment first, then inventory).
func (s *ConfirmOrderStep) Compensate(ctx context.Context, saga *domain.Saga, order *domain.Order, idempotencyKey string) error {
	return nil
}

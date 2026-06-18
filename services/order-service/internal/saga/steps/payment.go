package steps

import (
	"context"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
)

// PaymentClient is the narrow interface this step needs from the payment
// module. It mirrors saga.PaymentClient so the same concrete client can be
// passed through without an explicit adapter.
type PaymentClient interface {
	ProcessPayment(ctx context.Context, orderID string, amountCents int64, idempotencyKey string) (string, error)
	Refund(ctx context.Context, paymentID string, idempotencyKey string) error
}

// ProcessPaymentStep charges the customer. It records the pending "paying"
// status before the remote call and the final "paid" status after success.
type ProcessPaymentStep struct {
	client PaymentClient
}

// NewProcessPaymentStep creates a ProcessPaymentStep adapter.
func NewProcessPaymentStep(client PaymentClient) *ProcessPaymentStep {
	return &ProcessPaymentStep{client: client}
}

// Name returns the step identifier used in compensation planning.
func (s *ProcessPaymentStep) Name() string {
	return "payment"
}

// Prepare records the intermediate paying state before the remote call. It
// returns false when the saga is already in the paying state so recovery does
// not produce duplicate transitions.
func (s *ProcessPaymentStep) Prepare(saga *domain.Saga) bool {
	if saga.Status == domain.SagaStatusPaying {
		return false
	}
	saga.Status = domain.SagaStatusPaying
	saga.CurrentStep = "payment"
	return true
}

// Execute processes the payment and stores the resulting payment identifier.
func (s *ProcessPaymentStep) Execute(ctx context.Context, saga *domain.Saga, order *domain.Order, idempotencyKey string) error {
	key := paymentKey(idempotencyKey)
	paymentID, err := s.client.ProcessPayment(ctx, order.ID.String(), order.TotalAmount, key)
	if err != nil {
		return err
	}

	saga.PaymentID = paymentID
	saga.Status = domain.SagaStatusPaid
	saga.CurrentStep = "paid"
	return nil
}

// Compensate refunds the payment when a payment identifier is present.
func (s *ProcessPaymentStep) Compensate(ctx context.Context, saga *domain.Saga, order *domain.Order, idempotencyKey string) error {
	if saga.PaymentID == "" {
		return nil
	}
	key := refundKey(idempotencyKey, saga.PaymentID)
	return s.client.Refund(ctx, saga.PaymentID, key)
}

func paymentKey(base string) string {
	return fmt.Sprintf("payment:%s", base)
}

func refundKey(base, paymentID string) string {
	return fmt.Sprintf("refund:%s:%s", base, paymentID)
}

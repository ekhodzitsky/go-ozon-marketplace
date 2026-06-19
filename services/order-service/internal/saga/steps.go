package saga

import (
	"context"
	"errors"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"go.uber.org/zap"
)

// InventoryClient — минимум, что нужен саге для резерва и освобождения товара.
type InventoryClient interface {
	Reserve(ctx context.Context, productID string, quantity int32, orderID string, idempotencyKey string) error
	Release(ctx context.Context, productID string, quantity int32, orderID string, idempotencyKey string) error
}

// PaymentClient — минимум, что нужен саге для оплаты и возврата.
type PaymentClient interface {
	ProcessPayment(ctx context.Context, orderID string, amountCents int64, idempotencyKey string) (string, error)
	Refund(ctx context.Context, paymentID string, idempotencyKey string) error
}

// Step — один шаг саги.
type Step interface {
	Name() string
	Execute(ctx context.Context, saga *Saga, order *domain.Order, idempotencyKey string) error
	Compensate(ctx context.Context, saga *Saga, order *domain.Order, idempotencyKey string) error
}

// Preparable — шаг, который перед вызовом фиксирует промежуточный статус.
type Preparable interface {
	Prepare(saga *Saga) bool
}

// startStep переводит заказ из pending в awaiting_payment.
type startStep struct {
	orderRepo repository.OrderRepository
	log       *zap.Logger
}

// NewStartStep создает первый шаг саги.
func NewStartStep(orderRepo repository.OrderRepository, log *zap.Logger) Step {
	return &startStep{orderRepo: orderRepo, log: log}
}

func (s *startStep) Name() string { return "start" }

func (s *startStep) Execute(ctx context.Context, saga *Saga, order *domain.Order, idempotencyKey string) error {
	if err := s.orderRepo.UpdateStatus(ctx, order.ID, domain.OrderStatusAwaitingPayment); err != nil {
		return err
	}
	saga.Status = SagaStatusReserving
	saga.CurrentStep = "reserve"
	return nil
}

func (s *startStep) Compensate(ctx context.Context, saga *Saga, order *domain.Order, idempotencyKey string) error {
	return nil
}

// reserveInventoryStep резервирует товары по одному, чтобы recovery могла продолжить с последнего.
type reserveInventoryStep struct {
	client InventoryClient
}

// NewReserveInventoryStep создает шаг резервирования товара.
func NewReserveInventoryStep(client InventoryClient) Step {
	return &reserveInventoryStep{client: client}
}

func (s *reserveInventoryStep) Name() string { return "inventory" }

func (s *reserveInventoryStep) Execute(ctx context.Context, saga *Saga, order *domain.Order, idempotencyKey string) error {
	idx := len(saga.ReservedItems)
	if idx >= len(order.Items) {
		saga.Status = SagaStatusReserved
		saga.CurrentStep = "reserved"
		return nil
	}

	item := order.Items[idx]
	key := reserveKey(idempotencyKey, item.ProductID.String())
	if err := s.client.Reserve(ctx, item.ProductID.String(), int32(item.Quantity), order.ID.String(), key); err != nil {
		return err
	}

	saga.ReservedItems = append(saga.ReservedItems, SagaReservedItem{
		ProductID: item.ProductID.String(),
		Quantity:  int32(item.Quantity),
	})
	return nil
}

func (s *reserveInventoryStep) Compensate(ctx context.Context, saga *Saga, order *domain.Order, idempotencyKey string) error {
	var errs []error
	for _, item := range saga.ReservedItems {
		key := releaseKey(order.ID.String(), item.ProductID)
		if err := s.client.Release(ctx, item.ProductID, item.Quantity, order.ID.String(), key); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func reserveKey(base, productID string) string {
	return fmt.Sprintf("reserve:%s:%s", base, productID)
}

func releaseKey(orderID, productID string) string {
	return fmt.Sprintf("release:%s:%s", orderID, productID)
}

// processPaymentStep списывает деньги и сохраняет payment_id.
type processPaymentStep struct {
	client PaymentClient
}

// NewProcessPaymentStep создает шаг оплаты.
func NewProcessPaymentStep(client PaymentClient) Step {
	return &processPaymentStep{client: client}
}

func (s *processPaymentStep) Name() string { return "payment" }

func (s *processPaymentStep) Prepare(saga *Saga) bool {
	if saga.Status == SagaStatusPaying {
		return false
	}
	saga.Status = SagaStatusPaying
	saga.CurrentStep = "payment"
	return true
}

func (s *processPaymentStep) Execute(ctx context.Context, saga *Saga, order *domain.Order, idempotencyKey string) error {
	key := paymentKey(idempotencyKey)
	paymentID, err := s.client.ProcessPayment(ctx, order.ID.String(), order.TotalAmount, key)
	if err != nil {
		return err
	}

	saga.PaymentID = paymentID
	saga.Status = SagaStatusPaid
	saga.CurrentStep = "paid"
	return nil
}

func (s *processPaymentStep) Compensate(ctx context.Context, saga *Saga, order *domain.Order, idempotencyKey string) error {
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

// confirmOrderStep финализирует заказ, переводя его в paid.
type confirmOrderStep struct {
	orderRepo repository.OrderRepository
}

// NewConfirmOrderStep создает финальный шаг саги.
func NewConfirmOrderStep(orderRepo repository.OrderRepository) Step {
	return &confirmOrderStep{orderRepo: orderRepo}
}

func (s *confirmOrderStep) Name() string { return "confirm" }

func (s *confirmOrderStep) Prepare(saga *Saga) bool {
	if saga.Status == SagaStatusConfirming {
		return false
	}
	saga.Status = SagaStatusConfirming
	saga.CurrentStep = "confirm"
	return true
}

func (s *confirmOrderStep) Execute(ctx context.Context, saga *Saga, order *domain.Order, idempotencyKey string) error {
	if err := s.orderRepo.UpdateStatus(ctx, order.ID, domain.OrderStatusPaid); err != nil {
		return err
	}

	saga.Status = SagaStatusConfirmed
	saga.CurrentStep = "confirmed"
	return nil
}

func (s *confirmOrderStep) Compensate(ctx context.Context, saga *Saga, order *domain.Order, idempotencyKey string) error {
	return nil
}

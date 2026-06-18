package steps

import (
	"context"
	"errors"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
)

// InventoryClient is the narrow interface this step needs from the inventory
// module. It mirrors saga.InventoryClient so the same concrete client can be
// passed through without an explicit adapter.
type InventoryClient interface {
	Reserve(ctx context.Context, productID string, quantity int32, orderID string, idempotencyKey string) error
	Release(ctx context.Context, productID string, quantity int32, orderID string, idempotencyKey string) error
}

// ReserveInventoryStep reserves order items one at a time. The step keeps the
// partial reservation state inside the saga so recovery can resume from the
// last successful item.
type ReserveInventoryStep struct {
	client InventoryClient
}

// NewReserveInventoryStep creates a ReserveInventoryStep adapter.
func NewReserveInventoryStep(client InventoryClient) *ReserveInventoryStep {
	return &ReserveInventoryStep{client: client}
}

// Name returns the step identifier used in compensation planning.
func (s *ReserveInventoryStep) Name() string {
	return "inventory"
}

// Execute reserves the next unreserved item. When every item is already
// reserved it transitions the saga to the reserved state.
func (s *ReserveInventoryStep) Execute(ctx context.Context, saga *domain.Saga, order *domain.Order, idempotencyKey string) error {
	idx := len(saga.ReservedItems)
	if idx >= len(order.Items) {
		saga.Status = domain.SagaStatusReserved
		saga.CurrentStep = "reserved"
		return nil
	}

	item := order.Items[idx]
	key := reserveKey(idempotencyKey, item.ProductID.String())
	if err := s.client.Reserve(ctx, item.ProductID.String(), int32(item.Quantity), order.ID.String(), key); err != nil {
		return err
	}

	saga.ReservedItems = append(saga.ReservedItems, domain.SagaReservedItem{
		ProductID: item.ProductID.String(),
		Quantity:  int32(item.Quantity),
	})
	return nil
}

// Compensate releases every reserved item. It attempts all releases and
// returns a joined error so the caller can decide how to log, while still
// maximizing the work completed.
func (s *ReserveInventoryStep) Compensate(ctx context.Context, saga *domain.Saga, order *domain.Order, idempotencyKey string) error {
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

package saga

import (
	"context"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"go.uber.org/zap"
)

type Orchestrator struct {
	orderRepo repository.OrderRepository
	invClient InventoryClient
	payClient PaymentClient
	log       *zap.Logger
}

func NewOrchestrator(orderRepo repository.OrderRepository, invClient InventoryClient, payClient PaymentClient, log *zap.Logger) *Orchestrator {
	return &Orchestrator{
		orderRepo: orderRepo,
		invClient: invClient,
		payClient: payClient,
		log:       log,
	}
}

func (o *Orchestrator) ProcessOrder(ctx context.Context, order *domain.Order) error {
	if err := o.orderRepo.UpdateStatus(ctx, order.ID, "awaiting_payment"); err != nil {
		return fmt.Errorf("update status awaiting_payment: %w", err)
	}

	for _, item := range order.Items {
		err := o.invClient.Reserve(ctx, item.ProductID.String(), int32(item.Quantity), order.ID.String())
		if err != nil {
			o.compensateInventory(ctx, order)
			_ = o.orderRepo.UpdateStatus(ctx, order.ID, "cancelled")
			return fmt.Errorf("reserve inventory product %s: %w", item.ProductID, err)
		}
	}

	_, err := o.payClient.ProcessPayment(ctx, order.ID.String(), order.UserID.String(), order.TotalAmount)
	if err != nil {
		o.compensateInventory(ctx, order)
		_ = o.orderRepo.UpdateStatus(ctx, order.ID, "cancelled")
		return fmt.Errorf("process payment: %w", err)
	}

	if err := o.orderRepo.UpdateStatus(ctx, order.ID, "confirmed"); err != nil {
		return fmt.Errorf("update status confirmed: %w", err)
	}

	return nil
}

func (o *Orchestrator) compensateInventory(ctx context.Context, order *domain.Order) {
	for _, item := range order.Items {
		err := o.invClient.Release(ctx, item.ProductID.String(), int32(item.Quantity), order.ID.String())
		if err != nil {
			o.log.Error("failed to release inventory", zap.Error(err), zap.String("product_id", item.ProductID.String()))
		}
	}
}

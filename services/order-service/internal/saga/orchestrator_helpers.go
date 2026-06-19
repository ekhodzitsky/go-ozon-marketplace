package saga

import (
	"context"
	"fmt"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"go.uber.org/zap"
)

func (o *Orchestrator) withCallTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, o.callTimeout)
}

func (o *Orchestrator) withQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, o.queryTimeout)
}

func (o *Orchestrator) retry(ctx context.Context, fn func() error) error {
	const maxRetries = 3
	const baseDelay = 200 * time.Millisecond
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(baseDelay * time.Duration(1<<(i-1))):
			}
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		o.log.Warn("retryable error", zap.Error(lastErr), zap.Int("attempt", i+1))
	}
	return lastErr
}

func (o *Orchestrator) saveSaga(ctx context.Context, saga *domain.Saga, status domain.SagaStatus, step, errMsg string) error {
	saga.Status = status
	saga.CurrentStep = step
	saga.ErrorMessage = errMsg
	saga.UpdatedAt = time.Now().UTC()
	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()
	if err := o.sagaRepo.Save(qCtx, saga); err != nil {
		o.log.Error("failed to save saga", zap.Error(err), zap.String("order_id", saga.OrderID.String()))
		return err
	}
	return nil
}

func (o *Orchestrator) compensateInventory(ctx context.Context, order *domain.Order, reserved []domain.SagaReservedItem) {
	for _, item := range reserved {
		err := o.retry(ctx, func() error {
			cCtx, cancel := o.withCallTimeout(ctx)
			defer cancel()
			_, err := o.invClient.Release(cCtx, &inventoryv1.ReleaseRequest{
				ProductId:      item.ProductID,
				Quantity:       item.Quantity,
				OrderId:        order.ID.String(),
				IdempotencyKey: releaseIdempotencyKey(order.ID.String(), item.ProductID),
			})
			return err
		})
		if err != nil {
			o.log.Error("failed to release inventory", zap.Error(err), zap.String("product_id", item.ProductID))
		}
	}
}

func (o *Orchestrator) Recover(ctx context.Context) error {
	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()
	sagas, err := o.sagaRepo.ListIncomplete(qCtx, 100)
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

func reserveIdempotencyKey(base, productID string) string {
	return fmt.Sprintf("reserve:%s:%s", base, productID)
}

func releaseIdempotencyKey(orderID, productID string) string {
	return fmt.Sprintf("release:%s:%s", orderID, productID)
}

func paymentIdempotencyKey(base string) string {
	return fmt.Sprintf("payment:%s", base)
}

func refundIdempotencyKey(base, paymentID string) string {
	return fmt.Sprintf("refund:%s:%s", base, paymentID)
}

func recoveryIdempotencyKey(orderID string) string {
	return fmt.Sprintf("recovery:%s", orderID)
}

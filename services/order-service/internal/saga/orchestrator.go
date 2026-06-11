package saga

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
)

type Orchestrator struct {
	orderRepo    repository.OrderRepository
	sagaRepo     repository.SagaRepository
	invClient    InventoryClient
	payClient    PaymentClient
	log          *zap.Logger
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewOrchestrator(orderRepo repository.OrderRepository, sagaRepo repository.SagaRepository, invClient InventoryClient, payClient PaymentClient, log *zap.Logger, callTimeout time.Duration, queryTimeout time.Duration) *Orchestrator {
	if callTimeout == 0 {
		callTimeout = 5 * time.Second
	}
	if queryTimeout == 0 {
		queryTimeout = 3 * time.Second
	}
	return &Orchestrator{
		orderRepo:    orderRepo,
		sagaRepo:     sagaRepo,
		invClient:    invClient,
		payClient:    payClient,
		log:          log,
		callTimeout:  callTimeout,
		queryTimeout: queryTimeout,
	}
}

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

func (o *Orchestrator) ProcessOrder(ctx context.Context, order *domain.Order) error {
	qCtx, cancel := o.withQueryTimeout(ctx)
	defer cancel()
	saga, err := o.sagaRepo.GetByOrderID(qCtx, order.ID)
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
		if err := o.sagaRepo.Create(qCtx, saga); err != nil {
			return fmt.Errorf("create saga: %w", err)
		}
	}

	switch saga.Status {
	case domain.SagaStatusConfirmed, domain.SagaStatusCancelled, domain.SagaStatusFailed:
		return nil
	}

	if saga.Status == domain.SagaStatusPending {
		if err := o.retry(ctx, func() error {
			qCtx, cancel := o.withQueryTimeout(ctx)
			defer cancel()
			return o.orderRepo.UpdateStatus(qCtx, order.ID, "awaiting_payment")
		}); err != nil {
			_ = o.saveSaga(ctx, saga, domain.SagaStatusFailed, "update_order_status", err.Error())
			return fmt.Errorf("update status awaiting_payment: %w", err)
		}
	}

	if saga.Status == domain.SagaStatusPending || saga.Status == domain.SagaStatusReserving {
		if saga.Status != domain.SagaStatusReserving {
			saga.Status = domain.SagaStatusReserving
			saga.CurrentStep = "reserve"
			qCtx, cancel := o.withQueryTimeout(ctx)
			defer cancel()
			_ = o.sagaRepo.Save(qCtx, saga)
		}

		startIdx := len(saga.ReservedItems)
		for i := startIdx; i < len(order.Items); i++ {
			item := order.Items[i]
			err := o.retry(ctx, func() error {
				cCtx, cancel := o.withCallTimeout(ctx)
				defer cancel()
				return o.invClient.Reserve(cCtx, item.ProductID.String(), int32(item.Quantity), order.ID.String())
			})
			if err != nil {
				_ = o.saveSaga(ctx, saga, domain.SagaStatusCompensating, "compensate_inventory", err.Error())
				o.compensateInventory(ctx, order, saga.ReservedItems)
				qCtx, cancel := o.withQueryTimeout(ctx)
				defer cancel()
				_ = o.orderRepo.UpdateStatus(qCtx, order.ID, "cancelled")
				_ = o.saveSaga(ctx, saga, domain.SagaStatusCancelled, "cancelled", "")
				return fmt.Errorf("reserve inventory product %s: %w", item.ProductID, err)
			}
			saga.ReservedItems = append(saga.ReservedItems, domain.SagaReservedItem{
				ProductID: item.ProductID.String(),
				Quantity:  int32(item.Quantity),
			})
			qCtx, cancel := o.withQueryTimeout(ctx)
			defer cancel()
			_ = o.sagaRepo.Save(qCtx, saga)
		}
		saga.Status = domain.SagaStatusReserved
		saga.CurrentStep = "reserved"
		qCtx, cancel := o.withQueryTimeout(ctx)
		defer cancel()
		_ = o.sagaRepo.Save(qCtx, saga)
	}

	if saga.Status == domain.SagaStatusReserved || saga.Status == domain.SagaStatusPaying {
		if saga.Status != domain.SagaStatusPaying {
			saga.Status = domain.SagaStatusPaying
			saga.CurrentStep = "payment"
			qCtx, cancel := o.withQueryTimeout(ctx)
			defer cancel()
			_ = o.sagaRepo.Save(qCtx, saga)
		}

		var paymentID string
		err := o.retry(ctx, func() error {
			cCtx, cancel := o.withCallTimeout(ctx)
			defer cancel()
			var payErr error
			paymentID, payErr = o.payClient.ProcessPayment(cCtx, order.ID.String(), order.UserID.String(), order.TotalAmount)
			return payErr
		})
		if err != nil {
			_ = o.saveSaga(ctx, saga, domain.SagaStatusCompensating, "compensate_inventory", err.Error())
			o.compensateInventory(ctx, order, saga.ReservedItems)
			qCtx, cancel := o.withQueryTimeout(ctx)
			defer cancel()
			_ = o.orderRepo.UpdateStatus(qCtx, order.ID, "cancelled")
			_ = o.saveSaga(ctx, saga, domain.SagaStatusCancelled, "cancelled", "")
			return fmt.Errorf("process payment: %w", err)
		}
		saga.Status = domain.SagaStatusPaid
		saga.CurrentStep = "paid"
		saga.PaymentID = paymentID
		qCtx, cancel := o.withQueryTimeout(ctx)
		defer cancel()
		_ = o.sagaRepo.Save(qCtx, saga)
	}

	if saga.Status == domain.SagaStatusPaid || saga.Status == domain.SagaStatusConfirming {
		if saga.Status != domain.SagaStatusConfirming {
			saga.Status = domain.SagaStatusConfirming
			saga.CurrentStep = "confirm"
			qCtx, cancel := o.withQueryTimeout(ctx)
			defer cancel()
			_ = o.sagaRepo.Save(qCtx, saga)
		}

		err := o.retry(ctx, func() error {
			qCtx, cancel := o.withQueryTimeout(ctx)
			defer cancel()
			return o.orderRepo.UpdateStatus(qCtx, order.ID, "confirmed")
		})
		if err != nil {
			_ = o.saveSaga(ctx, saga, domain.SagaStatusCompensating, "compensate_payment+inventory", err.Error())
			if saga.PaymentID != "" {
				if refundErr := o.retry(ctx, func() error {
					cCtx, cancel := o.withCallTimeout(ctx)
					defer cancel()
					return o.payClient.Refund(cCtx, saga.PaymentID)
				}); refundErr != nil {
					o.log.Error("failed to refund payment", zap.Error(refundErr), zap.String("payment_id", saga.PaymentID))
				}
			}
			o.compensateInventory(ctx, order, saga.ReservedItems)
			qCtx, cancel := o.withQueryTimeout(ctx)
			defer cancel()
			_ = o.orderRepo.UpdateStatus(qCtx, order.ID, "cancelled")
			_ = o.saveSaga(ctx, saga, domain.SagaStatusCancelled, "cancelled", "")
			return fmt.Errorf("update status confirmed: %w", err)
		}
		saga.Status = domain.SagaStatusConfirmed
		saga.CurrentStep = "confirmed"
		qCtx, cancel := o.withQueryTimeout(ctx)
		defer cancel()
		_ = o.sagaRepo.Save(qCtx, saga)
	}

	return nil
}

func (o *Orchestrator) compensateInventory(ctx context.Context, order *domain.Order, reserved []domain.SagaReservedItem) {
	for _, item := range reserved {
		err := o.retry(ctx, func() error {
			cCtx, cancel := o.withCallTimeout(ctx)
			defer cancel()
			return o.invClient.Release(cCtx, item.ProductID, item.Quantity, order.ID.String())
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
			if err := o.ProcessOrder(ctx, order); err != nil {
				o.log.Error("recover: process order failed", zap.Error(err), zap.String("order_id", s.OrderID.String()))
			}
		}(s)
	}
	return nil
}

package usecase

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	DefaultCallTimeout  = 5 * time.Second
	DefaultQueryTimeout = 3 * time.Second
)

type paymentUsecase struct {
	repo         repository.PaymentRepository
	txm          repository.TxManager
	log          *zap.Logger
	randGen      func() float64
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewPaymentUsecase(repo repository.PaymentRepository, txm repository.TxManager, log *zap.Logger, callTimeout time.Duration, queryTimeout time.Duration) *paymentUsecase {
	if callTimeout == 0 {
		callTimeout = DefaultCallTimeout
	}
	if queryTimeout == 0 {
		queryTimeout = DefaultQueryTimeout
	}
	return &paymentUsecase{repo: repo, txm: txm, log: log, callTimeout: callTimeout, queryTimeout: queryTimeout}
}

func (u *paymentUsecase) ProcessPayment(ctx context.Context, orderID, userID uuid.UUID, amountCents int64) (*domain.Payment, error) {
	// simulate processing result deterministically
	randVal := rand.Float64()
	if u.randGen != nil {
		randVal = u.randGen()
	}
	var finalStatus domain.Status
	if randVal < 0.9 {
		finalStatus = domain.StatusSuccess
	} else {
		finalStatus = domain.StatusFailed
	}

	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()

	payment := &domain.Payment{
		ID:      uuid.New(),
		OrderID: orderID,
		UserID:  userID,
		Amount:  amountCents,
		Status:  domain.StatusPending,
	}

	var result *domain.Payment
	err := u.txm.Run(ctx, func(repo repository.PaymentRepository) error {
		existing, err := repo.CreateOrGet(ctx, payment)
		if err != nil {
			return fmt.Errorf("create or get payment: %w", err)
		}

		if existing.Status == domain.StatusPending {
			if _, err := repo.UpdateStatusIf(ctx, existing.ID, finalStatus, domain.StatusPending); err != nil {
				return fmt.Errorf("update payment status: %w", err)
			}
			existing.Status = finalStatus
		}

		result = existing
		return nil
	})
	if err != nil {
		return nil, err
	}

	u.log.Info("payment processed",
		zap.String("event", "payment.processed"),
		zap.String("payment_id", result.ID.String()),
		zap.String("order_id", result.OrderID.String()),
		zap.String("user_id", result.UserID.String()),
		zap.Int64("amount_cents", result.Amount),
		zap.String("status", string(result.Status)),
	)

	return result, nil
}

func (u *paymentUsecase) Refund(ctx context.Context, paymentID uuid.UUID, idempotencyKey string) (*domain.Payment, *domain.Refund, error) {
	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()

	var resultPayment *domain.Payment
	var resultRefund *domain.Refund

	err := u.txm.Run(ctx, func(repo repository.PaymentRepository) error {
		payment, err := repo.GetByID(ctx, paymentID)
		if err != nil {
			return fmt.Errorf("get payment: %w", err)
		}

		if payment.Status != domain.StatusSuccess {
			return fmt.Errorf("%w: payment status %s cannot be refunded", apperrors.ErrFailedPrecondition, payment.Status)
		}

		refund := &domain.Refund{
			ID:             uuid.New(),
			PaymentID:      payment.ID,
			Amount:         payment.Amount,
			Reason:         "",
			Status:         domain.StatusRefunded,
			IdempotencyKey: idempotencyKey,
			CreatedAt:      time.Now().UTC(),
		}
		if err := repo.CreateRefund(ctx, refund); err != nil {
			return fmt.Errorf("create refund: %w", err)
		}

		updated, err := repo.UpdateStatusIf(ctx, payment.ID, domain.StatusRefunded, domain.StatusSuccess)
		if err != nil {
			return fmt.Errorf("update payment status: %w", err)
		}
		if !updated {
			return fmt.Errorf("%w: payment already refunded or status changed", apperrors.ErrFailedPrecondition)
		}

		payment.Status = domain.StatusRefunded
		resultPayment = payment
		resultRefund = refund
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	u.log.Info("payment refunded",
		zap.String("event", "payment.refunded"),
		zap.String("payment_id", resultPayment.ID.String()),
		zap.String("refund_id", resultRefund.ID.String()),
		zap.Int64("amount_cents", resultRefund.Amount),
	)

	return resultPayment, resultRefund, nil
}

func (u *paymentUsecase) GetByID(ctx context.Context, paymentID uuid.UUID) (*domain.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.repo.GetByID(ctx, paymentID)
}

func (u *paymentUsecase) GetRefund(ctx context.Context, refundID uuid.UUID) (*domain.Refund, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.repo.GetRefund(ctx, refundID)
}

func (u *paymentUsecase) ListRefunds(ctx context.Context, paymentID uuid.UUID) ([]*domain.Refund, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.repo.ListRefunds(ctx, paymentID)
}

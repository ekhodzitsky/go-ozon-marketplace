package usecase

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type PaymentUsecase struct {
	repo    repository.PaymentRepository
	log     *zap.Logger
	randGen func() float64
	sleeper func()
}

func NewPaymentUsecase(repo repository.PaymentRepository, log *zap.Logger) *PaymentUsecase {
	return &PaymentUsecase{repo: repo, log: log}
}

func (u *PaymentUsecase) ProcessPayment(ctx context.Context, orderID, userID uuid.UUID, amount float64) (*domain.Payment, error) {
	payment := &domain.Payment{
		ID:      uuid.New(),
		OrderID: orderID,
		UserID:  userID,
		Amount:  amount,
		Status:  domain.StatusPending,
	}

	if err := u.repo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}

	// simulate async processing
	if u.sleeper != nil {
		u.sleeper()
	} else {
		time.Sleep(100 * time.Millisecond)
	}

	// 90% success rate
	randVal := rand.Float64()
	if u.randGen != nil {
		randVal = u.randGen()
	}
	if randVal < 0.9 {
		payment.Status = domain.StatusSuccess
	} else {
		payment.Status = domain.StatusFailed
	}

	if err := u.repo.UpdateStatus(ctx, payment.ID, payment.Status); err != nil {
		return nil, fmt.Errorf("update payment status: %w", err)
	}

	u.log.Info("payment event published",
		zap.String("event", "payment.processed"),
		zap.String("payment_id", payment.ID.String()),
		zap.String("order_id", payment.OrderID.String()),
		zap.String("user_id", payment.UserID.String()),
		zap.Float64("amount", payment.Amount),
		zap.String("status", payment.Status),
	)

	return payment, nil
}

func (u *PaymentUsecase) Refund(ctx context.Context, paymentID uuid.UUID) (*domain.Payment, error) {
	payment, err := u.repo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("get payment: %w", err)
	}

	if err := u.repo.UpdateStatus(ctx, payment.ID, domain.StatusRefunded); err != nil {
		return nil, fmt.Errorf("update payment status: %w", err)
	}

	payment.Status = domain.StatusRefunded
	return payment, nil
}
